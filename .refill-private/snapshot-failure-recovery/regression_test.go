package snapshot_failure_recovery_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"image-integrity-review/internal/domain"
	"image-integrity-review/internal/repository"
)

func TestFailedUpdateDoesNotAppearAfterRestart(t *testing.T) {
	root := t.TempDir()
	store, err := repository.OpenFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	item, err := domain.NewReviewCase("case-partial", "PARTIAL-001", "部分提交", "研究论文", []domain.FigureRecord{{
		FigureLabel: "Figure 1", Caption: "图注", ContentDigest: "aaaaaaaaaaaaaaaa", PixelWidth: 10, PixelHeight: 10,
	}}, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Create(context.Background(), item, repository.CommitMeta{
		EventType: "case_created", RequestID: "partial-create", Actor: domain.Actor{Type: domain.ActorEditor, ID: "editor"},
	})
	if err != nil {
		t.Fatal(err)
	}

	snapshotDir := filepath.Join(root, "snapshots")
	if err := os.RemoveAll(snapshotDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotDir, []byte("blocks snapshot writes"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, updateErr := store.Update(context.Background(), item.ID, repository.CommitMeta{
		EventType: "integrity_checks_completed", RequestID: "partial-submit", ExpectedRevision: created.Case.Revision,
		Actor: domain.Actor{Type: domain.ActorEditor, ID: "editor"},
	}, func(value *domain.ReviewCase) error {
		return value.SubmitForChecks(time.Date(2026, 8, 24, 12, 1, 0, 0, time.UTC))
	})
	if updateErr == nil {
		t.Fatal("快照写入被阻断时 Update 应返回错误")
	}
	current, err := store.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.State != domain.StateDraft {
		t.Fatalf("返回错误后当前进程状态 = %s，期望 draft", current.State)
	}

	if err := os.Remove(snapshotDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(snapshotDir, 0o750); err != nil {
		t.Fatal(err)
	}
	reopened, err := repository.OpenFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := reopened.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.State != domain.StateDraft || restored.Revision != created.Case.Revision {
		t.Fatalf("失败的 Update 在重启后生效：state=%s revision=%d", restored.State, restored.Revision)
	}
}

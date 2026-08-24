package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"image-integrity-review/internal/domain"
)

func newStoredCase(t *testing.T, store *FileStore) (*domain.ReviewCase, CommitMeta) {
	t.Helper()
	item, err := domain.NewReviewCase("case-store", "STORE-001", "仓储测试", "研究论文", []domain.FigureRecord{{FigureLabel: "1", Caption: "图注", ContentDigest: "aaaaaaaaaaaaaaaa", PixelWidth: 100, PixelHeight: 100}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	meta := CommitMeta{EventType: "case_created", Actor: domain.Actor{Type: domain.ActorEditor, ID: "editor-1"}, RequestID: "create-1", ExpectedRevision: 0}
	result, err := store.Create(context.Background(), item, meta)
	if err != nil {
		t.Fatal(err)
	}
	return result.Case, meta
}

func TestFileStoreRevisionIdempotencyAndRecovery(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	item, _ := newStoredCase(t, store)
	update := CommitMeta{EventType: "submitted", Actor: domain.Actor{Type: domain.ActorEditor, ID: "editor-1"}, RequestID: "submit-1", ExpectedRevision: item.Revision}
	first, err := store.Update(context.Background(), item.ID, update, func(value *domain.ReviewCase) error { return value.SubmitForChecks(time.Now()) })
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := store.Update(context.Background(), item.ID, update, func(value *domain.ReviewCase) error { t.Fatal("幂等重放不应再次执行变更"); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replayed || replayed.Case.Revision != first.Case.Revision {
		t.Fatalf("幂等结果无效：%+v", replayed)
	}
	conflict := update
	conflict.RequestID = "stale-request"
	if _, err := store.Update(context.Background(), item.ID, conflict, func(*domain.ReviewCase) error { return nil }); !errors.Is(err, ErrRevision) {
		t.Fatalf("期望 revision 冲突，实际 %v", err)
	}
	reopened, err := OpenFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := reopened.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Revision != first.Case.Revision || restored.State != domain.StatePendingReview {
		t.Fatalf("恢复结果错误：%+v", restored)
	}
}

func TestFileStoreRejectsCorruptSnapshot(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	item, _ := newStoredCase(t, store)
	path := filepath.Join(dir, "snapshots", item.ID+".json")
	if err := os.WriteFile(path, []byte(`{"digest":"bad","case":{"id":"case-store"}}`), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFileStore(dir); err == nil {
		t.Fatal("损坏快照应被拒绝")
	}
}

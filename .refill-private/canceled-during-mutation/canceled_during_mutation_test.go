package canceledduringmutation

import (
	"context"
	"errors"
	"testing"
	"time"

	"image-integrity-review/internal/domain"
	"image-integrity-review/internal/repository"
)

func TestCanceledDuringMutationMustNotCommit(t *testing.T) {
	store, err := repository.OpenFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	item, err := domain.NewReviewCase(
		"case-cancel-mutate",
		"CANCEL-MUTATE-001",
		"初始标题",
		"研究论文",
		[]domain.FigureRecord{{FigureLabel: "1", Caption: "图注", ContentDigest: "aaaaaaaaaaaaaaaa", PixelWidth: 100, PixelHeight: 100}},
		time.Unix(1700000000, 0).UTC(),
	)
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Create(context.Background(), item, repository.CommitMeta{
		EventType:        "case_created",
		Actor:            domain.Actor{Type: domain.ActorEditor, ID: "editor-1"},
		RequestID:        "create-cancel-mutate",
		ExpectedRevision: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = store.Update(ctx, created.Case.ID, repository.CommitMeta{
		EventType:        "title_changed",
		Actor:            domain.Actor{Type: domain.ActorEditor, ID: "editor-1"},
		RequestID:        "update-cancel-mutate",
		ExpectedRevision: created.Case.Revision,
	}, func(value *domain.ReviewCase) error {
		cancel()
		value.Title = "不应提交的标题"
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("取消 mutate 后应返回 context.Canceled，实际错误: %v", err)
	}

	got, err := store.Get(context.Background(), created.Case.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "初始标题" || got.Revision != created.Case.Revision {
		t.Fatalf("取消请求不应提交聚合，得到 title=%q revision=%d", got.Title, got.Revision)
	}
}

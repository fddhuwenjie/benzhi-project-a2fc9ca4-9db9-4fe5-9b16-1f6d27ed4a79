package commit_result_alias_test

import (
	"context"
	"testing"
	"time"

	"image-integrity-review/internal/domain"
	"image-integrity-review/internal/repository"
)

func TestCommitResultCannotMutateStoredCase(t *testing.T) {
	store, err := repository.OpenFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	item, err := domain.NewReviewCase("case-alias", "ALIAS-001", "原始标题", "研究论文", []domain.FigureRecord{{
		FigureLabel: "Figure 1", Caption: "图注", ContentDigest: "aaaaaaaaaaaaaaaa", PixelWidth: 10, PixelHeight: 10,
	}}, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.Create(context.Background(), item, repository.CommitMeta{
		EventType: "case_created", RequestID: "create-alias", Actor: domain.Actor{Type: domain.ActorEditor, ID: "editor"},
	})
	if err != nil {
		t.Fatal(err)
	}
	result.Case.Title = "未审计的外部篡改"
	result.Case.Figures[0].Caption = "未审计的图注篡改"

	stored, err := store.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Title != "原始标题" || stored.Figures[0].Caption != "图注" {
		t.Fatalf("修改 CommitResult.Case 污染了仓库状态：%+v", stored)
	}
}

package archive_half_commit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"image-integrity-review/internal/application"
	"image-integrity-review/internal/domain"
	"image-integrity-review/internal/repository"
)

type failingArchiveStore struct {
	repository.Store
}

func (f failingArchiveStore) SaveArchive(context.Context, domain.ArchiveDocument) error {
	return errors.New("injected archive storage failure")
}

func TestArchiveFailureDoesNotCommitArchivedState(t *testing.T) {
	base, err := repository.OpenFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	item, err := domain.NewReviewCase("case-archive-failure", "ARCHIVE-FAIL-001", "归档失败", "研究论文", []domain.FigureRecord{{
		FigureLabel: "Figure 1", Caption: "图注", ContentDigest: "aaaaaaaaaaaaaaaa", PixelWidth: 10, PixelHeight: 10,
	}}, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	item.State = domain.StateReadyDecision
	created, err := base.Create(context.Background(), item, repository.CommitMeta{
		EventType: "test_ready", RequestID: "archive-setup", Actor: domain.Actor{Type: domain.ActorSystem, ID: "setup"},
	})
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(failingArchiveStore{Store: base})
	_, _, decideErr := service.Decide(context.Background(), item.ID, application.DecisionInput{
		WriteContext: application.WriteContext{
			Actor: domain.Actor{Type: domain.ActorEditor, ID: "editor"}, RequestID: "approve-with-failure", ExpectedRevision: created.Case.Revision,
		},
		Decision: "approved", Note: "终审通过",
	})
	if decideErr == nil {
		t.Fatal("归档文件写入失败时 Decide 应返回错误")
	}
	stored, err := base.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != domain.StateReadyDecision || stored.FinalDecision != "" || stored.ArchivedAt != nil {
		t.Fatalf("归档文件写入失败后案件已被半提交：%+v", stored)
	}
}

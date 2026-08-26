package crosscaseidempotency

import (
	"context"
	"testing"

	"image-integrity-review/internal/application"
	"image-integrity-review/internal/domain"
	"image-integrity-review/internal/repository"
)

func TestRequestIDReuseMustStayWithinCase(t *testing.T) {
	store, err := repository.OpenFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)
	editor := domain.Actor{Type: domain.ActorEditor, ID: "editor-private"}
	create := func(requestID, manuscriptCode string) *domain.ReviewCase {
		t.Helper()
		item, _, err := service.CreateCase(context.Background(), application.CreateCaseInput{
			WriteContext:   application.WriteContext{Actor: editor, RequestID: requestID},
			ManuscriptCode: manuscriptCode,
			Title:          "跨案件幂等隔离复现",
			JournalSection: "研究论文",
			Figures: []domain.FigureRecord{{
				FigureLabel: "Figure 1", Caption: "原始图像", ContentDigest: "aaaaaaaaaaaaaaaa",
				PixelWidth: 640, PixelHeight: 480, ExperimentSource: "实验批次 A", RawDataReference: "raw://private/1",
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return item
	}

	caseA := create("create-private-a", "PRIVATE-A")
	caseB := create("create-private-b", "PRIVATE-B")
	sharedRequestID := "submit-shared-across-cases"
	first, replayed, err := service.SubmitDraft(context.Background(), caseA.ID, application.WriteContext{
		Actor: editor, RequestID: sharedRequestID, ExpectedRevision: caseA.Revision,
	})
	if err != nil || replayed || first.ID != caseA.ID {
		t.Fatalf("案件 A 首次提交失败: case=%v replayed=%v err=%v", first, replayed, err)
	}
	second, replayed, err := service.SubmitDraft(context.Background(), caseB.ID, application.WriteContext{
		Actor: editor, RequestID: sharedRequestID, ExpectedRevision: caseB.Revision,
	})
	if err != nil {
		t.Fatalf("案件 B 提交返回错误: %v", err)
	}
	if replayed || second.ID != caseB.ID {
		t.Fatalf("跨案件 request_id 污染: 返回 case=%s replayed=%v，期望 case=%s 的首次提交", second.ID, replayed, caseB.ID)
	}
	storedB, err := service.GetCase(context.Background(), caseB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedB.State != domain.StatePendingReview || storedB.Revision != caseB.Revision+1 {
		t.Fatalf("案件 B 未提交: state=%s revision=%d", storedB.State, storedB.Revision)
	}
}

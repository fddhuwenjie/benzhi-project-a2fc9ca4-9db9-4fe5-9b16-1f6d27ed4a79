package application

import (
	"context"
	"testing"
	"time"

	"image-integrity-review/internal/domain"
	"image-integrity-review/internal/repository"
)

func TestQueueFiltersIntersectAndSummariesUseFullMatchSet(t *testing.T) {
	store, err := repository.OpenFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store)
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	service.SetClock(func() time.Time { return now })
	ctx := context.Background()
	editor := domain.Actor{Type: domain.ActorEditor, ID: "editor-1"}
	create := func(requestID, manuscript, section string) *domain.ReviewCase {
		t.Helper()
		item, _, err := service.CreateCase(ctx, CreateCaseInput{
			WriteContext:   WriteContext{Actor: editor, RequestID: requestID, ExpectedRevision: 0},
			ManuscriptCode: manuscript, Title: "队列测试" + manuscript, JournalSection: section,
			Figures: []domain.FigureRecord{
				{FigureLabel: "Figure 1", PanelLabel: "A", Caption: "图 A", ContentDigest: "aaaaaaaaaaaaaaaa", PixelWidth: 100, PixelHeight: 100, ExperimentSource: "实验", RawDataReference: "raw://1"},
				{FigureLabel: "Figure 1", PanelLabel: "B", Caption: "图 B", ContentDigest: "aaaaaaaaaaaaaaaa", PixelWidth: 100, PixelHeight: 100, ExperimentSource: "实验", RawDataReference: "raw://2"},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	methodPending := create("create-method-pending", "QUEUE-001", "方法学")
	create("create-method-draft", "QUEUE-002", "方法学")
	otherPending := create("create-other-pending", "QUEUE-003", "论著")
	for index, item := range []*domain.ReviewCase{methodPending, otherPending} {
		if _, _, err := service.SubmitDraft(ctx, item.ID, WriteContext{Actor: editor, RequestID: "submit-" + string(rune('1'+index)), ExpectedRevision: item.Revision}); err != nil {
			t.Fatal(err)
		}
	}
	filter := QueueFilter{State: domain.StatePendingReview, JournalSection: "方法学", Severity: domain.SeverityCritical, OpenOnly: true, Page: 1, PageSize: 1}
	view, err := service.QueueFiltered(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Cases) != 1 || view.Cases[0].ID != methodPending.ID || view.Summary.Total != 1 || view.Pagination.Total != 1 {
		t.Fatalf("交集筛选结果错误：%+v", view)
	}
	if view.Summary.ByState[string(domain.StatePendingReview)] != 1 || view.Summary.OpenRisksBySeverity[string(domain.SeverityCritical)] != 1 || view.Summary.WaitingRoles["审查员"] != 1 {
		t.Fatalf("筛选全集统计错误：%+v", view.Summary)
	}
	filter.Page = 2
	overflow, err := service.QueueFiltered(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(overflow.Cases) != 0 || overflow.Summary.Total != 1 {
		t.Fatalf("超出末页应保留筛选统计：%+v", overflow)
	}
	filter.Keyword = "不存在"
	filter.Page = 1
	empty, err := service.QueueFiltered(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Summary.Total != 0 || empty.Summary.OpenRisks != 0 || empty.Summary.ByState[string(domain.StatePendingReview)] != 0 {
		t.Fatalf("空结果统计应为零：%+v", empty.Summary)
	}
}

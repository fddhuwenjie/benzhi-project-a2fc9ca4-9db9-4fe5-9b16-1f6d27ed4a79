package domain

import (
	"testing"
	"time"
)

func testCase(t *testing.T) *ReviewCase {
	t.Helper()
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	item, err := NewReviewCase("case-test", "MS-001", "测试稿件", "研究论文", []FigureRecord{
		{FigureLabel: "Figure 1", PanelLabel: "A", Caption: "图 A", ContentDigest: "aaaaaaaaaaaaaaaa", PixelWidth: 100, PixelHeight: 100, ExperimentSource: "实验一"},
		{FigureLabel: "Figure 1", PanelLabel: "B", Caption: "图 B", ContentDigest: "aaaaaaaaaaaaaaaa", PixelWidth: 1000, PixelHeight: 1000, ExperimentSource: "实验一"},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func TestIntegrityRulesAreDeterministic(t *testing.T) {
	item := testCase(t)
	now := item.CreatedAt.Add(time.Minute)
	first := EvaluateIntegrity(item.ID, item.Figures, now)
	second := EvaluateIntegrity(item.ID, item.Figures, now)
	if len(first) != 4 {
		t.Fatalf("风险项数量 = %d，期望 4", len(first))
	}
	for i := range first {
		if first[i].ID != second[i].ID || first[i].RuleCode != second[i].RuleCode || first[i].Evidence != second[i].Evidence {
			t.Fatalf("第 %d 项规则结果不确定", i)
		}
	}
}

func TestCompleteAggregateWorkflowAndFreeze(t *testing.T) {
	item := testCase(t)
	now := item.CreatedAt.Add(time.Minute)
	if err := item.SubmitForChecks(now); err != nil {
		t.Fatal(err)
	}
	if err := item.Claim("reviewer-1", now); err != nil {
		t.Fatal(err)
	}
	for i := range item.Findings {
		if err := item.RecordVerdict(item.Findings[i].ID, VerdictExplain, "请补充", now); err != nil {
			t.Fatal(err)
		}
	}
	if err := item.BeginAuthorResponse(HashAccessToken("token"), now); err != nil {
		t.Fatal(err)
	}
	for i := range item.Findings {
		if err := item.SubmitAuthorResponse(item.Findings[i].ID, "作者说明", "bbbbbbbbbbbbbbbb", "raw://1", now); err != nil {
			t.Fatal(err)
		}
	}
	if err := item.FinishAuthorResponse(now); err != nil {
		t.Fatal(err)
	}
	for i := range item.Findings {
		if err := item.ResolveFinding(item.Findings[i].ID, ResolutionAccepted, "通过", now); err != nil {
			t.Fatal(err)
		}
	}
	if err := item.FinishRecheck(now); err != nil {
		t.Fatal(err)
	}
	if err := item.Archive("终审通过", now); err != nil {
		t.Fatal(err)
	}
	if item.State != StateArchived || item.ArchivedAt == nil {
		t.Fatalf("未归档：%+v", item)
	}
	if err := item.SubmitForChecks(now); AsDomainError(err) == nil || AsDomainError(err).Code != CodeFrozen {
		t.Fatalf("归档后写入未被拒绝：%v", err)
	}
}

func TestCannotSkipIncompleteVerdicts(t *testing.T) {
	item := testCase(t)
	if err := item.SubmitForChecks(item.CreatedAt); err != nil {
		t.Fatal(err)
	}
	if err := item.Claim("reviewer-1", item.CreatedAt); err != nil {
		t.Fatal(err)
	}
	err := item.BeginAuthorResponse(HashAccessToken("token"), item.CreatedAt)
	if AsDomainError(err) == nil || AsDomainError(err).Code != CodeState {
		t.Fatalf("期望状态错误，实际 %v", err)
	}
}

func TestDraftFigureRevisionIsAtomicAndKeepsStableIDs(t *testing.T) {
	item := testCase(t)
	originalRevisionFigures := append([]FigureRecord(nil), item.Figures...)
	invalid := append([]FigureRecord(nil), item.Figures...)
	invalid[1].FigureLabel = invalid[0].FigureLabel
	invalid[1].PanelLabel = invalid[0].PanelLabel
	if _, err := item.ReviseDraftFigures(invalid, item.CreatedAt.Add(time.Minute)); AsDomainError(err) == nil || AsDomainError(err).Code != CodeValidation {
		t.Fatalf("期望整批校验错误，实际 %v", err)
	}
	if item.Figures[1].PanelLabel != originalRevisionFigures[1].PanelLabel {
		t.Fatal("失败修订不应改变图像清单")
	}
	valid := []FigureRecord{item.Figures[0], {FigureLabel: "Figure 2", Caption: "新增图", ContentDigest: "bbbbbbbbbbbbbbbb", PixelWidth: 200, PixelHeight: 300}}
	valid[0].ExperimentSource = "修订后的来源"
	changes, err := item.ReviseDraftFigures(valid, item.CreatedAt.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if item.Figures[0].ID != originalRevisionFigures[0].ID || item.Figures[1].ID == "" {
		t.Fatalf("稳定标识分配错误：%+v", item.Figures)
	}
	if len(changes.Added) != 1 || len(changes.Modified) != 1 || len(changes.Deleted) != 1 {
		t.Fatalf("修订差异错误：%+v", changes)
	}
}

func TestBatchVerdictsValidateBeforeMutation(t *testing.T) {
	item := testCase(t)
	if err := item.SubmitForChecks(item.CreatedAt); err != nil {
		t.Fatal(err)
	}
	if err := item.Claim("reviewer-1", item.CreatedAt); err != nil {
		t.Fatal(err)
	}
	changes := []VerdictChange{
		{FindingID: item.Findings[0].ID, Verdict: VerdictExplain, Note: "有效"},
		{FindingID: "missing", Verdict: VerdictExcluded, Note: "不存在"},
	}
	if err := item.RecordVerdicts(changes, item.CreatedAt); AsDomainError(err) == nil || AsDomainError(err).Code != CodeNotFound {
		t.Fatalf("期望未找到错误，实际 %v", err)
	}
	if item.Findings[0].ReviewVerdict != "" {
		t.Fatal("批量失败不应保留前序判读")
	}
	changes[1].FindingID = item.Findings[1].ID
	if err := item.RecordVerdicts(changes, item.CreatedAt); err != nil {
		t.Fatal(err)
	}
	if item.Findings[0].ReviewVerdict != VerdictExplain || item.Findings[1].ReviewVerdict != VerdictExcluded {
		t.Fatal("批量判读未完整应用")
	}
}

func TestSupplementRoundsFreezeHistoryAndReopenRejectedOnly(t *testing.T) {
	item := testCase(t)
	now := item.CreatedAt.Add(time.Minute)
	if err := item.SubmitForChecks(now); err != nil {
		t.Fatal(err)
	}
	if err := item.Claim("reviewer-1", now); err != nil {
		t.Fatal(err)
	}
	changes := make([]VerdictChange, len(item.Findings))
	for i := range item.Findings {
		changes[i] = VerdictChange{FindingID: item.Findings[i].ID, Verdict: VerdictExplain, Note: "补充说明"}
	}
	if err := item.RecordVerdicts(changes, now); err != nil {
		t.Fatal(err)
	}
	if err := item.BeginAuthorResponse(HashAccessToken("token"), now); err != nil {
		t.Fatal(err)
	}
	for i := range item.Findings {
		if err := item.SubmitAuthorResponseForRound(1, item.Findings[i].ID, "第一轮说明", "bbbbbbbbbbbbbbbb", "raw://round/1", now); err != nil {
			t.Fatal(err)
		}
	}
	if err := item.FinishAuthorResponseForRound(1, now); err != nil {
		t.Fatal(err)
	}
	for i := range item.Findings {
		resolution := ResolutionAccepted
		if i == 1 {
			resolution = ResolutionRejected
		}
		if err := item.ResolveFindingForRound(1, item.Findings[i].ID, resolution, "第一轮复核", now); err != nil {
			t.Fatal(err)
		}
	}
	if err := item.FinishRecheck(now); err != nil {
		t.Fatal(err)
	}
	reopenedID := item.Findings[1].ID
	if err := item.ReturnForSupplement("补充第二项证据", now); err != nil {
		t.Fatal(err)
	}
	if item.CurrentRound != 2 || len(item.ResponseRounds) != 2 || len(item.ResponseRounds[1].Findings) != 1 || item.ResponseRounds[1].Findings[0].FindingID != reopenedID {
		t.Fatalf("退回轮次错误：%+v", item.ResponseRounds)
	}
	if item.ResponseRounds[0].Findings[1].AuthorExplanation != "第一轮说明" || item.ResponseRounds[0].ReturnReason == "" {
		t.Fatal("第一轮证据未完整冻结")
	}
	if err := item.SubmitAuthorResponseForRound(1, reopenedID, "覆盖旧轮", "cccccccccccccccc", "raw://old", now); AsDomainError(err) == nil || AsDomainError(err).Code != CodeState {
		t.Fatalf("旧轮次写入应冲突，实际 %v", err)
	}
	if item.Findings[0].Resolution != ResolutionAccepted || item.Findings[1].ResponseStatus != ResponsePending {
		t.Fatal("退回应只重新开放拒绝项")
	}
}

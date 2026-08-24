package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"sort"
	"strings"
	"time"
)

type FigureRevisionChanges struct {
	Added    []string `json:"added"`
	Modified []string `json:"modified"`
	Deleted  []string `json:"deleted"`
}

type VerdictChange struct {
	FindingID string        `json:"finding_id"`
	Verdict   ReviewVerdict `json:"verdict"`
	Note      string        `json:"note"`
}

func NewReviewCase(id, manuscriptCode, title, section string, figures []FigureRecord, now time.Time) (*ReviewCase, error) {
	if err := ValidateNewCase(manuscriptCode, title, section, figures); err != nil {
		return nil, err
	}
	item := &ReviewCase{
		ID:               id,
		ManuscriptCode:   strings.TrimSpace(manuscriptCode),
		Title:            strings.TrimSpace(title),
		JournalSection:   strings.TrimSpace(section),
		State:            StateDraft,
		Revision:         1,
		NextFigureNumber: len(figures) + 1,
		CreatedAt:        now.UTC(),
		UpdatedAt:        now.UTC(),
		Figures:          make([]FigureRecord, len(figures)),
		Findings:         []RiskFinding{},
		ResponseRounds:   []ResponseRound{},
	}
	for i, figure := range figures {
		figure.ID = id + "-fig-" + itoa(i+1)
		figure.CaseID = id
		figure = NormalizeFigure(figure)
		figure.CreatedAt = now.UTC()
		item.Figures[i] = figure
	}
	return item, nil
}

func (c *ReviewCase) ReviseDraftFigures(figures []FigureRecord, now time.Time) (FigureRevisionChanges, error) {
	if err := c.EnsureMutable("修订图像清单"); err != nil {
		return FigureRevisionChanges{}, err
	}
	if c.State != StateDraft {
		return FigureRevisionChanges{}, StateError(c.State, "修订图像清单")
	}
	if len(figures) > 200 {
		return FigureRevisionChanges{}, ValidationError(FieldError{Field: "figures", Message: "图像数量不能超过 200"})
	}
	existing := make(map[string]FigureRecord, len(c.Figures))
	for _, figure := range c.Figures {
		existing[figure.ID] = figure
	}
	seenIDs := map[string]bool{}
	normalized := make([]FigureRecord, len(figures))
	changes := FigureRevisionChanges{}
	next := c.NextFigureNumber
	if next <= 0 {
		next = len(c.Figures) + 1
	}
	fields := make([]FieldError, 0)
	for i, figure := range figures {
		figure = NormalizeFigure(figure)
		prefix := "figures[" + itoa(i) + "].id"
		if figure.ID == "" {
			figure.ID = c.ID + "-fig-" + itoa(next)
			next++
			figure.CreatedAt = now.UTC()
			changes.Added = append(changes.Added, figure.ID)
		} else {
			old, ok := existing[figure.ID]
			if !ok {
				fields = append(fields, FieldError{Field: prefix, Message: "图像标识不属于当前案件"})
			} else {
				figure.CreatedAt = old.CreatedAt
				comparableOld := old
				comparableOld.CaseID = ""
				comparableNew := figure
				comparableNew.CaseID = ""
				if !reflect.DeepEqual(comparableOld, comparableNew) {
					changes.Modified = append(changes.Modified, figure.ID)
				}
			}
		}
		if seenIDs[figure.ID] {
			fields = append(fields, FieldError{Field: prefix, Message: "图像标识不能重复"})
		}
		seenIDs[figure.ID] = true
		figure.CaseID = c.ID
		normalized[i] = figure
	}
	for id := range existing {
		if !seenIDs[id] {
			changes.Deleted = append(changes.Deleted, id)
		}
	}
	sort.Strings(changes.Added)
	sort.Strings(changes.Modified)
	sort.Strings(changes.Deleted)
	if err := ValidateFigureList(normalized); err != nil {
		if value := AsDomainError(err); value != nil {
			fields = append(fields, value.Fields...)
		}
	}
	if len(fields) > 0 {
		return FigureRevisionChanges{}, ValidationError(fields...)
	}
	c.Figures = normalized
	c.NextFigureNumber = next
	c.UpdatedAt = now.UTC()
	return changes, nil
}

func (c *ReviewCase) DraftPreflight(now time.Time) ([]RiskFinding, error) {
	if c.State != StateDraft {
		return nil, StateError(c.State, "草稿核验预检")
	}
	return EvaluateIntegrity(c.ID, c.Figures, now), nil
}

func (c *ReviewCase) EnsureMutable(action string) error {
	if c.State == StateArchived {
		return NewError(CodeFrozen, "归档案件已冻结，不能执行"+action)
	}
	return nil
}

func (c *ReviewCase) SubmitForChecks(now time.Time) error {
	if err := c.EnsureMutable("提交核验"); err != nil {
		return err
	}
	if c.State != StateDraft {
		return StateError(c.State, "提交核验")
	}
	c.RuleVersion = IntegrityRuleVersion
	c.Findings = EvaluateIntegrity(c.ID, c.Figures, now)
	c.State = StatePendingReview
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *ReviewCase) Claim(reviewerID string, now time.Time) error {
	if err := c.EnsureMutable("领取案件"); err != nil {
		return err
	}
	if c.State != StatePendingReview {
		return StateError(c.State, "领取案件")
	}
	if strings.TrimSpace(reviewerID) == "" {
		return ValidationError(FieldError{Field: "actor_id", Message: "审查员标识不能为空"})
	}
	c.AssigneeID = strings.TrimSpace(reviewerID)
	c.State = StateInReview
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *ReviewCase) RecordVerdict(findingID string, verdict ReviewVerdict, note string, now time.Time) error {
	if err := c.EnsureMutable("记录判读"); err != nil {
		return err
	}
	if c.State != StateInReview {
		return StateError(c.State, "记录判读")
	}
	if verdict != VerdictEstablished && verdict != VerdictExcluded && verdict != VerdictExplain {
		return ValidationError(FieldError{Field: "verdict", Message: "判读结论无效"})
	}
	finding := c.findFinding(findingID)
	if finding == nil {
		return NewError(CodeNotFound, "风险项不存在")
	}
	finding.ReviewVerdict = verdict
	finding.ReviewNote = strings.TrimSpace(note)
	finding.UpdatedAt = now.UTC()
	if verdict == VerdictExplain || verdict == VerdictEstablished {
		finding.ResponseStatus = ResponsePending
	} else {
		finding.ResponseStatus = ResponseNotRequired
	}
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *ReviewCase) RecordVerdicts(changes []VerdictChange, now time.Time) error {
	if err := c.EnsureMutable("批量记录判读"); err != nil {
		return err
	}
	if c.State != StateInReview {
		return StateError(c.State, "批量记录判读")
	}
	if len(changes) == 0 {
		return ValidationError(FieldError{Field: "items", Message: "至少选择一个风险项"})
	}
	if len(changes) > 100 {
		return ValidationError(FieldError{Field: "items", Message: "单次最多判读 100 个风险项"})
	}
	seen := map[string]bool{}
	for i, change := range changes {
		prefix := "items[" + itoa(i) + "]."
		if strings.TrimSpace(change.FindingID) == "" {
			return ValidationError(FieldError{Field: prefix + "finding_id", Message: "风险项标识不能为空"})
		}
		if seen[change.FindingID] {
			return ValidationError(FieldError{Field: prefix + "finding_id", Message: "风险项标识不能重复"})
		}
		seen[change.FindingID] = true
		if change.Verdict != VerdictEstablished && change.Verdict != VerdictExcluded && change.Verdict != VerdictExplain {
			return ValidationError(FieldError{Field: prefix + "verdict", Message: "判读结论无效"})
		}
		if c.findFinding(change.FindingID) == nil {
			return NewError(CodeNotFound, "风险项不存在："+change.FindingID)
		}
	}
	for _, change := range changes {
		finding := c.findFinding(change.FindingID)
		finding.ReviewVerdict = change.Verdict
		finding.ReviewNote = strings.TrimSpace(change.Note)
		finding.UpdatedAt = now.UTC()
		if change.Verdict == VerdictExplain || change.Verdict == VerdictEstablished {
			finding.ResponseStatus = ResponsePending
		} else {
			finding.ResponseStatus = ResponseNotRequired
		}
	}
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *ReviewCase) BeginAuthorResponse(accessHash string, now time.Time) error {
	if err := c.EnsureMutable("发起作者回应"); err != nil {
		return err
	}
	if c.State != StateInReview {
		return StateError(c.State, "发起作者回应")
	}
	for i := range c.Findings {
		if c.Findings[i].ReviewVerdict == "" {
			return NewError(CodeState, "全部风险项完成判读后才能发起作者回应")
		}
	}
	if strings.TrimSpace(accessHash) == "" {
		return NewError(CodeValidation, "作者访问凭据不能为空")
	}
	if c.CurrentRound != 0 || len(c.ResponseRounds) != 0 {
		return NewError(CodeState, "作者回应轮次已经建立")
	}
	c.AuthorAccessHash = accessHash
	c.CurrentRound = 1
	c.ResponseRounds = append(c.ResponseRounds, ResponseRound{
		Number:    1,
		Status:    "active",
		StartedAt: now.UTC(),
		Findings:  roundEvidence(c.Findings, nil),
	})
	c.State = StateAwaitingAuthor
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *ReviewCase) SubmitAuthorResponse(findingID, explanation, replacementDigest, rawReference string, now time.Time) error {
	return c.SubmitAuthorResponseForRound(c.CurrentRound, findingID, explanation, replacementDigest, rawReference, now)
}

func (c *ReviewCase) SubmitAuthorResponseForRound(roundNumber int, findingID, explanation, replacementDigest, rawReference string, now time.Time) error {
	if err := c.EnsureMutable("提交作者回应"); err != nil {
		return err
	}
	if c.State != StateAwaitingAuthor {
		return StateError(c.State, "提交作者回应")
	}
	if err := c.requireActiveRound(roundNumber); err != nil {
		return err
	}
	finding := c.findFinding(findingID)
	if finding == nil {
		return NewError(CodeNotFound, "风险项不存在")
	}
	if finding.ResponseStatus == ResponseNotRequired {
		return NewError(CodeState, "该风险项无需作者回应")
	}
	if strings.TrimSpace(explanation) == "" {
		return ValidationError(FieldError{Field: "explanation", Message: "结构化说明不能为空"})
	}
	if replacementDigest != "" && !digestPattern.MatchString(replacementDigest) {
		return ValidationError(FieldError{Field: "replacement_digest", Message: "替换图像摘要格式无效"})
	}
	finding.AuthorExplanation = strings.TrimSpace(explanation)
	finding.ReplacementDigest = strings.ToLower(strings.TrimSpace(replacementDigest))
	finding.RawDataReference = strings.TrimSpace(rawReference)
	finding.ResponseStatus = ResponseSubmitted
	finding.Resolution = ""
	finding.ResolutionNote = ""
	finding.UpdatedAt = now.UTC()
	c.syncActiveRound()
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *ReviewCase) FinishAuthorResponse(now time.Time) error {
	return c.FinishAuthorResponseForRound(c.CurrentRound, now)
}

func (c *ReviewCase) FinishAuthorResponseForRound(roundNumber int, now time.Time) error {
	if err := c.EnsureMutable("完成作者回应"); err != nil {
		return err
	}
	if c.State != StateAwaitingAuthor {
		return StateError(c.State, "完成作者回应")
	}
	if err := c.requireActiveRound(roundNumber); err != nil {
		return err
	}
	for i := range c.Findings {
		if c.Findings[i].ResponseStatus == ResponsePending {
			return NewError(CodeState, "所有待回应风险项均提交后才能进入复核")
		}
	}
	c.syncActiveRound()
	c.State = StateAwaitingRecheck
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *ReviewCase) ResolveFinding(findingID string, resolution Resolution, note string, now time.Time) error {
	return c.ResolveFindingForRound(c.CurrentRound, findingID, resolution, note, now)
}

func (c *ReviewCase) ResolveFindingForRound(roundNumber int, findingID string, resolution Resolution, note string, now time.Time) error {
	if err := c.EnsureMutable("复核作者回应"); err != nil {
		return err
	}
	if c.State != StateAwaitingRecheck {
		return StateError(c.State, "复核作者回应")
	}
	if err := c.requireActiveRound(roundNumber); err != nil {
		return err
	}
	if resolution != ResolutionAccepted && resolution != ResolutionRejected {
		return ValidationError(FieldError{Field: "resolution", Message: "复核结论无效"})
	}
	finding := c.findFinding(findingID)
	if finding == nil {
		return NewError(CodeNotFound, "风险项不存在")
	}
	if finding.ResponseStatus == ResponseNotRequired {
		return NewError(CodeState, "无需回应的风险项不能复核")
	}
	finding.Resolution = resolution
	finding.ResolutionNote = strings.TrimSpace(note)
	finding.UpdatedAt = now.UTC()
	c.syncActiveRound()
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *ReviewCase) FinishRecheck(now time.Time) error {
	if err := c.EnsureMutable("完成复核"); err != nil {
		return err
	}
	if c.State != StateAwaitingRecheck {
		return StateError(c.State, "完成复核")
	}
	for i := range c.Findings {
		if c.Findings[i].ResponseStatus == ResponseSubmitted && c.Findings[i].Resolution == "" {
			return NewError(CodeState, "全部作者回应完成复核后才能提交终审")
		}
	}
	c.syncActiveRound()
	if round := c.activeRound(); round != nil {
		round.Status = "reviewed"
	}
	c.State = StateReadyDecision
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *ReviewCase) ReturnForSupplement(note string, now time.Time) error {
	if err := c.EnsureMutable("退回补充"); err != nil {
		return err
	}
	if c.State != StateReadyDecision {
		return StateError(c.State, "退回补充")
	}
	if strings.TrimSpace(note) == "" {
		return ValidationError(FieldError{Field: "note", Message: "退回原因不能为空"})
	}
	rejected := make([]string, 0)
	for i := range c.Findings {
		if c.Findings[i].Resolution == ResolutionRejected {
			rejected = append(rejected, c.Findings[i].ID)
		}
	}
	if len(rejected) == 0 {
		return NewError(CodeState, "没有复核拒绝的风险项，应选择通过归档")
	}
	c.syncActiveRound()
	stamp := now.UTC()
	if round := c.activeRound(); round != nil {
		round.Status = "returned"
		round.ReturnReason = strings.TrimSpace(note)
		round.ClosedAt = &stamp
	}
	for i := range c.Findings {
		if c.Findings[i].Resolution != ResolutionRejected {
			continue
		}
		c.Findings[i].AuthorExplanation = ""
		c.Findings[i].ReplacementDigest = ""
		c.Findings[i].RawDataReference = ""
		c.Findings[i].ResponseStatus = ResponsePending
		c.Findings[i].Resolution = ""
		c.Findings[i].ResolutionNote = ""
		c.Findings[i].UpdatedAt = stamp
	}
	c.CurrentRound++
	c.ResponseRounds = append(c.ResponseRounds, ResponseRound{
		Number:    c.CurrentRound,
		Status:    "active",
		StartedAt: stamp,
		Findings:  roundEvidence(c.Findings, rejected),
	})
	c.FinalDecision = "returned"
	c.DecisionNote = strings.TrimSpace(note)
	c.State = StateAwaitingAuthor
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *ReviewCase) Archive(note string, at time.Time) error {
	if err := c.EnsureMutable("通过归档"); err != nil {
		return err
	}
	if c.State != StateReadyDecision {
		return StateError(c.State, "通过归档")
	}
	for i := range c.Findings {
		if c.Findings[i].Resolution == ResolutionRejected {
			return NewError(CodeState, "存在复核不通过的风险项，不能归档")
		}
	}
	c.FinalDecision = "approved"
	c.DecisionNote = strings.TrimSpace(note)
	c.State = StateArchived
	stamp := at.UTC()
	c.syncActiveRound()
	if round := c.activeRound(); round != nil {
		round.Status = "approved"
		round.ClosedAt = &stamp
	}
	c.ArchivedAt = &stamp
	c.UpdatedAt = stamp
	return nil
}

func (c *ReviewCase) requireActiveRound(roundNumber int) error {
	if roundNumber <= 0 {
		return ValidationError(FieldError{Field: "round_number", Message: "轮次号必须为正整数"})
	}
	if roundNumber != c.CurrentRound {
		return NewError(CodeState, "回应轮次已关闭，请刷新后提交当前轮次")
	}
	round := c.activeRound()
	if round == nil || (round.Status != "active" && round.Status != "reviewed") {
		return NewError(CodeState, "当前回应轮次不可修改")
	}
	return nil
}

func (c *ReviewCase) activeRound() *ResponseRound {
	for i := range c.ResponseRounds {
		if c.ResponseRounds[i].Number == c.CurrentRound {
			return &c.ResponseRounds[i]
		}
	}
	return nil
}

func (c *ReviewCase) syncActiveRound() {
	round := c.activeRound()
	if round == nil {
		return
	}
	ids := make([]string, 0, len(round.Findings))
	for _, evidence := range round.Findings {
		ids = append(ids, evidence.FindingID)
	}
	round.Findings = roundEvidence(c.Findings, ids)
}

func roundEvidence(findings []RiskFinding, onlyIDs []string) []RoundFindingEvidence {
	include := map[string]bool{}
	for _, id := range onlyIDs {
		include[id] = true
	}
	result := make([]RoundFindingEvidence, 0, len(findings))
	for _, finding := range findings {
		if onlyIDs != nil && !include[finding.ID] {
			continue
		}
		result = append(result, RoundFindingEvidence{
			FindingID:         finding.ID,
			ReviewVerdict:     finding.ReviewVerdict,
			ReviewNote:        finding.ReviewNote,
			AuthorExplanation: finding.AuthorExplanation,
			ReplacementDigest: finding.ReplacementDigest,
			RawDataReference:  finding.RawDataReference,
			ResponseStatus:    finding.ResponseStatus,
			Resolution:        finding.Resolution,
			ResolutionNote:    finding.ResolutionNote,
			UpdatedAt:         finding.UpdatedAt,
		})
	}
	return result
}

func (c *ReviewCase) findFinding(id string) *RiskFinding {
	for i := range c.Findings {
		if c.Findings[i].ID == id {
			return &c.Findings[i]
		}
	}
	return nil
}

func HashAccessToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

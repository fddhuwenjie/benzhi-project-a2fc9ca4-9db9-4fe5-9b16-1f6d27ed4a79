package application

import (
	"context"
	"sort"

	"image-integrity-review/internal/domain"
	"image-integrity-review/internal/repository"
)

type QueueSummary struct {
	Total               int            `json:"total"`
	Active              int            `json:"active"`
	Archived            int            `json:"archived"`
	ByState             map[string]int `json:"by_state"`
	OpenRisks           int            `json:"open_risks"`
	OpenRisksBySeverity map[string]int `json:"open_risks_by_severity"`
	WaitingRoles        map[string]int `json:"waiting_roles"`
	WaitingRole         string         `json:"waiting_role,omitempty"`
}

type QueueView struct {
	Cases      []domain.ReviewCase `json:"cases"`
	Summary    QueueSummary        `json:"summary"`
	Pagination QueuePagination     `json:"pagination"`
}

type QueuePagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type CaseProgress struct {
	TotalFindings      int `json:"total_findings"`
	ReviewedFindings   int `json:"reviewed_findings"`
	PendingResponses   int `json:"pending_responses"`
	SubmittedResponses int `json:"submitted_responses"`
	ResolvedResponses  int `json:"resolved_responses"`
}

func (s *Service) Queue(ctx context.Context) (QueueView, error) {
	return s.QueueFiltered(ctx, QueueFilter{Page: 1, PageSize: 20})
}

func (s *Service) QueueFiltered(ctx context.Context, filter QueueFilter) (QueueView, error) {
	if err := validateQueueFilter(filter); err != nil {
		return QueueView{}, err
	}
	result, err := s.store.Query(ctx, repository.CaseQuery{
		State: filter.State, JournalSection: filter.JournalSection, AssigneeID: filter.AssigneeID,
		Keyword: filter.Keyword, Severity: filter.Severity, OpenOnly: filter.OpenOnly,
		Page: filter.Page, PageSize: filter.PageSize,
	})
	if err != nil {
		return QueueView{}, err
	}
	view := QueueView{
		Cases: result.Cases,
		Summary: QueueSummary{
			Total:               len(result.Matches),
			ByState:             map[string]int{},
			OpenRisksBySeverity: map[string]int{},
			WaitingRoles:        map[string]int{"责任编辑": 0, "审查员": 0, "作者": 0},
		},
		Pagination: QueuePagination{Page: filter.Page, PageSize: filter.PageSize, Total: len(result.Matches)},
	}
	for _, state := range domain.CaseStates {
		view.Summary.ByState[string(state)] = 0
	}
	for _, severity := range domain.Severities {
		view.Summary.OpenRisksBySeverity[string(severity)] = 0
	}
	for i := range result.Matches {
		item := &result.Matches[i]
		view.Summary.ByState[string(item.State)]++
		if item.State == domain.StateArchived {
			view.Summary.Archived++
		} else {
			view.Summary.Active++
		}
		for _, finding := range item.Findings {
			if domain.FindingOpen(finding) {
				view.Summary.OpenRisks++
				view.Summary.OpenRisksBySeverity[string(finding.Severity)]++
			}
		}
		if role := domain.WaitingRole(item.State); role != "" {
			view.Summary.WaitingRoles[role]++
		}
	}
	type roleCount struct {
		role  string
		count int
	}
	roles := make([]roleCount, 0, len(view.Summary.WaitingRoles))
	for role, count := range view.Summary.WaitingRoles {
		roles = append(roles, roleCount{role: role, count: count})
	}
	sort.Slice(roles, func(i, j int) bool {
		if roles[i].count == roles[j].count {
			return roles[i].role < roles[j].role
		}
		return roles[i].count > roles[j].count
	})
	if len(roles) > 0 && roles[0].count > 0 {
		view.Summary.WaitingRole = roles[0].role
	}
	if view.Pagination.Total > 0 {
		view.Pagination.TotalPages = (view.Pagination.Total + view.Pagination.PageSize - 1) / view.Pagination.PageSize
	}
	return view, nil
}

func validateQueueFilter(filter QueueFilter) error {
	fields := make([]domain.FieldError, 0)
	if filter.State != "" && !domain.ValidCaseState(filter.State) {
		fields = append(fields, domain.FieldError{Field: "status", Message: "案件状态无效"})
	}
	if filter.Severity != "" && !domain.ValidSeverity(filter.Severity) {
		fields = append(fields, domain.FieldError{Field: "severity", Message: "风险严重级别无效"})
	}
	if filter.Page < 1 || filter.Page > 10000 {
		fields = append(fields, domain.FieldError{Field: "page", Message: "page 必须在 1 到 10000 之间"})
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		fields = append(fields, domain.FieldError{Field: "page_size", Message: "page_size 必须在 1 到 100 之间"})
	}
	if len(filter.Keyword) > 200 {
		fields = append(fields, domain.FieldError{Field: "q", Message: "关键词不能超过 200 个字符"})
	}
	if len(fields) > 0 {
		return domain.ValidationError(fields...)
	}
	return nil
}

func ProgressFor(item *domain.ReviewCase) CaseProgress {
	progress := CaseProgress{TotalFindings: len(item.Findings)}
	for i := range item.Findings {
		finding := &item.Findings[i]
		if finding.ReviewVerdict != "" {
			progress.ReviewedFindings++
		}
		switch finding.ResponseStatus {
		case domain.ResponsePending:
			progress.PendingResponses++
		case domain.ResponseSubmitted:
			progress.SubmittedResponses++
		}
		if finding.Resolution != "" || finding.ResponseStatus == domain.ResponseNotRequired {
			progress.ResolvedResponses++
		}
	}
	return progress
}

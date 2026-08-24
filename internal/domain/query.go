package domain

import "strings"

var CaseStates = []CaseState{
	StateDraft,
	StatePendingReview,
	StateInReview,
	StateAwaitingAuthor,
	StateAwaitingRecheck,
	StateReadyDecision,
	StateArchived,
}

var Severities = []Severity{
	SeverityLow,
	SeverityMedium,
	SeverityHigh,
	SeverityCritical,
}

func ValidCaseState(value CaseState) bool {
	for _, candidate := range CaseStates {
		if value == candidate {
			return true
		}
	}
	return false
}

func ValidSeverity(value Severity) bool {
	for _, candidate := range Severities {
		if value == candidate {
			return true
		}
	}
	return false
}

func FindingOpen(finding RiskFinding) bool {
	if finding.ReviewVerdict == "" {
		return true
	}
	if finding.ReviewVerdict == VerdictExcluded || finding.ResponseStatus == ResponseNotRequired {
		return false
	}
	return finding.Resolution != ResolutionAccepted
}

func WaitingRole(state CaseState) string {
	switch state {
	case StateDraft, StateReadyDecision:
		return "责任编辑"
	case StatePendingReview, StateInReview, StateAwaitingRecheck:
		return "审查员"
	case StateAwaitingAuthor:
		return "作者"
	default:
		return ""
	}
}

func CaseMatches(item *ReviewCase, state CaseState, section, assignee, keyword string, severity Severity, openOnly bool) bool {
	if state != "" && item.State != state {
		return false
	}
	if section != "" && !strings.EqualFold(strings.TrimSpace(item.JournalSection), strings.TrimSpace(section)) {
		return false
	}
	if assignee != "" && !strings.EqualFold(strings.TrimSpace(item.AssigneeID), strings.TrimSpace(assignee)) {
		return false
	}
	if keyword != "" {
		needle := strings.ToLower(strings.TrimSpace(keyword))
		if !strings.Contains(strings.ToLower(item.ManuscriptCode), needle) && !strings.Contains(strings.ToLower(item.Title), needle) {
			return false
		}
	}
	if severity != "" || openOnly {
		matched := false
		for _, finding := range item.Findings {
			if severity != "" && finding.Severity != severity {
				continue
			}
			if openOnly && !FindingOpen(finding) {
				continue
			}
			matched = true
			break
		}
		if !matched {
			return false
		}
	}
	return true
}

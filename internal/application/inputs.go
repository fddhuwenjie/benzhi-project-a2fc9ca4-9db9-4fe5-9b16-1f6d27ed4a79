package application

import "image-integrity-review/internal/domain"

type WriteContext struct {
	Actor            domain.Actor `json:"actor"`
	RequestID        string       `json:"request_id"`
	ExpectedRevision int64        `json:"expected_revision"`
}

type CreateCaseInput struct {
	WriteContext
	ManuscriptCode string                `json:"manuscript_code"`
	Title          string                `json:"title"`
	JournalSection string                `json:"journal_section"`
	Figures        []domain.FigureRecord `json:"figures"`
}

type VerdictInput struct {
	WriteContext
	FindingID string               `json:"finding_id"`
	Verdict   domain.ReviewVerdict `json:"verdict"`
	Note      string               `json:"note"`
}

type BatchVerdictInput struct {
	WriteContext
	Items []domain.VerdictChange `json:"items"`
}

type ReviseFiguresInput struct {
	WriteContext
	Figures []domain.FigureRecord `json:"figures"`
}

type QueueFilter struct {
	State          domain.CaseState
	JournalSection string
	AssigneeID     string
	Keyword        string
	Severity       domain.Severity
	OpenOnly       bool
	Page           int
	PageSize       int
}

type AuthorResponseInput struct {
	WriteContext
	AccessToken       string `json:"access_token"`
	FindingID         string `json:"finding_id"`
	Explanation       string `json:"explanation"`
	ReplacementDigest string `json:"replacement_digest"`
	RawDataReference  string `json:"raw_data_reference"`
	RoundNumber       int    `json:"round_number"`
}

type ResolutionInput struct {
	WriteContext
	FindingID   string            `json:"finding_id"`
	Resolution  domain.Resolution `json:"resolution"`
	Note        string            `json:"note"`
	RoundNumber int               `json:"round_number"`
}

type DecisionInput struct {
	WriteContext
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

type AccessCredential struct {
	Case        *domain.ReviewCase `json:"case"`
	AccessToken string             `json:"access_token,omitempty"`
	Replayed    bool               `json:"replayed"`
}

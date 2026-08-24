package domain

import (
	"encoding/json"
	"time"
)

type CaseState string

const (
	StateDraft           CaseState = "draft"
	StatePendingReview   CaseState = "pending_review"
	StateInReview        CaseState = "in_review"
	StateAwaitingAuthor  CaseState = "awaiting_author"
	StateAwaitingRecheck CaseState = "awaiting_recheck"
	StateReadyDecision   CaseState = "ready_decision"
	StateArchived        CaseState = "archived"
)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type ReviewVerdict string

const (
	VerdictEstablished ReviewVerdict = "established"
	VerdictExcluded    ReviewVerdict = "excluded"
	VerdictExplain     ReviewVerdict = "needs_explanation"
)

type ResponseStatus string

const (
	ResponseNotRequired ResponseStatus = "not_required"
	ResponsePending     ResponseStatus = "pending"
	ResponseSubmitted   ResponseStatus = "submitted"
)

type Resolution string

const (
	ResolutionAccepted Resolution = "accepted"
	ResolutionRejected Resolution = "rejected"
)

type ActorType string

const (
	ActorEditor   ActorType = "editor"
	ActorReviewer ActorType = "reviewer"
	ActorAuthor   ActorType = "author"
	ActorSystem   ActorType = "system"
)

type FigureRecord struct {
	ID               string    `json:"id"`
	CaseID           string    `json:"case_id"`
	FigureLabel      string    `json:"figure_label"`
	PanelLabel       string    `json:"panel_label,omitempty"`
	Caption          string    `json:"caption"`
	ContentDigest    string    `json:"content_digest"`
	PixelWidth       int       `json:"pixel_width"`
	PixelHeight      int       `json:"pixel_height"`
	ExperimentSource string    `json:"experiment_source"`
	RawDataReference string    `json:"raw_data_reference"`
	ReplacementOf    string    `json:"replacement_of,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type RiskFinding struct {
	ID                string         `json:"id"`
	CaseID            string         `json:"case_id"`
	FigureIDs         []string       `json:"figure_ids"`
	RuleCode          string         `json:"rule_code"`
	RuleVersion       string         `json:"rule_version"`
	Severity          Severity       `json:"severity"`
	Evidence          string         `json:"evidence"`
	ReviewVerdict     ReviewVerdict  `json:"review_verdict,omitempty"`
	ReviewNote        string         `json:"review_note,omitempty"`
	AuthorExplanation string         `json:"author_explanation,omitempty"`
	ReplacementDigest string         `json:"replacement_digest,omitempty"`
	RawDataReference  string         `json:"raw_data_reference,omitempty"`
	ResponseStatus    ResponseStatus `json:"response_status"`
	Resolution        Resolution     `json:"resolution,omitempty"`
	ResolutionNote    string         `json:"resolution_note,omitempty"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type RoundFindingEvidence struct {
	FindingID         string         `json:"finding_id"`
	ReviewVerdict     ReviewVerdict  `json:"review_verdict,omitempty"`
	ReviewNote        string         `json:"review_note,omitempty"`
	AuthorExplanation string         `json:"author_explanation,omitempty"`
	ReplacementDigest string         `json:"replacement_digest,omitempty"`
	RawDataReference  string         `json:"raw_data_reference,omitempty"`
	ResponseStatus    ResponseStatus `json:"response_status"`
	Resolution        Resolution     `json:"resolution,omitempty"`
	ResolutionNote    string         `json:"resolution_note,omitempty"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type ResponseRound struct {
	Number       int                    `json:"number"`
	Status       string                 `json:"status"`
	StartedAt    time.Time              `json:"started_at"`
	ClosedAt     *time.Time             `json:"closed_at,omitempty"`
	ReturnReason string                 `json:"return_reason,omitempty"`
	Findings     []RoundFindingEvidence `json:"findings"`
}

type ReviewCase struct {
	ID               string          `json:"id"`
	ManuscriptCode   string          `json:"manuscript_code"`
	Title            string          `json:"title"`
	JournalSection   string          `json:"journal_section"`
	State            CaseState       `json:"state"`
	AssigneeID       string          `json:"assignee_id,omitempty"`
	AuthorAccessHash string          `json:"author_access_hash,omitempty"`
	RuleVersion      string          `json:"rule_version,omitempty"`
	Revision         int64           `json:"revision"`
	NextFigureNumber int             `json:"next_figure_number,omitempty"`
	Figures          []FigureRecord  `json:"figures"`
	Findings         []RiskFinding   `json:"findings"`
	CurrentRound     int             `json:"current_round,omitempty"`
	ResponseRounds   []ResponseRound `json:"response_rounds,omitempty"`
	FinalDecision    string          `json:"final_decision,omitempty"`
	DecisionNote     string          `json:"decision_note,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	ArchivedAt       *time.Time      `json:"archived_at,omitempty"`
	ArchiveDigest    string          `json:"archive_digest,omitempty"`
}

type AuditEvent struct {
	EventID          string          `json:"event_id"`
	CaseID           string          `json:"case_id"`
	EventType        string          `json:"event_type"`
	ActorType        ActorType       `json:"actor_type"`
	ActorID          string          `json:"actor_id"`
	RequestID        string          `json:"request_id"`
	ExpectedRevision int64           `json:"expected_revision"`
	ResultRevision   int64           `json:"result_revision"`
	FromState        CaseState       `json:"from_state,omitempty"`
	ToState          CaseState       `json:"to_state"`
	Payload          json.RawMessage `json:"payload"`
	OccurredAt       time.Time       `json:"occurred_at"`
}

type Actor struct {
	Type ActorType `json:"type"`
	ID   string    `json:"id"`
}

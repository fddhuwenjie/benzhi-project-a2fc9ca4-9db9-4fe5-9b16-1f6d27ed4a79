package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

type ArchiveDocument struct {
	SchemaVersion  string          `json:"schema_version"`
	CaseID         string          `json:"case_id"`
	ManuscriptCode string          `json:"manuscript_code"`
	Title          string          `json:"title"`
	JournalSection string          `json:"journal_section"`
	RuleVersion    string          `json:"rule_version"`
	Figures        []FigureRecord  `json:"figures"`
	Findings       []RiskFinding   `json:"findings"`
	ResponseRounds []ResponseRound `json:"response_rounds"`
	FinalDecision  string          `json:"final_decision"`
	DecisionNote   string          `json:"decision_note,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	ArchivedAt     time.Time       `json:"archived_at"`
	Timeline       []AuditEvent    `json:"timeline"`
	Digest         string          `json:"digest,omitempty"`
}

func CanonicalArchiveBytes(document ArchiveDocument) ([]byte, error) {
	document.Digest = ""
	return json.Marshal(document)
}

func ArchiveDigest(document ArchiveDocument) (string, error) {
	content, err := CanonicalArchiveBytes(document)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

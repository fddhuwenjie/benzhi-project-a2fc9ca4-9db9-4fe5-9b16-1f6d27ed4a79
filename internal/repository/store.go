package repository

import (
	"context"
	"errors"

	"image-integrity-review/internal/domain"
)

var (
	ErrNotFound       = errors.New("案件不存在")
	ErrRevision       = errors.New("revision 冲突")
	ErrManuscriptCode = errors.New("稿件编号已存在")
)

type CommitMeta struct {
	EventType        string
	Actor            domain.Actor
	RequestID        string
	ExpectedRevision int64
	Payload          any
}

type CommitResult struct {
	Case     *domain.ReviewCase
	Event    domain.AuditEvent
	Replayed bool
}

type CaseQuery struct {
	State          domain.CaseState
	JournalSection string
	AssigneeID     string
	Keyword        string
	Severity       domain.Severity
	OpenOnly       bool
	Page           int
	PageSize       int
}

type CaseQueryResult struct {
	Cases   []domain.ReviewCase
	Matches []domain.ReviewCase
}

type Store interface {
	Create(context.Context, *domain.ReviewCase, CommitMeta) (CommitResult, error)
	Update(context.Context, string, CommitMeta, func(*domain.ReviewCase) error) (CommitResult, error)
	Get(context.Context, string) (*domain.ReviewCase, error)
	List(context.Context) ([]domain.ReviewCase, error)
	Query(context.Context, CaseQuery) (CaseQueryResult, error)
	Events(context.Context, string) ([]domain.AuditEvent, error)
	SaveArchive(context.Context, domain.ArchiveDocument) error
	ReadArchive(context.Context, string) (domain.ArchiveDocument, error)
	VerifyArchive(context.Context, string, string) (bool, error)
	RemoveArchive(context.Context, string) error
}

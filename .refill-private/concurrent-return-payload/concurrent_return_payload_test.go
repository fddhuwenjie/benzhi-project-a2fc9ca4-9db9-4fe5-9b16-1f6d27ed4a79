package concurrent_return_payload_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"image-integrity-review/internal/application"
	"image-integrity-review/internal/domain"
	"image-integrity-review/internal/repository"
)

type controlledStore struct {
	firstEntered chan struct{}
	releaseFirst chan struct{}

	mu       sync.Mutex
	payloads map[string]map[string]any
}

func newControlledStore() *controlledStore {
	return &controlledStore{
		firstEntered: make(chan struct{}),
		releaseFirst: make(chan struct{}),
		payloads:     make(map[string]map[string]any),
	}
}

func (s *controlledStore) Update(_ context.Context, caseID string, meta repository.CommitMeta, mutate func(*domain.ReviewCase) error) (repository.CommitResult, error) {
	if caseID == "case-a" {
		close(s.firstEntered)
		<-s.releaseFirst
	}
	item := returnableCase(caseID)
	if err := mutate(item); err != nil {
		return repository.CommitResult{}, err
	}
	payload, ok := meta.Payload.(map[string]any)
	if !ok {
		return repository.CommitResult{}, fmt.Errorf("审计 payload 类型错误：%T", meta.Payload)
	}
	copied := make(map[string]any, len(payload))
	for key, value := range payload {
		copied[key] = value
	}
	s.mu.Lock()
	s.payloads[caseID] = copied
	s.mu.Unlock()
	return repository.CommitResult{Case: item}, nil
}

func returnableCase(caseID string) *domain.ReviewCase {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	return &domain.ReviewCase{
		ID:           caseID,
		State:        domain.StateReadyDecision,
		Revision:     8,
		CurrentRound: 1,
		Findings: []domain.RiskFinding{{
			ID: "finding-" + caseID, Resolution: domain.ResolutionRejected,
			ResponseStatus: domain.ResponseSubmitted, UpdatedAt: now,
		}},
		ResponseRounds: []domain.ResponseRound{{
			Number: 1, Status: "reviewed", StartedAt: now,
			Findings: []domain.RoundFindingEvidence{{FindingID: "finding-" + caseID}},
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (s *controlledStore) payload(caseID string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.payloads[caseID]
}

func (s *controlledStore) Create(context.Context, *domain.ReviewCase, repository.CommitMeta) (repository.CommitResult, error) {
	panic("unexpected Create")
}

func (s *controlledStore) Get(context.Context, string) (*domain.ReviewCase, error) {
	panic("unexpected Get")
}

func (s *controlledStore) List(context.Context) ([]domain.ReviewCase, error) {
	panic("unexpected List")
}

func (s *controlledStore) Query(context.Context, repository.CaseQuery) (repository.CaseQueryResult, error) {
	panic("unexpected Query")
}

func (s *controlledStore) Events(context.Context, string) ([]domain.AuditEvent, error) {
	panic("unexpected Events")
}

func (s *controlledStore) SaveArchive(context.Context, domain.ArchiveDocument) error {
	panic("unexpected SaveArchive")
}

func (s *controlledStore) ReadArchive(context.Context, string) (domain.ArchiveDocument, error) {
	panic("unexpected ReadArchive")
}

func (s *controlledStore) VerifyArchive(context.Context, string, string) (bool, error) {
	panic("unexpected VerifyArchive")
}

func TestConcurrentReturnsKeepAuditPayloadIsolated(t *testing.T) {
	store := newControlledStore()
	service := application.NewService(store)
	service.SetClock(func() time.Time {
		return time.Date(2026, 8, 24, 12, 30, 0, 0, time.UTC)
	})
	actor := domain.Actor{Type: domain.ActorEditor, ID: "editor-1"}

	firstDone := make(chan error, 1)
	go func() {
		_, _, err := service.Decide(context.Background(), "case-a", application.DecisionInput{
			WriteContext: application.WriteContext{Actor: actor, RequestID: "return-a", ExpectedRevision: 8},
			Decision:     "returned",
			Note:         "案件 A 需补充原始数据",
		})
		firstDone <- err
	}()

	<-store.firstEntered
	if _, _, err := service.Decide(context.Background(), "case-b", application.DecisionInput{
		WriteContext: application.WriteContext{Actor: actor, RequestID: "return-b", ExpectedRevision: 8},
		Decision:     "returned",
		Note:         "案件 B 需替换图像",
	}); err != nil {
		t.Fatalf("案件 B 退回失败：%v", err)
	}
	close(store.releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("案件 A 退回失败：%v", err)
	}

	if got := store.payload("case-a")["note"]; got != "案件 A 需补充原始数据" {
		t.Fatalf("案件 A 的审计 note 被并发案件污染：got %q", got)
	}
}

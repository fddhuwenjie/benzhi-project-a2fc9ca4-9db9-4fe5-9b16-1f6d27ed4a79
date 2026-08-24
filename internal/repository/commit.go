package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"image-integrity-review/internal/domain"
)

type eventPayload struct {
	Data      any                `json:"data,omitempty"`
	Aggregate *domain.ReviewCase `json:"aggregate"`
}

func (s *FileStore) Create(_ context.Context, item *domain.ReviewCase, meta CommitMeta) (CommitResult, error) {
	if item == nil {
		return CommitResult{}, errors.New("案件不能为空")
	}
	lock := s.lockFor(item.ID)
	lock.Lock()
	defer lock.Unlock()
	s.mu.RLock()
	if existing, ok := s.requestEvents[meta.RequestID]; ok {
		s.mu.RUnlock()
		return s.replay(existing)
	}
	_, idExists := s.cases[item.ID]
	_, codeExists := s.manuscripts[strings.ToLower(item.ManuscriptCode)]
	s.mu.RUnlock()
	if idExists {
		return CommitResult{}, fmt.Errorf("%w: %s", ErrRevision, item.ID)
	}
	if codeExists {
		return CommitResult{}, ErrManuscriptCode
	}
	if meta.ExpectedRevision != 0 {
		return CommitResult{}, fmt.Errorf("%w: 当前 revision 为 0", ErrRevision)
	}
	copyItem, err := cloneCase(item)
	if err != nil {
		return CommitResult{}, err
	}
	event, err := s.newEvent(copyItem, "", meta)
	if err != nil {
		return CommitResult{}, err
	}
	if err := s.persist(event, copyItem); err != nil {
		return CommitResult{}, err
	}
	s.install(event, copyItem)
	return CommitResult{Case: copyItem, Event: event}, nil
}

func (s *FileStore) Update(_ context.Context, caseID string, meta CommitMeta, mutate func(*domain.ReviewCase) error) (CommitResult, error) {
	lock := s.lockFor(caseID)
	lock.Lock()
	defer lock.Unlock()
	s.mu.RLock()
	if existing, ok := s.requestEvents[meta.RequestID]; ok {
		s.mu.RUnlock()
		return s.replay(existing)
	}
	current, ok := s.cases[caseID]
	if !ok {
		s.mu.RUnlock()
		return CommitResult{}, ErrNotFound
	}
	copyItem, err := cloneCase(current)
	s.mu.RUnlock()
	if err != nil {
		return CommitResult{}, err
	}
	if copyItem.Revision != meta.ExpectedRevision {
		return CommitResult{}, fmt.Errorf("%w: 当前 revision 为 %d", ErrRevision, copyItem.Revision)
	}
	fromState := copyItem.State
	if err := mutate(copyItem); err != nil {
		return CommitResult{}, err
	}
	copyItem.Revision++
	if copyItem.UpdatedAt.IsZero() {
		copyItem.UpdatedAt = time.Now().UTC()
	}
	event, err := s.newEvent(copyItem, fromState, meta)
	if err != nil {
		return CommitResult{}, err
	}
	if err := s.persist(event, copyItem); err != nil {
		return CommitResult{}, err
	}
	s.install(event, copyItem)
	return CommitResult{Case: copyItem, Event: event}, nil
}

func (s *FileStore) newEvent(item *domain.ReviewCase, from domain.CaseState, meta CommitMeta) (domain.AuditEvent, error) {
	payload, err := json.Marshal(eventPayload{Data: meta.Payload, Aggregate: item})
	if err != nil {
		return domain.AuditEvent{}, err
	}
	occurred := item.UpdatedAt
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}
	return domain.AuditEvent{
		EventID:          fmt.Sprintf("%s-%d-%d", item.ID, item.Revision, occurred.UnixNano()),
		CaseID:           item.ID,
		EventType:        meta.EventType,
		ActorType:        meta.Actor.Type,
		ActorID:          meta.Actor.ID,
		RequestID:        meta.RequestID,
		ExpectedRevision: meta.ExpectedRevision,
		ResultRevision:   item.Revision,
		FromState:        from,
		ToState:          item.State,
		Payload:          payload,
		OccurredAt:       occurred.UTC(),
	}, nil
}

func (s *FileStore) replay(event domain.AuditEvent) (CommitResult, error) {
	var payload eventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.Aggregate == nil {
		return CommitResult{}, fmt.Errorf("读取幂等结果: %w", err)
	}
	item, err := cloneCase(payload.Aggregate)
	if err != nil {
		return CommitResult{}, err
	}
	return CommitResult{Case: item, Event: event, Replayed: true}, nil
}

func (s *FileStore) install(event domain.AuditEvent, item *domain.ReviewCase) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cases[item.ID] = item
	s.manuscripts[strings.ToLower(item.ManuscriptCode)] = item.ID
	s.events[item.ID] = append(s.events[item.ID], event)
	s.requestEvents[event.RequestID] = event
}

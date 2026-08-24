package application

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"image-integrity-review/internal/domain"
	"image-integrity-review/internal/repository"
)

type Service struct {
	store                 repository.Store
	now                   func() time.Time
	returnDecisionPayload map[string]any
}

func NewService(store repository.Store) *Service {
	return &Service{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) SetClock(clock func() time.Time) {
	if clock != nil {
		s.now = clock
	}
}

func (s *Service) prepareReturnDecisionPayload(note string) map[string]any {
	if s.returnDecisionPayload == nil {
		s.returnDecisionPayload = make(map[string]any, 4)
	}
	for key := range s.returnDecisionPayload {
		delete(s.returnDecisionPayload, key)
	}
	s.returnDecisionPayload["note"] = note
	return s.returnDecisionPayload
}

func randomToken(bytes int) (string, error) {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("生成安全随机值: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func caseIDForRequest(requestID string) string {
	return "case-" + domain.HashAccessToken("case:" + requestID)[:20]
}

func validateWrite(input WriteContext) error {
	if err := domain.ValidateRequest(input.RequestID, input.ExpectedRevision); err != nil {
		return err
	}
	if input.Actor.ID == "" {
		return domain.ValidationError(domain.FieldError{Field: "actor_id", Message: "操作者标识不能为空"})
	}
	return nil
}

func requireRole(actor domain.Actor, roles ...domain.ActorType) error {
	for _, role := range roles {
		if actor.Type == role {
			return nil
		}
	}
	return domain.NewError(domain.CodeForbidden, "当前角色无权执行该操作")
}

func commitMeta(eventType string, input WriteContext, payload any) repository.CommitMeta {
	return repository.CommitMeta{
		EventType:        eventType,
		Actor:            input.Actor,
		RequestID:        input.RequestID,
		ExpectedRevision: input.ExpectedRevision,
		Payload:          payload,
	}
}

package application

import (
	"context"
	"crypto/subtle"
	"encoding/json"

	"image-integrity-review/internal/domain"
)

func (s *Service) ClaimCase(ctx context.Context, caseID string, input WriteContext) (*domain.ReviewCase, bool, error) {
	if err := validateWrite(input); err != nil {
		return nil, false, err
	}
	if err := requireRole(input.Actor, domain.ActorReviewer); err != nil {
		return nil, false, err
	}
	result, err := s.store.Update(ctx, caseID, commitMeta("case_claimed", input, nil), func(item *domain.ReviewCase) error {
		return item.Claim(input.Actor.ID, s.now())
	})
	if err != nil {
		return nil, false, err
	}
	return result.Case, result.Replayed, nil
}

func (s *Service) RecordVerdict(ctx context.Context, caseID string, input VerdictInput) (*domain.ReviewCase, bool, error) {
	if err := validateWrite(input.WriteContext); err != nil {
		return nil, false, err
	}
	if err := requireRole(input.Actor, domain.ActorReviewer); err != nil {
		return nil, false, err
	}
	result, err := s.store.Update(ctx, caseID, commitMeta("finding_reviewed", input.WriteContext, map[string]any{"finding_id": input.FindingID, "verdict": input.Verdict}), func(item *domain.ReviewCase) error {
		if item.AssigneeID != input.Actor.ID {
			return domain.NewError(domain.CodeForbidden, "仅案件领取人可记录判读")
		}
		return item.RecordVerdict(input.FindingID, input.Verdict, input.Note, s.now())
	})
	if err != nil {
		return nil, false, err
	}
	return result.Case, result.Replayed, nil
}

func (s *Service) RecordVerdicts(ctx context.Context, caseID string, input BatchVerdictInput) (*domain.ReviewCase, bool, error) {
	if err := validateWrite(input.WriteContext); err != nil {
		return nil, false, err
	}
	if err := requireRole(input.Actor, domain.ActorReviewer); err != nil {
		return nil, false, err
	}
	payloadItems := make([]map[string]any, len(input.Items))
	for i, item := range input.Items {
		payloadItems[i] = map[string]any{"finding_id": item.FindingID, "verdict": item.Verdict}
	}
	result, err := s.store.Update(ctx, caseID, commitMeta("findings_batch_reviewed", input.WriteContext, map[string]any{"items": payloadItems}), func(item *domain.ReviewCase) error {
		if item.AssigneeID != input.Actor.ID {
			return domain.NewError(domain.CodeForbidden, "仅案件领取人可记录判读")
		}
		return item.RecordVerdicts(input.Items, s.now())
	})
	if err != nil {
		return nil, false, err
	}
	return result.Case, result.Replayed, nil
}

func (s *Service) BeginAuthorResponse(ctx context.Context, caseID string, input WriteContext) (AccessCredential, error) {
	if err := validateWrite(input); err != nil {
		return AccessCredential{}, err
	}
	if err := requireRole(input.Actor, domain.ActorReviewer); err != nil {
		return AccessCredential{}, err
	}
	token, err := randomToken(24)
	if err != nil {
		return AccessCredential{}, err
	}
	payload := map[string]any{"author_access_token": token}
	result, err := s.store.Update(ctx, caseID, commitMeta("author_response_requested", input, payload), func(item *domain.ReviewCase) error {
		if item.AssigneeID != input.Actor.ID {
			return domain.NewError(domain.CodeForbidden, "仅案件领取人可发起作者回应")
		}
		err := item.BeginAuthorResponse(domain.HashAccessToken(token), s.now())
		if err == nil {
			payload["round_number"] = item.CurrentRound
		}
		return err
	})
	if err != nil {
		return AccessCredential{}, err
	}
	if result.Replayed {
		if replayed := tokenFromEvent(result.Event.Payload); replayed != "" {
			token = replayed
		}
	}
	return AccessCredential{Case: result.Case, AccessToken: token, Replayed: result.Replayed}, nil
}

func tokenFromEvent(payload json.RawMessage) string {
	var outer struct {
		Data struct {
			Token string `json:"author_access_token"`
		} `json:"data"`
	}
	if json.Unmarshal(payload, &outer) != nil {
		return ""
	}
	return outer.Data.Token
}

func authorizeAuthor(item *domain.ReviewCase, accessToken string) error {
	want := []byte(item.AuthorAccessHash)
	got := []byte(domain.HashAccessToken(accessToken))
	if len(want) != len(got) || subtle.ConstantTimeCompare(want, got) != 1 {
		return domain.NewError(domain.CodeForbidden, "作者访问凭据无效")
	}
	return nil
}

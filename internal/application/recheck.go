package application

import (
	"context"

	"image-integrity-review/internal/domain"
)

func (s *Service) ResolveFinding(ctx context.Context, caseID string, input ResolutionInput) (*domain.ReviewCase, bool, error) {
	if err := validateWrite(input.WriteContext); err != nil {
		return nil, false, err
	}
	if err := requireRole(input.Actor, domain.ActorReviewer); err != nil {
		return nil, false, err
	}
	result, err := s.store.Update(ctx, caseID, commitMeta("author_response_reviewed", input.WriteContext, map[string]any{"finding_id": input.FindingID, "resolution": input.Resolution, "round_number": input.RoundNumber}), func(item *domain.ReviewCase) error {
		if item.AssigneeID != input.Actor.ID {
			return domain.NewError(domain.CodeForbidden, "仅案件领取人可复核回应")
		}
		return item.ResolveFindingForRound(input.RoundNumber, input.FindingID, input.Resolution, input.Note, s.now())
	})
	if err != nil {
		return nil, false, err
	}
	return result.Case, result.Replayed, nil
}

func (s *Service) FinishRecheck(ctx context.Context, caseID string, input WriteContext) (*domain.ReviewCase, bool, error) {
	if err := validateWrite(input); err != nil {
		return nil, false, err
	}
	if err := requireRole(input.Actor, domain.ActorReviewer); err != nil {
		return nil, false, err
	}
	result, err := s.store.Update(ctx, caseID, commitMeta("recheck_completed", input, nil), func(item *domain.ReviewCase) error {
		if item.AssigneeID != input.Actor.ID {
			return domain.NewError(domain.CodeForbidden, "仅案件领取人可完成复核")
		}
		return item.FinishRecheck(s.now())
	})
	if err != nil {
		return nil, false, err
	}
	return result.Case, result.Replayed, nil
}

package application

import (
	"context"

	"image-integrity-review/internal/domain"
)

func (s *Service) GetAuthorCase(ctx context.Context, caseID, accessToken string) (*domain.ReviewCase, error) {
	item, err := s.store.Get(ctx, caseID)
	if err != nil {
		return nil, err
	}
	if err := authorizeAuthor(item, accessToken); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) SubmitAuthorResponse(ctx context.Context, caseID string, input AuthorResponseInput) (*domain.ReviewCase, bool, error) {
	if err := validateWrite(input.WriteContext); err != nil {
		return nil, false, err
	}
	if err := requireRole(input.Actor, domain.ActorAuthor); err != nil {
		return nil, false, err
	}
	result, err := s.store.Update(ctx, caseID, commitMeta("author_response_submitted", input.WriteContext, map[string]any{"finding_id": input.FindingID, "round_number": input.RoundNumber}), func(item *domain.ReviewCase) error {
		if err := authorizeAuthor(item, input.AccessToken); err != nil {
			return err
		}
		return item.SubmitAuthorResponseForRound(input.RoundNumber, input.FindingID, input.Explanation, input.ReplacementDigest, input.RawDataReference, s.now())
	})
	if err != nil {
		return nil, false, err
	}
	return result.Case, result.Replayed, nil
}

func (s *Service) FinishAuthorResponseRound(ctx context.Context, caseID, accessToken string, roundNumber int, input WriteContext) (*domain.ReviewCase, bool, error) {
	if err := validateWrite(input); err != nil {
		return nil, false, err
	}
	if err := requireRole(input.Actor, domain.ActorAuthor); err != nil {
		return nil, false, err
	}
	result, err := s.store.Update(ctx, caseID, commitMeta("author_response_completed", input, map[string]any{"round_number": roundNumber}), func(item *domain.ReviewCase) error {
		if err := authorizeAuthor(item, accessToken); err != nil {
			return err
		}
		return item.FinishAuthorResponseForRound(roundNumber, s.now())
	})
	if err != nil {
		return nil, false, err
	}
	return result.Case, result.Replayed, nil
}

func (s *Service) FinishAuthorResponse(ctx context.Context, caseID, accessToken string, input WriteContext) (*domain.ReviewCase, bool, error) {
	if err := validateWrite(input); err != nil {
		return nil, false, err
	}
	if err := requireRole(input.Actor, domain.ActorAuthor); err != nil {
		return nil, false, err
	}
	result, err := s.store.Update(ctx, caseID, commitMeta("author_response_completed", input, nil), func(item *domain.ReviewCase) error {
		if err := authorizeAuthor(item, accessToken); err != nil {
			return err
		}
		return item.FinishAuthorResponse(s.now())
	})
	if err != nil {
		return nil, false, err
	}
	return result.Case, result.Replayed, nil
}

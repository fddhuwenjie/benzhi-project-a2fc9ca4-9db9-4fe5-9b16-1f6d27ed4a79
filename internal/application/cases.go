package application

import (
	"context"
	"errors"

	"image-integrity-review/internal/domain"
	"image-integrity-review/internal/repository"
)

func (s *Service) CreateCase(ctx context.Context, input CreateCaseInput) (*domain.ReviewCase, bool, error) {
	if err := validateWrite(input.WriteContext); err != nil {
		return nil, false, err
	}
	if err := requireRole(input.Actor, domain.ActorEditor); err != nil {
		return nil, false, err
	}
	item, err := domain.NewReviewCase(caseIDForRequest(input.RequestID), input.ManuscriptCode, input.Title, input.JournalSection, input.Figures, s.now())
	if err != nil {
		return nil, false, err
	}
	result, err := s.store.Create(ctx, item, commitMeta("case_created", input.WriteContext, map[string]any{"figure_count": len(input.Figures)}))
	if errors.Is(err, repository.ErrManuscriptCode) {
		return nil, false, domain.NewError(domain.CodeConflict, "稿件编号已存在")
	}
	if err != nil {
		return nil, false, err
	}
	return result.Case, result.Replayed, nil
}

func (s *Service) GetCase(ctx context.Context, caseID string) (*domain.ReviewCase, error) {
	return s.store.Get(ctx, caseID)
}

func (s *Service) ListCases(ctx context.Context) ([]domain.ReviewCase, error) {
	return s.store.List(ctx)
}

func (s *Service) Timeline(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	return s.store.Events(ctx, caseID)
}

func (s *Service) ReviseDraftFigures(ctx context.Context, caseID string, input ReviseFiguresInput) (*domain.ReviewCase, []domain.RiskFinding, bool, error) {
	if err := validateWrite(input.WriteContext); err != nil {
		return nil, nil, false, err
	}
	if err := requireRole(input.Actor, domain.ActorEditor); err != nil {
		return nil, nil, false, err
	}
	payload := map[string]any{}
	result, err := s.store.Update(ctx, caseID, commitMeta("draft_figures_revised", input.WriteContext, payload), func(item *domain.ReviewCase) error {
		changes, err := item.ReviseDraftFigures(input.Figures, s.now())
		if err == nil {
			payload["added_figure_ids"] = changes.Added
			payload["modified_figure_ids"] = changes.Modified
			payload["deleted_figure_ids"] = changes.Deleted
		}
		return err
	})
	if err != nil {
		return nil, nil, false, err
	}
	preflight, err := result.Case.DraftPreflight(s.now())
	if err != nil {
		return nil, nil, false, err
	}
	return result.Case, preflight, result.Replayed, nil
}

func (s *Service) DraftPreflight(ctx context.Context, caseID string) ([]domain.RiskFinding, error) {
	item, err := s.store.Get(ctx, caseID)
	if err != nil {
		return nil, err
	}
	return item.DraftPreflight(s.now())
}

func (s *Service) SubmitDraft(ctx context.Context, caseID string, input WriteContext) (*domain.ReviewCase, bool, error) {
	if err := validateWrite(input); err != nil {
		return nil, false, err
	}
	if err := requireRole(input.Actor, domain.ActorEditor); err != nil {
		return nil, false, err
	}
	result, err := s.store.Update(ctx, caseID, commitMeta("integrity_checks_completed", input, nil), func(item *domain.ReviewCase) error {
		return item.SubmitForChecks(s.now())
	})
	if err != nil {
		return nil, false, err
	}
	return result.Case, result.Replayed, nil
}

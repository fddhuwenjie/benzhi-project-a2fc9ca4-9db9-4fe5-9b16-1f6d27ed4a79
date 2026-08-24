package application

import (
	"context"
	"strings"

	"image-integrity-review/internal/domain"
)

func (s *Service) Decide(ctx context.Context, caseID string, input DecisionInput) (*domain.ReviewCase, bool, error) {
	if err := validateWrite(input.WriteContext); err != nil {
		return nil, false, err
	}
	if err := requireRole(input.Actor, domain.ActorEditor); err != nil {
		return nil, false, err
	}
	if input.Decision == "returned" {
		payload := map[string]any{"note": input.Note}
		result, err := s.store.Update(ctx, caseID, commitMeta("case_returned", input.WriteContext, payload), func(item *domain.ReviewCase) error {
			closedRound := item.CurrentRound
			err := item.ReturnForSupplement(input.Note, s.now())
			if err == nil {
				payload["closed_round"] = closedRound
				payload["next_round"] = item.CurrentRound
				round := item.ResponseRounds[len(item.ResponseRounds)-1]
				ids := make([]string, 0, len(round.Findings))
				for _, finding := range round.Findings {
					ids = append(ids, finding.FindingID)
				}
				payload["reopened_finding_ids"] = ids
			}
			return err
		})
		if err != nil {
			return nil, false, err
		}
		return result.Case, result.Replayed, nil
	}
	if input.Decision != "approved" {
		return nil, false, domain.ValidationError(domain.FieldError{Field: "decision", Message: "终审决定必须为 approved 或 returned"})
	}
	approved, err := s.store.Update(ctx, caseID, commitMeta("case_approved", input.WriteContext, map[string]any{"note": input.Note}), func(item *domain.ReviewCase) error {
		return item.Archive(input.Note, s.now())
	})
	if err != nil {
		return nil, false, err
	}
	events, err := s.store.Events(ctx, caseID)
	if err != nil {
		return nil, false, err
	}
	document := buildArchive(approved.Case, events)
	digest, err := domain.ArchiveDigest(document)
	if err != nil {
		return nil, false, err
	}
	if err := s.store.SaveArchive(ctx, document); err != nil {
		return nil, false, err
	}
	digestInput := input.WriteContext
	digestInput.RequestID += ":archive-digest"
	digestInput.ExpectedRevision = approved.Case.Revision
	attached, err := s.store.Update(ctx, caseID, commitMeta("archive_digest_recorded", digestInput, map[string]any{"digest": digest}), func(item *domain.ReviewCase) error {
		item.ArchiveDigest = digest
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return attached.Case, approved.Replayed && attached.Replayed, nil
}

func buildArchive(item *domain.ReviewCase, timeline []domain.AuditEvent) domain.ArchiveDocument {
	stableTimeline := make([]domain.AuditEvent, 0, len(timeline))
	for _, event := range timeline {
		if event.EventType != "archive_digest_recorded" {
			stableTimeline = append(stableTimeline, event)
		}
	}
	archivedAt := item.UpdatedAt
	if item.ArchivedAt != nil {
		archivedAt = *item.ArchivedAt
	}
	return domain.ArchiveDocument{
		SchemaVersion:  "image-integrity-archive-v2",
		CaseID:         item.ID,
		ManuscriptCode: item.ManuscriptCode,
		Title:          item.Title,
		JournalSection: item.JournalSection,
		RuleVersion:    item.RuleVersion,
		Figures:        item.Figures,
		Findings:       item.Findings,
		ResponseRounds: item.ResponseRounds,
		FinalDecision:  item.FinalDecision,
		DecisionNote:   item.DecisionNote,
		CreatedAt:      item.CreatedAt,
		ArchivedAt:     archivedAt,
		Timeline:       stableTimeline,
	}
}

func (s *Service) DownloadArchive(ctx context.Context, caseID string) (domain.ArchiveDocument, error) {
	return s.store.ReadArchive(ctx, caseID)
}

func (s *Service) VerifyArchive(ctx context.Context, caseReference, digest string) (bool, error) {
	caseID := caseReference
	if !strings.HasPrefix(caseReference, "case-") {
		items, err := s.store.List(ctx)
		if err != nil {
			return false, err
		}
		caseID = ""
		for i := range items {
			if strings.EqualFold(items[i].ManuscriptCode, caseReference) {
				caseID = items[i].ID
				break
			}
		}
		if caseID == "" {
			return false, domain.NewError(domain.CodeNotFound, "案件不存在")
		}
	}
	return s.store.VerifyArchive(ctx, caseID, digest)
}

package event_payload_alias_test

import (
	"context"
	"testing"
	"time"

	"image-integrity-review/internal/domain"
	"image-integrity-review/internal/repository"
)

func TestReturnedEventPayloadsCannotCorruptReplay(t *testing.T) {
	newFixture := func(id string) (*repository.FileStore, *domain.ReviewCase, repository.CommitMeta, repository.CommitResult) {
		store, err := repository.OpenFileStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		item, err := domain.NewReviewCase(id, "EVENT-"+id, "事件载荷", "研究论文", []domain.FigureRecord{{
			FigureLabel: "Figure 1", Caption: "图注", ContentDigest: "aaaaaaaaaaaaaaaa", PixelWidth: 10, PixelHeight: 10,
		}}, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		meta := repository.CommitMeta{
			EventType: "case_created", RequestID: "request-" + id, Actor: domain.Actor{Type: domain.ActorEditor, ID: "editor"},
		}
		result, err := store.Create(context.Background(), item, meta)
		if err != nil {
			t.Fatal(err)
		}
		return store, item, meta, result
	}

	store, item, meta, result := newFixture("case-result-event")
	for i := range result.Event.Payload {
		result.Event.Payload[i] = 'x'
	}
	if _, err := store.Create(context.Background(), item, meta); err != nil {
		t.Errorf("修改 CommitResult.Event 破坏了幂等重放：%v", err)
	}

	store, item, meta, _ = newFixture("case-timeline-event")
	events, err := store.Events(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := range events[0].Payload {
		events[0].Payload[i] = 'x'
	}
	replayed, err := store.Create(context.Background(), item, meta)
	if err != nil {
		t.Errorf("修改 Timeline 返回值破坏了幂等重放：%v", err)
		return
	}
	if !replayed.Replayed || replayed.Case.ID != item.ID {
		t.Fatalf("幂等重放结果无效：%+v", replayed)
	}
}

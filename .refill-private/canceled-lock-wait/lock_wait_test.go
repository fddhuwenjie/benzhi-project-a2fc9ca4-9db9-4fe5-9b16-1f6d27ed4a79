package canceledlockwait_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"image-integrity-review/internal/domain"
	"image-integrity-review/internal/repository"
)

type observedContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (c *observedContext) Err() error {
	c.once.Do(func() { close(c.checked) })
	return c.Context.Err()
}

func TestCanceledWaiterMustNotCommitAfterCaseLockRelease(t *testing.T) {
	store, err := repository.OpenFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	item, err := domain.NewReviewCase(
		"case-lock-cancel",
		"LOCK-CANCEL-001",
		"取消等待测试",
		"研究论文",
		[]domain.FigureRecord{{
			FigureLabel:   "Figure 1",
			Caption:       "原始图注",
			ContentDigest: "aaaaaaaaaaaaaaaa",
			PixelWidth:    100,
			PixelHeight:   100,
		}},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Create(context.Background(), item, repository.CommitMeta{
		EventType:        "case_created",
		Actor:            domain.Actor{Type: domain.ActorEditor, ID: "editor-1"},
		RequestID:        "create-lock-cancel",
		ExpectedRevision: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstResult := make(chan error, 1)
	go func() {
		_, updateErr := store.Update(context.Background(), item.ID, repository.CommitMeta{
			EventType:        "title_changed",
			Actor:            domain.Actor{Type: domain.ActorEditor, ID: "editor-1"},
			RequestID:        "first-title-change",
			ExpectedRevision: created.Case.Revision,
		}, func(value *domain.ReviewCase) error {
			close(firstEntered)
			<-releaseFirst
			value.Title = "首个更新"
			value.UpdatedAt = now.Add(time.Minute)
			return nil
		})
		firstResult <- updateErr
	}()
	<-firstEntered

	base, cancel := context.WithCancel(context.Background())
	waitingContext := &observedContext{Context: base, checked: make(chan struct{})}
	secondResult := make(chan error, 1)
	go func() {
		_, updateErr := store.Update(waitingContext, item.ID, repository.CommitMeta{
			EventType:        "title_changed",
			Actor:            domain.Actor{Type: domain.ActorEditor, ID: "editor-2"},
			RequestID:        "canceled-title-change",
			ExpectedRevision: created.Case.Revision + 1,
		}, func(value *domain.ReviewCase) error {
			value.Title = "已取消更新"
			value.UpdatedAt = now.Add(2 * time.Minute)
			return nil
		})
		secondResult <- updateErr
	}()
	<-waitingContext.checked
	cancel()
	close(releaseFirst)

	if err := <-firstResult; err != nil {
		t.Fatalf("首个更新失败：%v", err)
	}
	secondErr := <-secondResult
	stored, err := store.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(secondErr, context.Canceled) || stored.Title != "首个更新" || stored.Revision != created.Case.Revision+1 {
		t.Fatalf("等待案件锁时取消的更新仍被提交：error=%v title=%q revision=%d", secondErr, stored.Title, stored.Revision)
	}
}

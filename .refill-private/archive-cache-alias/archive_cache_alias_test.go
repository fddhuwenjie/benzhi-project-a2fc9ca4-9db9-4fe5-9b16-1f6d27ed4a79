package archive_cache_alias_test

import (
	"context"
	"testing"
	"time"

	"image-integrity-review/internal/domain"
	"image-integrity-review/internal/repository"
)

func TestArchiveCacheMustIsolateReturnedDocuments(t *testing.T) {
	store, err := repository.OpenFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("打开仓储: %v", err)
	}
	document := domain.ArchiveDocument{
		SchemaVersion:  "image-integrity-archive-v2",
		CaseID:         "case-cache-isolation",
		ManuscriptCode: "MS-CACHE-001",
		Title:          "缓存所有权测试",
		JournalSection: "研究论文",
		RuleVersion:    "integrity-rules-v1",
		Figures: []domain.FigureRecord{{
			ID: "figure-1", CaseID: "case-cache-isolation", FigureLabel: "Figure 1",
			Caption: "磁盘中的原始图注", ContentDigest: "sha256:original",
		}},
		CreatedAt:  time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC),
		ArchivedAt: time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
	}
	if err := store.SaveArchive(context.Background(), document); err != nil {
		t.Fatalf("保存归档: %v", err)
	}

	first, err := store.ReadArchive(context.Background(), document.CaseID)
	if err != nil {
		t.Fatalf("首次读取归档: %v", err)
	}
	first.Figures[0].Caption = "调用方篡改的图注"

	second, err := store.ReadArchive(context.Background(), document.CaseID)
	if err != nil {
		t.Fatalf("再次读取归档: %v", err)
	}
	if got, want := second.Figures[0].Caption, "磁盘中的原始图注"; got != want {
		t.Fatalf("归档缓存泄漏了调用方修改: got %q, want %q", got, want)
	}
}

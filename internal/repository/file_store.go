package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"image-integrity-review/internal/domain"
)

type FileStore struct {
	root          string
	snapshotDir   string
	archiveDir    string
	eventsPath    string
	mu            sync.RWMutex
	appendMu      sync.Mutex
	cases         map[string]*domain.ReviewCase
	manuscripts   map[string]string
	caseLocks     map[string]*sync.Mutex
	events        map[string][]domain.AuditEvent
	requestEvents map[string]map[string]domain.AuditEvent
	archiveCache  map[string]domain.ArchiveDocument
}

func OpenFileStore(root string) (*FileStore, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("持久化目录不能为空")
	}
	store := &FileStore{
		root:          root,
		snapshotDir:   filepath.Join(root, "snapshots"),
		archiveDir:    filepath.Join(root, "archives"),
		eventsPath:    filepath.Join(root, "events.jsonl"),
		cases:         map[string]*domain.ReviewCase{},
		manuscripts:   map[string]string{},
		caseLocks:     map[string]*sync.Mutex{},
		events:        map[string][]domain.AuditEvent{},
		requestEvents: map[string]map[string]domain.AuditEvent{},
		archiveCache:  map[string]domain.ArchiveDocument{},
	}
	for _, dir := range []string{root, store.snapshotDir, store.archiveDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("创建持久化目录: %w", err)
		}
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileStore) Get(_ context.Context, caseID string) (*domain.ReviewCase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.cases[caseID]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneCase(item)
}

func (s *FileStore) List(ctx context.Context) ([]domain.ReviewCase, error) {
	result, err := s.Query(ctx, CaseQuery{Page: 1, PageSize: int(^uint(0) >> 1)})
	return result.Cases, err
}

func (s *FileStore) Query(_ context.Context, query CaseQuery) (CaseQueryResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.ReviewCase, 0, len(s.cases))
	for _, item := range s.cases {
		if !domain.CaseMatches(item, query.State, query.JournalSection, query.AssigneeID, query.Keyword, query.Severity, query.OpenOnly) {
			continue
		}
		copyItem, err := cloneCase(item)
		if err != nil {
			return CaseQueryResult{}, err
		}
		result = append(result, *copyItem)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].UpdatedAt.Equal(result[j].UpdatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	matches := result
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(result) {
		return CaseQueryResult{Cases: []domain.ReviewCase{}, Matches: matches}, nil
	}
	end := start + pageSize
	if end > len(result) {
		end = len(result)
	}
	return CaseQueryResult{Cases: result[start:end], Matches: matches}, nil
}

func (s *FileStore) Events(_ context.Context, caseID string) ([]domain.AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.cases[caseID]; !ok {
		return nil, ErrNotFound
	}
	items := s.events[caseID]
	result := make([]domain.AuditEvent, len(items))
	copy(result, items)
	return result, nil
}

func (s *FileStore) lockFor(caseID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, ok := s.caseLocks[caseID]
	if !ok {
		lock = &sync.Mutex{}
		s.caseLocks[caseID] = lock
	}
	return lock
}

func cloneCase(item *domain.ReviewCase) (*domain.ReviewCase, error) {
	content, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	var result domain.ReviewCase
	if err := json.Unmarshal(content, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

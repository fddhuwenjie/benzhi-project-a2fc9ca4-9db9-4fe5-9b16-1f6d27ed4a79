package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"image-integrity-review/internal/domain"
)

func (s *FileStore) SaveArchive(_ context.Context, document domain.ArchiveDocument) error {
	digest, err := domain.ArchiveDigest(document)
	if err != nil {
		return err
	}
	document.Digest = digest
	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(s.archiveDir, document.CaseID+".json"), content, 0o440); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.archiveCache, document.CaseID)
	s.mu.Unlock()
	return nil
}

func (s *FileStore) ReadArchive(_ context.Context, caseID string) (domain.ArchiveDocument, error) {
	s.mu.RLock()
	cached, ok := s.archiveCache[caseID]
	s.mu.RUnlock()
	if ok {
		return cloneArchiveDocument(cached)
	}
	content, err := os.ReadFile(filepath.Join(s.archiveDir, caseID+".json"))
	if os.IsNotExist(err) {
		return domain.ArchiveDocument{}, ErrNotFound
	}
	if err != nil {
		return domain.ArchiveDocument{}, err
	}
	var document domain.ArchiveDocument
	if err := json.Unmarshal(content, &document); err != nil {
		return domain.ArchiveDocument{}, err
	}
	actual, err := domain.ArchiveDigest(document)
	if err != nil {
		return domain.ArchiveDocument{}, err
	}
	if !strings.EqualFold(actual, document.Digest) {
		return domain.ArchiveDocument{}, fmt.Errorf("归档摘要不匹配")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.archiveCache[caseID]; ok {
		return cloneArchiveDocument(cached)
	}
	stored, err := cloneArchiveDocument(document)
	if err != nil {
		return domain.ArchiveDocument{}, err
	}
	s.archiveCache[caseID] = stored
	return document, nil
}

func cloneArchiveDocument(document domain.ArchiveDocument) (domain.ArchiveDocument, error) {
	content, err := json.Marshal(document)
	if err != nil {
		return domain.ArchiveDocument{}, err
	}
	var result domain.ArchiveDocument
	if err := json.Unmarshal(content, &result); err != nil {
		return domain.ArchiveDocument{}, err
	}
	return result, nil
}

func (s *FileStore) VerifyArchive(ctx context.Context, caseID, digest string) (bool, error) {
	document, err := s.ReadArchive(ctx, caseID)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(document.Digest, strings.TrimSpace(digest)), nil
}

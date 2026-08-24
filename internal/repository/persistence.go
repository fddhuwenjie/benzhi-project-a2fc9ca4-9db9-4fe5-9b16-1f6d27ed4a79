package repository

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"image-integrity-review/internal/domain"
)

type snapshotEnvelope struct {
	Digest string            `json:"digest"`
	Case   domain.ReviewCase `json:"case"`
}

func (s *FileStore) persist(event domain.AuditEvent, item *domain.ReviewCase) error {
	s.appendMu.Lock()
	defer s.appendMu.Unlock()
	if err := appendJSONLine(s.eventsPath, event); err != nil {
		return fmt.Errorf("追加审计事件: %w", err)
	}
	if err := s.writeSnapshot(item); err != nil {
		return fmt.Errorf("更新案件快照: %w", err)
	}
	return nil
}

func appendJSONLine(path string, value any) error {
	content, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(content, '\n')); err != nil {
		return err
	}
	return file.Sync()
}

func (s *FileStore) writeSnapshot(item *domain.ReviewCase) error {
	caseBytes, err := json.Marshal(item)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(caseBytes)
	envelope := snapshotEnvelope{Digest: hex.EncodeToString(sum[:]), Case: *item}
	content, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(s.snapshotDir, item.ID+".json"), content, 0o640)
}

func atomicWrite(path string, content []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".pending-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tempName)
		}
	}()
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return err
	}
	ok = true
	return nil
}

func (s *FileStore) load() error {
	if err := s.loadSnapshots(); err != nil {
		return err
	}
	if err := s.loadEvents(); err != nil {
		return err
	}
	return nil
}

func (s *FileStore) loadSnapshots() error {
	entries, err := os.ReadDir(s.snapshotDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(s.snapshotDir, entry.Name()))
		if err != nil {
			return err
		}
		var envelope snapshotEnvelope
		if err := json.Unmarshal(content, &envelope); err != nil {
			return fmt.Errorf("解析快照 %s: %w", entry.Name(), err)
		}
		caseBytes, err := json.Marshal(envelope.Case)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(caseBytes)
		if !strings.EqualFold(envelope.Digest, hex.EncodeToString(sum[:])) {
			return fmt.Errorf("快照 %s 摘要不匹配", entry.Name())
		}
		item, err := cloneCase(&envelope.Case)
		if err != nil {
			return err
		}
		s.cases[item.ID] = item
		s.manuscripts[strings.ToLower(item.ManuscriptCode)] = item.ID
		s.caseLocks[item.ID] = &sync.Mutex{}
	}
	return nil
}

func (s *FileStore) loadEvents() error {
	file, err := os.Open(s.eventsPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	lineNumber := 0
	for {
		line, readErr := reader.ReadBytes('\n')
		lineNumber++
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			var event domain.AuditEvent
			if err := json.Unmarshal(trimmed, &event); err != nil {
				if errors.Is(readErr, io.EOF) {
					break
				}
				return fmt.Errorf("事件日志第 %d 行损坏: %w", lineNumber, err)
			}
			s.acceptLoadedEvent(event)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

func (s *FileStore) acceptLoadedEvent(event domain.AuditEvent) {
	s.events[event.CaseID] = append(s.events[event.CaseID], event)
	if s.requestEvents[event.CaseID] == nil {
		s.requestEvents[event.CaseID] = map[string]domain.AuditEvent{}
	}
	s.requestEvents[event.CaseID][event.RequestID] = event
	var payload eventPayload
	if json.Unmarshal(event.Payload, &payload) != nil || payload.Aggregate == nil {
		return
	}
	current, exists := s.cases[event.CaseID]
	if !exists || payload.Aggregate.Revision > current.Revision {
		item, err := cloneCase(payload.Aggregate)
		if err == nil {
			s.cases[item.ID] = item
			s.manuscripts[strings.ToLower(item.ManuscriptCode)] = item.ID
			if _, ok := s.caseLocks[item.ID]; !ok {
				s.caseLocks[item.ID] = &sync.Mutex{}
			}
		}
	}
}

package configuration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"

	core "shuttle/internal/core/configuration"
)

type backup struct {
	Target   string
	Original string
	Applied  string
	Absent   bool
}

type operationFile struct {
	Operation core.Operation
	Backups   []backup
}

// FileOperationStore uses an explicit root so tests never access user data.
// One owner must hold the application operation lock while using this store.
type FileOperationStore struct {
	Root string
	mu   sync.Mutex
}

func NewFileOperationStore(root string) *FileOperationStore { return &FileOperationStore{Root: root} }

var operationID = regexp.MustCompile(`^(op|restore)-[a-zA-Z0-9-]+$`)

func (s *FileOperationStore) path(id string) (string, error) {
	if !operationID.MatchString(id) {
		return "", fmt.Errorf("invalid operation ID")
	}
	return filepath.Join(s.Root, id, "record.json"), nil
}

func (s *FileOperationStore) load(id string) (operationFile, error) {
	path, err := s.path(id)
	if err != nil {
		return operationFile{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return operationFile{}, err
	}
	var record operationFile
	if err := json.Unmarshal(raw, &record); err != nil {
		return record, fmt.Errorf("operation %s: %w", id, err)
	}
	if record.Operation.ID != id || record.Operation.SchemaVersion != 1 {
		return record, fmt.Errorf("operation %s has invalid identity or schema", id)
	}
	return record, nil
}

func (s *FileOperationStore) save(record operationFile) error {
	path, err := s.path(record.Operation.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".record-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func (s *FileOperationStore) Begin(op core.Operation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.path(op.ID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("operation already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	return s.save(operationFile{Operation: op})
}

func (s *FileOperationStore) Save(id, target, o, a string, absent bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.load(id)
	if err != nil {
		return err
	}
	for _, item := range record.Backups {
		if item.Target == target {
			return fmt.Errorf("backup already exists for %s", target)
		}
	}
	record.Backups = append(record.Backups, backup{Target: target, Original: o, Applied: a, Absent: absent})
	// The durable backup is the pending checkpoint, before Editor.Apply.
	record.Operation.Steps = append(record.Operation.Steps, core.StepResult{Target: target, Action: "pending"})
	return s.save(record)
}

func (s *FileOperationStore) Load(id, target string) (string, string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.load(id)
	if err != nil {
		return "", "", false, err
	}
	for _, item := range record.Backups {
		if item.Target == target {
			return item.Original, item.Applied, item.Absent, nil
		}
	}
	return "", "", false, fmt.Errorf("backup target not found: %s", target)
}

func (s *FileOperationStore) Targets(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.load(id)
	if err != nil {
		return nil
	} // Callers must Get first to surface storage errors.
	var targets []string
	for i := len(record.Backups) - 1; i >= 0; i-- {
		targets = append(targets, record.Backups[i].Target)
	}
	return targets
}

func (s *FileOperationStore) RecordStep(id string, step core.StepResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.load(id)
	if err != nil {
		return err
	}
	for i := range record.Operation.Steps {
		if record.Operation.Steps[i].Target == step.Target {
			record.Operation.Steps[i] = step
			return s.save(record)
		}
	}
	return fmt.Errorf("pending step not found: %s", step.Target)
}

func (s *FileOperationStore) Finish(id, state string, steps []core.StepResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.load(id)
	if err != nil {
		return err
	}
	record.Operation.State = state
	record.Operation.Steps = steps
	return s.save(record)
}

func (s *FileOperationStore) Get(id string) (core.Operation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.load(id)
	return record.Operation, err
}

func (s *FileOperationStore) List() ([]core.Operation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.Root)
	if os.IsNotExist(err) {
		return []core.Operation{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := []core.Operation{}
	for _, entry := range entries {
		if !entry.IsDir() || !operationID.MatchString(entry.Name()) {
			continue
		}
		record, err := s.load(entry.Name())
		if err != nil {
			return nil, err
		}
		result = append(result, record.Operation)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].StartedAt.After(result[j].StartedAt) })
	return result, nil
}

// RecoverInterrupted must run once at startup, before accepting operations.
func (s *FileOperationStore) RecoverInterrupted() error {
	release, err := s.Lock()
	if err != nil {
		return err
	}
	defer release()
	ops, err := s.List()
	if err != nil {
		return err
	}
	for _, op := range ops {
		if op.State == "running" || op.State == "pending" || op.State == "cancelling" {
			if err := s.Finish(op.ID, "interrupted", op.Steps); err != nil {
				return err
			}
		}
	}
	return nil
}

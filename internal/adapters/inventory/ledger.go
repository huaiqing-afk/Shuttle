package inventory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	core "shuttle/internal/core/inventory"
)

// FileLedger is the install ledger of contract §9 D1. A missing file means
// Shuttle never installed anything (absent ≠ broken); unreadable or corrupt
// content is an error so callers can degrade to "unknown" instead of claiming
// the package is external.
type FileLedger struct {
	Path string
	mu   sync.Mutex
}

type ledgerFile struct {
	SchemaVersion int           `json:"schemaVersion"`
	Records       []core.Record `json:"records"`
}

const (
	// ledgerSchemaLegacy is schema 1: records without lifecycle state.
	ledgerSchemaLegacy = 1
	// ledgerSchema is what this build writes (contract §10.2).
	ledgerSchema = 2
)

func NewFileLedger(path string) *FileLedger { return &FileLedger{Path: path} }

func (l *FileLedger) Lookup(name string) (core.Record, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	doc, err := l.load()
	if err != nil {
		return core.Record{}, false, err
	}
	for _, record := range doc.Records {
		if record.Name == name {
			return record, true, nil
		}
	}
	return core.Record{}, false, nil
}

func (l *FileLedger) load() (ledgerFile, error) {
	raw, err := os.ReadFile(l.Path)
	if os.IsNotExist(err) {
		return ledgerFile{SchemaVersion: ledgerSchema}, nil
	}
	if err != nil {
		return ledgerFile{}, fmt.Errorf("读取安装台账失败: %w", err)
	}
	var doc ledgerFile
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ledgerFile{}, fmt.Errorf("安装台账损坏: %w", err)
	}
	switch doc.SchemaVersion {
	case ledgerSchema:
		return doc, nil
	case ledgerSchemaLegacy:
		// v1 records only ever existed after a successful write, so a missing
		// state means installed. Records with a state are already v2-shaped.
		for i := range doc.Records {
			if doc.Records[i].State == "" {
				doc.Records[i].State = core.StateInstalled
			}
		}
		return doc, nil
	default:
		return ledgerFile{}, fmt.Errorf("安装台账版本 %d 不受支持", doc.SchemaVersion)
	}
}

// List returns every record, used to reconcile interrupted installs.
func (l *FileLedger) List() ([]core.Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	doc, err := l.load()
	if err != nil {
		return nil, err
	}
	return doc.Records, nil
}

// Put records one state transition (contract §10.5). The caller holds the
// operations lock for the whole install, so this only serializes writers inside
// one process. A record is upserted by name: the ledger remembers one current
// state per package (D4), never a history.
func (l *FileLedger) Put(record core.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	doc, err := l.load()
	if err != nil {
		return err
	}
	replaced := false
	for i := range doc.Records {
		if doc.Records[i].Name == record.Name {
			doc.Records[i], replaced = record, true
			break
		}
	}
	if !replaced {
		doc.Records = append(doc.Records, record)
	}
	return l.save(doc.Records)
}

// Delete removes a record after successful uninstallation.
func (l *FileLedger) Delete(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	doc, err := l.load()
	if err != nil {
		return err
	}
	remaining := make([]core.Record, 0, len(doc.Records))
	for _, r := range doc.Records {
		if r.Name != name {
			remaining = append(remaining, r)
		}
	}
	return l.save(remaining)
}

// Save replaces the whole ledger. Write-side callers want Put; this stays for
// seeding and for tests that assert the stored document shape.
func (l *FileLedger) Save(records []core.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.save(records)
}

func (l *FileLedger) save(records []core.Record) error {
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o700); err != nil {
		return fmt.Errorf("创建台账目录失败: %w", err)
	}
	raw, err := json.MarshalIndent(ledgerFile{SchemaVersion: ledgerSchema, Records: records}, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化台账失败: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(l.Path), ".ledger-*")
	if err != nil {
		return fmt.Errorf("创建台账临时文件失败: %w", err)
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(raw); err != nil {
		temp.Close()
		return fmt.Errorf("写入台账临时文件失败: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("刷新台账临时文件失败: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("关闭台账临时文件失败: %w", err)
	}
	if err := os.Rename(temp.Name(), l.Path); err != nil {
		return fmt.Errorf("替换台账失败: %w", err)
	}
	return nil
}

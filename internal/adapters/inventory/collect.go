// Package inventory collects installed packages for core/inventory.
// Collection is read-only: it runs inspection commands and never installs or
// removes anything (docs/inventory-contract.md §1).
package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	core "shuttle/internal/core/inventory"
)

type Package = core.Package

const (
	SourceNPM    = core.SourceNPM
	SourceWinget = core.SourceWinget
)

// ErrSourceUnavailable marks a source that could not be collected. The caller
// shows this to the user; an unavailable source must never look like an empty
// machine (contract §6).
var ErrSourceUnavailable = errors.New("inventory source unavailable")

// commandRunner is injected so tests never spawn a real package manager.
type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

func (r commandRunner) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if r == nil {
		return nil, errors.New("no command runner configured")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	type outcome struct {
		out []byte
		err error
	}
	// A package manager that ignores its context must not hang the whole
	// detection; the caller gets the deadline instead (contract §6).
	done := make(chan outcome, 1)
	go func() {
		out, err := r(ctx, name, args...)
		done <- outcome{out, err}
	}()
	select {
	case result := <-done:
		return result.out, result.err
	case <-ctx.Done():
		return nil, fmt.Errorf("%w (command %s)", ctx.Err(), name)
	}
}

// Result is one source's outcome. Err is set when that source failed; Packages
// is then nil, so callers can present a partial inventory honestly.
type Result struct {
	Packages []Package
	Err      error
}

func (r Result) String() string {
	if r.Err != nil {
		return fmt.Sprintf("%d packages, error: %v", len(r.Packages), r.Err)
	}
	return fmt.Sprintf("%d packages", len(r.Packages))
}

// Collector reads the two sources frozen by contract §9 D2.
type Collector struct {
	Run commandRunner
}

// Timeout bounds each inspection command (contract §6). Callers pass a context
// with this deadline; Collect reports ErrSourceUnavailable when it expires.
const Timeout = 30 * time.Second

// CollectNPM reads the global npm tree. The root is attached to every package
// so the ledger can compare installation locations.
func (c Collector) CollectNPM(ctx context.Context) Result {
	rootRaw, err := c.Run.run(ctx, "npm", "root", "-g")
	if err != nil {
		return Result{Err: fmt.Errorf("%w: npm root -g: %w", ErrSourceUnavailable, err)}
	}
	root := strings.TrimSpace(string(rootRaw))
	listRaw, err := c.Run.run(ctx, "npm", "list", "-g", "--depth=0", "--json")
	if err != nil {
		return Result{Err: fmt.Errorf("%w: npm list: %w", ErrSourceUnavailable, err)}
	}
	packages, parseErr := ParseNPM(listRaw, root)
	if parseErr != nil {
		return Result{Err: parseErr}
	}
	return Result{Packages: packages}
}

// CollectWinget reads winget's structured export. `winget list` is deliberately
// not parsed: its fixed-width text table merges the name and ID columns for long
// names, and its header is localized (contract §2).
func (c Collector) CollectWinget(ctx context.Context, outPath string) Result {
	if _, err := c.Run.run(ctx, "winget", "export", "-o", outPath,
		"--include-versions", "--accept-source-agreements", "--disable-interactivity"); err != nil {
		return Result{Err: fmt.Errorf("%w: winget export: %w", ErrSourceUnavailable, err)}
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		return Result{Err: fmt.Errorf("%w: 读取 winget 导出结果: %w", ErrSourceUnavailable, err)}
	}
	packages, parseErr := ParseWingetExport(raw)
	if parseErr != nil {
		return Result{Err: parseErr}
	}
	return Result{Packages: packages}
}

// ErrParseFailed marks output we refuse to interpret half-way (contract §6).
var ErrParseFailed = errors.New("inventory output unparsable")

// ParseNPM decodes `npm list -g --json`. Unparseable output fails the whole
// source: a partial tree would misreport what is installed.
func ParseNPM(raw []byte, root string) ([]Package, error) {
	var tree struct {
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, fmt.Errorf("%w: npm list: %v", ErrParseFailed, err)
	}
	packages := make([]Package, 0, len(tree.Dependencies))
	for name, dep := range tree.Dependencies {
		packages = append(packages, Package{Name: name, Version: dep.Version, Source: SourceNPM, Root: root})
	}
	sortPackages(packages)
	return packages, nil
}

// ParseWingetExport decodes the winget packages schema. Only SourceDetails whose
// name is "winget" are accepted; export can list other origins, and their IDs are
// not uninstallable through winget.
func ParseWingetExport(raw []byte) ([]Package, error) {
	var doc struct {
		Sources []struct {
			Packages []struct {
				PackageIdentifier string `json:"PackageIdentifier"`
				Version           string `json:"Version"`
			} `json:"Packages"`
			SourceDetails struct {
				Name string `json:"Name"`
			} `json:"SourceDetails"`
		} `json:"Sources"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%w: winget export: %v", ErrParseFailed, err)
	}
	var packages []Package
	for _, source := range doc.Sources {
		if !strings.EqualFold(source.SourceDetails.Name, "winget") {
			continue
		}
		for _, pkg := range source.Packages {
			if pkg.PackageIdentifier == "" {
				continue
			}
			packages = append(packages, Package{
				Name: pkg.PackageIdentifier, Version: pkg.Version, Source: SourceWinget, Root: "winget"})
		}
	}
	sortPackages(packages)
	return packages, nil
}

// CoverageNote documents what the winget source cannot see. The caller surfaces
// it; silently reporting "no other software" would be a false claim.
const CoverageNote = "winget 导出只包含已关联 winget 源的包；ARP/MSIX 来源的已安装程序不会出现在结果中"

func sortPackages(packages []core.Package) {
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Source != packages[j].Source {
			return packages[i].Source < packages[j].Source
		}
		return packages[i].Name < packages[j].Name
	})
}

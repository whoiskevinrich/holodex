package extract

import (
	"fmt"
	"os"
	"sync/atomic"

	"gopkg.in/yaml.v3"
)

// patternFile is the raw shape of metadata-patterns.yaml (F48.1a): an
// ordered pattern list plus the multi-value delimiter (F48.1d).
type patternFile struct {
	Delimiter string   `yaml:"delimiter"`
	Patterns  []string `yaml:"patterns"`
}

type compiledPatterns struct {
	patterns  []*Pattern
	delimiter string
}

// PatternStore holds the current compiled filename-pattern list behind an
// atomic pointer, mirroring internal/mapping.Store, so reads are lock-free
// and a runtime reload (POST /admin/reload-config) swaps the list in
// atomically — F48.1a's "editable at runtime" requirement.
type PatternStore struct {
	path string
	cur  atomic.Pointer[compiledPatterns]
}

// NewPatternStore loads and compiles the initial pattern list from path. A
// missing file is fine — the pattern list is simply empty, so every filename
// falls through to tag-only resolution unchanged (F48.1b).
func NewPatternStore(path string) (*PatternStore, error) {
	s := &PatternStore{path: path}
	c, err := loadPatterns(path)
	if err != nil {
		return nil, err
	}
	s.cur.Store(c)
	return s, nil
}

// Reload re-reads and recompiles the pattern file, swapping it in atomically.
// An unparseable pattern (F48.1a's save-time validation) fails the reload and
// leaves the previous, known-good list in place.
func (s *PatternStore) Reload() error {
	c, err := loadPatterns(s.path)
	if err != nil {
		return err
	}
	s.cur.Store(c)
	return nil
}

// Current returns the live compiled patterns and delimiter; callers treat the
// result as immutable.
func (s *PatternStore) Current() (patterns []*Pattern, delimiter string) {
	c := s.cur.Load()
	return c.patterns, c.delimiter
}

func loadPatterns(path string) (*compiledPatterns, error) {
	if path == "" {
		return &compiledPatterns{}, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &compiledPatterns{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read filename patterns %s: %w", path, err)
	}
	var pf patternFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("parse filename patterns: %w", err)
	}
	compiled, err := CompileAll(pf.Patterns)
	if err != nil {
		return nil, fmt.Errorf("compile filename patterns: %w", err)
	}
	return &compiledPatterns{patterns: compiled, delimiter: pf.Delimiter}, nil
}

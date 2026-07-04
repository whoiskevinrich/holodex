// Package mapping implements configurable metadata field mapping (F20/F27, ADR-013):
// it maps one or more namespaced sources to a single canonical Holodex field with a
// display label, honoring source precedence and optional multi-value splitting.
//
// # Source namespace syntax
//
// Each entry in the `sources` list is a namespaced reference:
//
//	file:title        — special alias for videos.title (the scanner's primary title)
//	file:<Key>        — raw file tag from extra_metadata (case-insensitive key match)
//	<provider>:<key>  — enrichment shadow field: provider name + field key
//	                    e.g. "tmdb:title", "tmdb:genres"
//
// Legacy entries without a colon are treated as file:<Key> for backwards compat.
//
// Because the scanner already captures every raw tag at index time (F2.9),
// enabling or changing a mapping is pure re-interpretation — no re-scan.
package mapping

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"gopkg.in/yaml.v3"

	"holodex/internal/model"
	"holodex/internal/registry"
)

// Source is a parsed namespaced source reference from the `sources` list.
type Source struct {
	// Namespace is "file" for file-sourced data, or a provider name ("tmdb", …)
	// for enrichment shadow data.
	Namespace string
	// Key is the field key within the namespace:
	//   file namespace:     "title" (special alias for videos.title) or a raw tag key
	//   provider namespace: the enrichment field_key (e.g. "title", "genres")
	Key string
}

// IsFileTitle reports whether this source refers to videos.title directly.
func (s Source) IsFileTitle() bool {
	return s.Namespace == "file" && strings.ToLower(s.Key) == "title"
}

// Field is one canonical field built from a precedence-ordered list of namespaced
// sources. The first source with a non-empty value wins.
type Field struct {
	Canonical string   `yaml:"canonical"`
	Label     string   `yaml:"label"`
	// Display optionally overrides the registry render mode for this field (F39,
	// ADR-056): "" defers to the registry, else one of the render-mode vocabulary
	// ("long_text" | "url" | "image_url" | "chips"). An operator mapping may set it,
	// and a synthesized auto-registered field carries the provider-hinted mode here.
	Display string `yaml:"display"`
	// Sources is the raw list as written in YAML (e.g. ["tmdb:title", "file:Title"]).
	// ParsedSources is the authoritative form after parse().
	Sources       []string `yaml:"sources"`
	ParsedSources []Source `yaml:"-"`
	Filterable    bool     `yaml:"filterable"`
	Multi         bool     `yaml:"multi"` // split/aggregate multiple values
	// Merge (F30) forces merge-mode resolution (deduplicated cross-source union)
	// even for a single-valued field. multi:true implies merge.
	Merge bool `yaml:"merge"`
	// Casing (F30, decision #4) sets the output casing applied to this field's
	// resolved values: "" / "preserve" | "lower" | "upper" | "title". Dedup stays
	// case-insensitive regardless; this only affects the displayed/written form.
	Casing string `yaml:"casing"`
	// Browse, when true, means the resolved value for this field should replace
	// video.Title in the list-media response so browse cards reflect the
	// highest-precedence source (e.g. a TMDB title over a filename-derived title).
	Browse bool `yaml:"browse"`
}

type fileConfig struct {
	Fields []Field `yaml:"fields"`
}

// Mappings is an immutable, parsed mapping configuration (swapped atomically on
// reload, never mutated in place).
type Mappings struct {
	fields []Field
}

// Empty is the no-mappings configuration (the default when no file is present).
func Empty() *Mappings { return &Mappings{} }

func parse(data []byte) (*Mappings, error) {
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse metadata mappings: %w", err)
	}
	m := &Mappings{}
	for _, f := range fc.Fields {
		f.Canonical = strings.TrimSpace(f.Canonical)
		if f.Canonical == "" || len(f.Sources) == 0 {
			continue // skip malformed entries rather than failing the whole load
		}
		if f.Label == "" {
			// Fall back to the registry label, then to the canonical name.
			if def := registry.Lookup(f.Canonical); def.Label != "" {
				f.Label = def.Label
			} else {
				f.Label = f.Canonical
			}
		}
		f.ParsedSources = parseSources(f.Sources)
		m.fields = append(m.fields, f)
	}
	return m, nil
}

// parseSources splits each raw source string on the first colon.
// "tmdb:title"  → {Namespace:"tmdb", Key:"title"}
// "file:Title"  → {Namespace:"file", Key:"Title"}
// "Title"       → {Namespace:"file", Key:"Title"}  (legacy: no colon → file tag)
func parseSources(raw []string) []Source {
	out := make([]Source, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if i := strings.IndexByte(s, ':'); i > 0 {
			out = append(out, Source{
				Namespace: strings.ToLower(s[:i]),
				Key:       s[i+1:],
			})
		} else {
			// Legacy bare key — treat as a file tag.
			out = append(out, Source{Namespace: "file", Key: s})
		}
	}
	return out
}

// Load reads mappings from path. A missing file yields Empty (mappings are
// optional), not an error.
func Load(path string) (*Mappings, error) {
	if path == "" {
		return Empty(), nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Empty(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read metadata mappings %s: %w", path, err)
	}
	return parse(data)
}

// Fields returns all configured fields in declaration order.
func (m *Mappings) Fields() []Field { return m.fields }

// Filterable returns the subset of fields marked filterable (facet candidates).
func (m *Mappings) Filterable() []Field {
	out := make([]Field, 0, len(m.fields))
	for _, f := range m.fields {
		if f.Filterable {
			out = append(out, f)
		}
	}
	return out
}

// ByCanonical resolves a field by its canonical name (case-insensitive). Field
// counts are tiny (tens), so a linear scan beats a parallel index to maintain.
func (m *Mappings) ByCanonical(canonical string) (Field, bool) {
	canonical = strings.ToLower(strings.TrimSpace(canonical))
	for _, f := range m.fields {
		if strings.ToLower(f.Canonical) == canonical {
			return f, true
		}
	}
	return Field{}, false
}

// Resolved is a canonical field with the value(s) found for one video via the
// legacy file-only resolution path (Resolve). The unified resolver that merges
// enrichment sources lives in internal/resolver.
type Resolved struct {
	Canonical string   `json:"canonical"`
	Label     string   `json:"label"`
	Values    []string `json:"values"`
}

// Resolve interprets a video's raw metadata through the file-only sources in the
// mappings: for each field, the first file: source key present (precedence)
// supplies the value. Provider sources (tmdb:*, …) are skipped — the unified
// resolver in internal/resolver handles those.
//
// This method preserves backwards compatibility for callers that don't yet have
// enrichment data available (e.g. the MCP server, the facets endpoint).
func (m *Mappings) Resolve(extra []model.ExtraMetadata) []Resolved {
	bySource := make(map[string][]string, len(extra))
	for _, e := range extra {
		k := strings.ToLower(strings.TrimSpace(e.SourceKey))
		bySource[k] = append(bySource[k], e.Value)
	}

	out := make([]Resolved, 0, len(m.fields))
	for _, f := range m.fields {
		var values []string
		for _, src := range f.ParsedSources {
			if src.Namespace != "file" || src.IsFileTitle() {
				continue // skip provider sources and the special file:title alias
			}
			vals := bySource[strings.ToLower(strings.TrimSpace(src.Key))]
			if len(vals) == 0 {
				continue
			}
			if f.Multi {
				for _, v := range vals {
					values = append(values, SplitMulti(v)...)
				}
			} else {
				values = []string{strings.TrimSpace(vals[0])}
			}
			break // precedence: first present source wins
		}
		if values = Dedupe(values); len(values) > 0 {
			out = append(out, Resolved{Canonical: f.Canonical, Label: f.Label, Values: values})
		}
	}
	return out
}

// SplitMulti splits a multi-valued tag on common separators and trims each part.
// Exported so the resolver package can reuse it without duplicating the logic.
func SplitMulti(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '/' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// Dedupe removes case-insensitive duplicates from a string slice, preserving
// the first occurrence of each value. Exported for use by the resolver package.
func Dedupe(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := in[:0]
	for _, v := range in {
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
}

// Store holds the current Mappings behind an atomic pointer so reads are
// lock-free and reload (POST /admin/reload-config, F20.10) swaps atomically.
type Store struct {
	path string
	cur  atomic.Pointer[Mappings]
}

// NewStore loads the initial mappings from path (missing file is fine).
func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	m, err := Load(path)
	if err != nil {
		return nil, err
	}
	s.cur.Store(m)
	return s, nil
}

// Current returns the live mappings; callers treat the result as immutable.
func (s *Store) Current() *Mappings { return s.cur.Load() }

// Reload re-reads the config file and swaps it in atomically (F20.10).
func (s *Store) Reload() error {
	m, err := Load(s.path)
	if err != nil {
		return err
	}
	s.cur.Store(m)
	return nil
}

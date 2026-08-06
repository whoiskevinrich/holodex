package enrich

import (
	"bytes"
	"log/slog"
	"testing"
)

// newTestService builds a bare Service with just enough wiring for
// persistPreferredPattern/PreferredSearchPattern — no repo/store/client needed since
// those methods touch neither (ADR-080 D2 tier 2, FR2).
func newTestService(t *testing.T, buf *bytes.Buffer) *Service {
	t.Helper()
	return &Service{log: slog.New(slog.NewTextHandler(buf, nil))}
}

func TestPersistPreferredPattern_CachesValidPattern(t *testing.T) {
	var buf bytes.Buffer
	s := newTestService(t, &buf)

	if _, ok := s.PreferredSearchPattern("tmdb"); ok {
		t.Fatal("nothing cached yet should report ok=false")
	}

	s.persistPreferredPattern("tmdb", Manifest{PreferredSearchPattern: "{studio?} {title?}"})
	got, ok := s.PreferredSearchPattern("tmdb")
	if !ok || got != "{studio?} {title?}" {
		t.Fatalf("PreferredSearchPattern = %q, %v; want '{studio?} {title?}', true", got, ok)
	}
}

func TestPersistPreferredPattern_InvalidPatternDroppedAndLogged(t *testing.T) {
	var buf bytes.Buffer
	s := newTestService(t, &buf)

	s.persistPreferredPattern("tmdb", Manifest{PreferredSearchPattern: "{director?}"}) // unknown token
	if _, ok := s.PreferredSearchPattern("tmdb"); ok {
		t.Fatal("an invalid preferred_search_pattern must never be cached")
	}
	if !bytes.Contains(buf.Bytes(), []byte("tmdb")) {
		t.Errorf("expected a warning naming the provider, got log: %s", buf.String())
	}
}

func TestPersistPreferredPattern_EmptyClearsPriorValue(t *testing.T) {
	var buf bytes.Buffer
	s := newTestService(t, &buf)

	s.persistPreferredPattern("tmdb", Manifest{PreferredSearchPattern: "{title?}"})
	s.persistPreferredPattern("tmdb", Manifest{}) // provider stops advertising a preference
	if _, ok := s.PreferredSearchPattern("tmdb"); ok {
		t.Fatal("an absent preferred_search_pattern on a later /describe should clear the cache")
	}
}

func TestPersistPreferredPattern_PerProviderIsolation(t *testing.T) {
	var buf bytes.Buffer
	s := newTestService(t, &buf)

	s.persistPreferredPattern("tmdb", Manifest{PreferredSearchPattern: "{studio?}"})
	s.persistPreferredPattern("other", Manifest{PreferredSearchPattern: "{title?}"})

	if got, ok := s.PreferredSearchPattern("tmdb"); !ok || got != "{studio?}" {
		t.Errorf("tmdb pattern = %q, %v", got, ok)
	}
	if got, ok := s.PreferredSearchPattern("other"); !ok || got != "{title?}" {
		t.Errorf("other pattern = %q, %v", got, ok)
	}
}

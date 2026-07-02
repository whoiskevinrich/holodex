// Package refresh implements per-item metadata refresh (F31, ADR-047): an
// owner-gated action that re-reads one media file's embedded metadata — forced,
// bypassing the scanner's (size, mtime) change-detection so it catches edits
// another system made even when the file's mtime is unchanged — AND re-pulls the
// providers the item is matched to, as one operation.
//
// Two layers, kept strictly separate (the non-destructive layering invariant): the
// file re-extract writes only the file layer (videos + video_metadata); the
// re-enrich writes only each provider's shadow-store layer. Neither flattens the
// other — the resolver remains the sole merge point, so any future conflict policy
// stays implementable without re-extraction.
//
// The service reads the file (plan) then commits the file and re-enriches
// (apply); the file commit lands before any provider call, so a provider failure
// never undoes the file sync, and a file-read failure aborts before any write. A
// future batch op (F31.11) is what needs the finer provider plan/apply split and
// the per-field sources_disagree computation; this slice carries the report shape
// for it but leaves SourcesDisagree reserved.
package refresh

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"holodex/internal/enrich"
	"holodex/internal/model"
)

// FileExtractor builds a candidate Video from a file without persisting it — the
// read half of a refresh, with no change-detection. Implemented by
// *scanner.Scanner.BuildVideoFromFile.
type FileExtractor interface {
	BuildVideoFromFile(ctx context.Context, path string) (*model.Video, []model.ExtraMetadata, error)
}

// Store resolves a refresh target, reads the current row for change detection,
// and persists the file layer. Implemented by *repo.Repo. RefreshTarget must
// distinguish a missing row from a soft-deleted one (repo.ErrNotFound vs
// repo.ErrDeleted) so the handler can answer 404 vs 409; those sentinels
// propagate unwrapped through Refresh.
type Store interface {
	RefreshTarget(ctx context.Context, id int64) (path string, err error)
	GetVideo(ctx context.Context, id int64) (*model.Video, []model.ExtraMetadata, error)
	UpsertVideo(ctx context.Context, v *model.Video, extra []model.ExtraMetadata) (int64, error)
	// RecordJobRun appends one combined refresh row to the activity history
	// (F31.6). Best-effort — a recording failure never fails the refresh.
	RecordJobRun(ctx context.Context, run model.JobRun) error
}

// Enricher re-runs an entity's linked providers (F31.3) without recording its own
// activity row. Implemented by *enrich.Service; nil disables the provider half
// (a file-only refresh).
type Enricher interface {
	ProviderMatches(ctx context.Context, entityType string, entityID int64) ([]enrich.Match, error)
	ReEnrich(ctx context.Context, entityType string, entityID int64, provider, externalID string) ([]model.EnrichedField, error)
}

// Service orchestrates a per-item refresh.
type Service struct {
	ext    FileExtractor
	store  Store
	enrich Enricher                           // nil = file-only refresh
	relink func(context.Context, int64) error // F38: re-derive studio links post-refresh
	log    *slog.Logger
}

// NewService wires the refresh service (F31). ext re-reads files (the scanner);
// store resolves targets and persists (the repo); enricher re-pulls linked
// providers (the enrich service, or nil for file-only). log is best-effort and
// may be nil.
func NewService(ext FileExtractor, store Store, enricher Enricher, log *slog.Logger) *Service {
	return &Service{ext: ext, store: store, enrich: enricher, log: log}
}

// SetRelinker wires studio-link derivation (F38, ADR-053): after a refresh
// re-extracts + re-enriches an item, its studio links are re-derived from the new
// resolved `studio` value. Best-effort — a relink error is logged, never failing the
// refresh. Called once at startup; nil disables it.
func (s *Service) SetRelinker(fn func(context.Context, int64) error) { s.relink = fn }

// SourceResult is the outcome for one source in a refresh. For the file source,
// Changed reflects a real diff of the file layer. For a provider, OK reports the
// re-fetch succeeded and Changed mirrors it (a precise provider value-diff is
// deferred to the batch consumer); Error carries a generic message on failure.
type SourceResult struct {
	Source  string `json:"source"` // "file" or the provider name
	OK      bool   `json:"ok"`
	Changed bool   `json:"changed"`
	Error   string `json:"error,omitempty"`
}

// Report is the typed outcome of a refresh — the single source of truth for the
// HTTP response and (a later slice) the activity detail. Sources carries one entry
// per attempted source (file first, then each provider). SourcesDisagree is
// reserved: it is computed and consumed by the future batch conflict-resolution
// op (F31.11), not surfaced single-item, so this slice leaves it false.
type Report struct {
	VideoID         int64          `json:"video_id"`
	Sources         []SourceResult `json:"sources"`
	Changed         bool           `json:"changed"`
	SourcesDisagree bool           `json:"sources_disagree"`
}

// Refresh re-extracts one media file (forced) and re-enriches its linked
// providers, returning a report. A missing/soft-deleted id, or a file that can't
// be read, returns an error before any write — the row is never mutated on
// failure. The file commit precedes every provider call, so a provider failure is
// isolated to its own source result and never undoes the file sync.
func (s *Service) Refresh(ctx context.Context, id int64) (Report, error) {
	path, err := s.store.RefreshTarget(ctx, id)
	if err != nil {
		// ErrNotFound / ErrDeleted: a rejected request (404/409), not a run — not
		// recorded in the activity history.
		return Report{}, err
	}
	started := time.Now()
	report, runErr := s.run(ctx, id, path)
	s.record(started, report, runErr)
	return report, runErr
}

// run does the refresh work and returns the report; Refresh wraps it with
// activity recording. The file commit precedes every provider call, so a provider
// failure is isolated and never undoes the file sync; a file-read failure aborts
// before any write.
func (s *Service) run(ctx context.Context, id int64, path string) (Report, error) {
	// Old snapshot for change detection — best effort; a read failure just means
	// we report the file as changed (we can't prove it wasn't).
	oldV, oldExtra, _ := s.store.GetVideo(ctx, id)

	// plan (read): force re-extract. A file error aborts before any write.
	newV, newExtra, err := s.ext.BuildVideoFromFile(ctx, path)
	if err != nil {
		return Report{VideoID: id}, err
	}

	// apply (write): file layer first, so it lands regardless of provider outcome.
	if _, err := s.store.UpsertVideo(ctx, newV, newExtra); err != nil {
		return Report{VideoID: id}, err
	}

	report := Report{VideoID: id}
	report.Sources = append(report.Sources, SourceResult{
		Source:  "file",
		OK:      true,
		Changed: fileLayerChanged(oldV, oldExtra, newV, newExtra),
	})

	report.Sources = append(report.Sources, s.reEnrichLinked(ctx, id)...)

	for _, sr := range report.Sources {
		if sr.Changed {
			report.Changed = true
		}
	}
	// Re-derive studio links from the refreshed resolution (F38, ADR-053).
	if s.relink != nil {
		if err := s.relink(ctx, id); err != nil && s.log != nil {
			s.log.Warn("studio relink after refresh failed", "id", id, "err", err)
		}
	}
	return report, nil
}

// record appends one combined refresh row to the activity history (F31.6,
// ADR-047). Best-effort on a detached context (like the enrich/scanner recorders)
// so a failed/cancelled refresh — the case history most needs — still records. The
// detail carries the item id and per-source outcome only: no filesystem path, env
// value, or token (the ADR-028 no-secrets invariant).
func (s *Service) record(started time.Time, report Report, runErr error) {
	now := time.Now()
	run := model.JobRun{
		Kind:       model.JobKindRefresh,
		Trigger:    model.TriggerManual,
		Status:     model.JobStatusOK,
		StartedAt:  started,
		FinishedAt: now,
		DurationMs: now.Sub(started).Milliseconds(),
	}
	switch {
	case runErr != nil:
		run.Status = model.JobStatusErr
		run.Errors = 1
		run.Detail = fmt.Sprintf("#%d — file failed", report.VideoID)
		run.ErrorMessage = "refresh failed"
	default:
		run.Detail = refreshDetail(report)
		failed := 0
		for _, sr := range report.Sources {
			if !sr.OK {
				failed++
			}
		}
		if failed > 0 {
			run.Status = model.JobStatusErr
			run.Errors = failed
			run.ErrorMessage = "one or more providers failed"
		}
	}
	recCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.store.RecordJobRun(recCtx, run); err != nil {
		s.warn("refresh: record job run", "err", err)
	}
}

// refreshDetail renders a no-secrets one-line summary for the activity row, e.g.
// "#42 — file changed; tmdb updated; imdb failed".
func refreshDetail(r Report) string {
	parts := make([]string, 0, len(r.Sources))
	for _, sr := range r.Sources {
		switch {
		case sr.Source == "file":
			if sr.Changed {
				parts = append(parts, "file changed")
			} else {
				parts = append(parts, "file unchanged")
			}
		case sr.OK:
			parts = append(parts, sr.Source+" updated")
		default:
			parts = append(parts, sr.Source+" failed")
		}
	}
	return fmt.Sprintf("#%d — %s", r.VideoID, strings.Join(parts, "; "))
}

// reEnrichLinked re-fetches every provider the item is matched to, isolating each
// failure to its own source result (a flaky provider never fails the refresh).
// Returns nil when there is no enricher or no match (a clean file-only refresh).
func (s *Service) reEnrichLinked(ctx context.Context, id int64) []SourceResult {
	if s.enrich == nil {
		return nil
	}
	matches, err := s.enrich.ProviderMatches(ctx, model.EnrichEntityVideo, id)
	if err != nil {
		s.warn("refresh: list provider matches", "id", id, "err", err)
		return nil
	}
	out := make([]SourceResult, 0, len(matches))
	for _, m := range matches {
		res := SourceResult{Source: m.Provider, OK: true, Changed: true}
		if _, err := s.enrich.ReEnrich(ctx, model.EnrichEntityVideo, id, m.Provider, m.ExternalID); err != nil {
			s.warn("refresh: re-enrich provider", "id", id, "provider", m.Provider, "err", err)
			res.OK, res.Changed = false, false
			res.Error = "provider lookup failed"
		}
		out = append(out, res)
	}
	return out
}

func (s *Service) warn(msg string, args ...any) {
	if s.log != nil {
		s.log.Warn(msg, args...)
	}
}

// fileLayerChanged reports whether the re-extracted file layer differs from what
// was stored. A nil old snapshot (read failure / brand-new) counts as changed.
func fileLayerChanged(oldV *model.Video, oldExtra []model.ExtraMetadata, newV *model.Video, newExtra []model.ExtraMetadata) bool {
	if oldV == nil || newV == nil {
		return true
	}
	if oldV.Title != newV.Title ||
		oldV.Duration != newV.Duration ||
		oldV.Width != newV.Width || oldV.Height != newV.Height ||
		oldV.VideoCodec != newV.VideoCodec || oldV.AudioCodec != newV.AudioCodec ||
		oldV.BitrateKbps != newV.BitrateKbps || oldV.Container != newV.Container {
		return true
	}
	if !sameNames(personNames(oldV.People), personNames(newV.People)) ||
		!sameNames(tagNames(oldV.Tags), tagNames(newV.Tags)) ||
		!sameNames(extraPairs(oldExtra), extraPairs(newExtra)) {
		return true
	}
	return false
}

func personNames(ps []model.Person) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name
	}
	return out
}

func tagNames(ts []model.Tag) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Name
	}
	return out
}

func extraPairs(extra []model.ExtraMetadata) []string {
	out := make([]string, len(extra))
	for i, e := range extra {
		out[i] = e.SourceKey + "\x00" + e.Value
	}
	return out
}

// sameNames compares two string collections as multisets (order-independent).
func sameNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as := append([]string(nil), a...)
	bs := append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

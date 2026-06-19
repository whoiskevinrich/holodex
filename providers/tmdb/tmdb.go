package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var errNotFound = errors.New("not found")

// tmdbClient calls the TMDB v3 REST API. It holds no Holodex state; Holodex
// persists all enrichment. The client is stateless with respect to Holodex so
// restarting the container loses nothing Holodex relies on (spec §5).
type tmdbClient struct {
	token    string // bearer token (preferred; Authorization: Bearer <token>)
	apiKey   string // legacy API key (query param fallback)
	language string // TMDB language param (e.g. "en-US")
	hc       *http.Client
}

func newTMDBClient(token, apiKey, language string) *tmdbClient {
	return &tmdbClient{
		token:    token,
		apiKey:   apiKey,
		language: language,
		hc:       &http.Client{Timeout: 6 * time.Second}, // well inside Holodex's 8s budget
	}
}

// ---- provider wire types ----

type describeResponse struct {
	Provider        string   `json:"provider"`
	Version         string   `json:"version"`
	ProtocolVersion int      `json:"protocol_version"`
	EntityTypes     []string `json:"entity_types"`
	IDNamespaces    []string `json:"id_namespaces"`
	Fields          []string `json:"fields"`
	AssetKinds      []string `json:"asset_kinds,omitempty"`
}

type candidate struct {
	ExternalID     string  `json:"external_id"`
	Namespace      string  `json:"namespace"`
	Label          string  `json:"label"`
	Confidence     float64 `json:"confidence"`
	Disambiguation string  `json:"disambiguation,omitempty"`
}

type assetEntry struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

type enrichResponse struct {
	Fields map[string][]string `json:"fields"`
	Assets []assetEntry        `json:"assets,omitempty"`
}

// ---- TMDB API response shapes ----

type searchResult struct {
	Results []tmdbPerson `json:"results"`
}

type tmdbPerson struct {
	ID                 int        `json:"id"`
	Name               string     `json:"name"`
	Popularity         float64    `json:"popularity"`
	KnownForDepartment string     `json:"known_for_department"`
	KnownFor           []knownFor `json:"known_for"`
}

type knownFor struct {
	Title       string `json:"title"`        // movie title
	Name        string `json:"name"`         // TV show name
	ReleaseDate string `json:"release_date"` // YYYY-MM-DD or ""
}

type personDetails struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Biography    string   `json:"biography"`
	Birthday     string   `json:"birthday"`
	Deathday     string   `json:"deathday"`
	PlaceOfBirth string   `json:"place_of_birth"`
	Homepage     string   `json:"homepage"`
	ProfilePath  string   `json:"profile_path"`
	AlsoKnownAs  []string `json:"also_known_as"`
}

type findResult struct {
	PersonResults []tmdbPerson `json:"person_results"`
}

// ---- resolve ----

func (c *tmdbClient) resolve(ctx context.Context, h hintBody) ([]candidate, error) {
	// Embedded-ID path: fast and deterministic.
	for _, id := range h.ExternalIDs {
		ns, val, ok := splitID(id)
		if !ok {
			continue
		}
		switch ns {
		case "tmdb":
			n, err := strconv.Atoi(val)
			if err != nil {
				continue
			}
			det, err := c.fetchDetails(ctx, n)
			if err != nil {
				return nil, err
			}
			return []candidate{{
				ExternalID: fmt.Sprintf("tmdb:%d", det.ID),
				Namespace:  "tmdb",
				Label:      det.Name,
				Confidence: 1.0,
			}}, nil
		case "imdb":
			cands, err := c.findByIMDB(ctx, val)
			if err != nil {
				return nil, err
			}
			if len(cands) > 0 {
				return cands, nil
			}
		}
	}

	// Name-search fallback.
	if h.Query == "" {
		return []candidate{}, nil
	}
	return c.searchPerson(ctx, h.Query)
}

func (c *tmdbClient) searchPerson(ctx context.Context, query string) ([]candidate, error) {
	var result searchResult
	err := c.get(ctx, "/3/search/person", url.Values{
		"query":         {query},
		"include_adult": {"false"},
		"language":      {c.language},
		"page":          {"1"},
	}, &result)
	if err != nil {
		return nil, err
	}
	out := make([]candidate, 0, len(result.Results))
	for i, p := range result.Results {
		if i >= 10 {
			break
		}
		out = append(out, candidate{
			ExternalID:     fmt.Sprintf("tmdb:%d", p.ID),
			Namespace:      "tmdb",
			Label:          p.Name,
			Confidence:     rankConfidence(i, p.Popularity),
			Disambiguation: disambiguate(p),
		})
	}
	return out, nil
}

func (c *tmdbClient) findByIMDB(ctx context.Context, imdbID string) ([]candidate, error) {
	var result findResult
	err := c.get(ctx, "/3/find/"+url.PathEscape(imdbID), url.Values{
		"external_source": {"imdb_id"},
		"language":        {c.language},
	}, &result)
	if err != nil {
		return nil, err
	}
	out := make([]candidate, 0, len(result.PersonResults))
	for _, p := range result.PersonResults {
		out = append(out, candidate{
			ExternalID:     fmt.Sprintf("tmdb:%d", p.ID),
			Namespace:      "tmdb",
			Label:          p.Name,
			Confidence:     0.95,
			Disambiguation: disambiguate(p),
		})
	}
	return out, nil
}

// ---- enrich ----

func (c *tmdbClient) enrich(ctx context.Context, externalID string) (enrichResponse, error) {
	ns, val, ok := splitID(externalID)
	if !ok || ns != "tmdb" {
		return enrichResponse{}, fmt.Errorf("%w: %q", errNotFound, externalID)
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return enrichResponse{}, fmt.Errorf("%w: %q", errNotFound, externalID)
	}
	det, err := c.fetchDetails(ctx, n)
	if err != nil {
		return enrichResponse{}, err
	}
	return buildEnrichResponse(det), nil
}

func (c *tmdbClient) fetchDetails(ctx context.Context, id int) (personDetails, error) {
	var det personDetails
	err := c.get(ctx, fmt.Sprintf("/3/person/%d", id), url.Values{
		"language": {c.language},
	}, &det)
	return det, err
}

func buildEnrichResponse(det personDetails) enrichResponse {
	fields := make(map[string][]string)
	if bio := strings.TrimSpace(det.Biography); bio != "" {
		fields["bio"] = []string{trimAtSentence(bio, 4000)}
	}
	if det.Birthday != "" {
		fields["birthdate"] = []string{det.Birthday}
	}
	if det.Deathday != "" {
		fields["deathdate"] = []string{det.Deathday}
	}
	if pob := strings.TrimSpace(det.PlaceOfBirth); pob != "" {
		fields["nationality"] = []string{pob}
	}
	if hp := strings.TrimSpace(det.Homepage); hp != "" {
		fields["website"] = []string{hp}
	}
	var aliases []string
	for _, a := range det.AlsoKnownAs {
		if a = strings.TrimSpace(a); a != "" {
			aliases = append(aliases, a)
		}
	}
	if len(aliases) > 0 {
		fields["aliases"] = aliases
	}

	var assets []assetEntry
	if det.ProfilePath != "" {
		assets = []assetEntry{{
			Kind: "photo",
			URL:  "https://image.tmdb.org/t/p/original" + det.ProfilePath,
		}}
	}
	return enrichResponse{Fields: fields, Assets: assets}
}

// ---- HTTP transport ----

func (c *tmdbClient) get(ctx context.Context, path string, params url.Values, out any) error {
	if c.apiKey != "" && c.token == "" {
		params.Set("api_key", c.apiKey)
	}
	u := url.URL{
		Scheme:   "https",
		Host:     "api.themoviedb.org",
		Path:     path,
		RawQuery: params.Encode(),
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("build request %s: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("%w: TMDB %s", errNotFound, path)
	case http.StatusTooManyRequests:
		return fmt.Errorf("TMDB rate-limited (429) on %s", path)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("TMDB %s returned %d", path, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return json.Unmarshal(data, out)
}

// ---- helpers ----

func splitID(id string) (ns, val string, ok bool) {
	i := strings.IndexByte(id, ':')
	if i <= 0 {
		return "", "", false
	}
	return id[:i], id[i+1:], true
}

// rankConfidence converts a 0-based search rank and TMDB popularity into a 0–1
// confidence score. Rank is dominant; popularity adds a small boost for well-known
// results. Holodex does not threshold on this — any monotonic value is acceptable.
func rankConfidence(rank int, popularity float64) float64 {
	v := 1.0 - float64(rank)*0.08
	if v < 0.1 {
		v = 0.1
	}
	if popularity > 10 && v < 0.95 {
		v += 0.05
	}
	return v
}

// disambiguate builds the short hint string shown in the Holodex candidate picker.
func disambiguate(p tmdbPerson) string {
	var parts []string
	if p.KnownForDepartment != "" {
		parts = append(parts, p.KnownForDepartment)
	}
	for _, kf := range p.KnownFor {
		title := kf.Title
		if title == "" {
			title = kf.Name
		}
		if title == "" {
			continue
		}
		if len(kf.ReleaseDate) >= 4 {
			parts = append(parts, title+" · "+kf.ReleaseDate[:4])
		} else {
			parts = append(parts, title)
		}
		break
	}
	return strings.Join(parts, " · ")
}

// trimAtSentence trims s to at most max bytes, preferring a sentence boundary.
func trimAtSentence(s string, max int) string {
	if len(s) <= max {
		return s
	}
	trimmed := s[:max]
	if i := strings.LastIndex(trimmed, ". "); i > max/2 {
		return s[:i+1]
	}
	return trimmed
}

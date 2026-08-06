package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var errNotFound = errors.New("not found")

// studioExternalIDsField is the reserved "_"-prefixed sidecar field-key that hands
// per production-company TMDB ids to Holodex studio-link de-dup (HOLODEX-122,
// ADR-054). It must match holodex/internal/model.StudioExternalIDsField — this
// provider is a standalone package, so the contract is the shared string literal.
const studioExternalIDsField = "_studio_external_ids"

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
	Provider        string               `json:"provider"`
	Version         string               `json:"version"`
	ProtocolVersion int                  `json:"protocol_version"`
	EntityTypes     []string             `json:"entity_types"`
	IDNamespaces    []string             `json:"id_namespaces"`
	Fields          []string             `json:"fields"`
	AssetKinds      []string             `json:"asset_kinds,omitempty"`
	FieldHints      map[string]fieldHint `json:"field_hints,omitempty"`
	// Credits advertises structured video credits (Holodex contract §4.5, F32): this
	// provider's video enrich responses include people[] alongside the flat
	// actors/director fields. Omit/false would mean flat-text-only.
	Credits bool `json:"credits,omitempty"`
	// BrandIcon advertises this provider's brand mark (Holodex contract §4.8, ADR-059):
	// a single provider-level image URL Holodex self-hosts and shows in place of the
	// repeated "from tmdb" provenance text. Env-configured (TMDB_BRAND_ICON_URL) and
	// omitted when unset, so a deployment supplies a raster on an allowlisted host
	// (TMDB's own logo is SVG, which Holodex's raster ingest rejects) rather than
	// shipping a brittle default. Additive — an older Holodex ignores it.
	BrandIcon *iconRef `json:"brand_icon,omitempty"`
}

// iconRef is a single provider-level image reference (the brand icon, §4.8).
type iconRef struct {
	URL string `json:"url"`
}

// fieldHint is a per-field presentation hint for a non-canonical advertised key
// (Holodex F39 contract §4.7): label / render mode / ordering group. Purely
// additive — a Holodex that predates the field ignores it.
type fieldHint struct {
	Label  string `json:"label,omitempty"`
	Render string `json:"render,omitempty"`
	Group  string `json:"group,omitempty"`
	Order  int    `json:"order,omitempty"`
}

type candidate struct {
	ExternalID     string  `json:"external_id"`
	Namespace      string  `json:"namespace"`
	Label          string  `json:"label"`
	Confidence     float64 `json:"confidence"`
	Disambiguation string  `json:"disambiguation,omitempty"`
	// ProfileURL is TMDB's own page for this match (F47, RD6/P1-1) — Holodex
	// scheme-validates it server-side before ever rendering it as a link, so this
	// sidecar just emits the real themoviedb.org URL.
	ProfileURL string `json:"profile_url,omitempty"`
}

type assetEntry struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

// personCredit is one entry in a video enrich response's structured `people[]`
// array (Holodex contract §4.5, F32) — a cast/crew member with a stable provider
// identity, alongside the flat `actors`/`director` text fields Holodex still
// consumes as a fallback. ExternalID is required (ADR-055): a credit whose TMDB
// person id is unknown is skipped by buildPeopleCredits rather than emitted id-less.
type personCredit struct {
	Name       string      `json:"name"`
	Role       string      `json:"role"`
	ExternalID string      `json:"external_id"`
	Order      int         `json:"order,omitempty"`
	Headshot   *assetEntry `json:"headshot,omitempty"`
}

type enrichResponse struct {
	Fields map[string][]string `json:"fields"`
	Assets []assetEntry        `json:"assets,omitempty"`
	People []personCredit      `json:"people,omitempty"`
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
	ID                 int      `json:"id"`
	Name               string   `json:"name"`
	Biography          string   `json:"biography"`
	Birthday           string   `json:"birthday"`
	Deathday           string   `json:"deathday"`
	PlaceOfBirth       string   `json:"place_of_birth"`
	ProfilePath        string   `json:"profile_path"`
	AlsoKnownAs        []string `json:"also_known_as"`
	KnownForDepartment string   `json:"known_for_department"`
}

type findResult struct {
	PersonResults []tmdbPerson       `json:"person_results"`
	MovieResults  []movieSearchEntry `json:"movie_results"`
}

// ---- movie types ----

type movieSearchResult struct {
	Results []movieSearchEntry `json:"results"`
}

type movieSearchEntry struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	ReleaseDate string  `json:"release_date"`
	Popularity  float64 `json:"popularity"`
}

type movieGenre struct {
	Name string `json:"name"`
}

type productionCompany struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type movieDetails struct {
	ID                  int                 `json:"id"`
	Title               string              `json:"title"`
	OriginalTitle       string              `json:"original_title"`
	Overview            string              `json:"overview"`
	ReleaseDate         string              `json:"release_date"`
	Runtime             int                 `json:"runtime"`
	Genres              []movieGenre        `json:"genres"`
	Tagline             string              `json:"tagline"`
	OriginalLanguage    string              `json:"original_language"`
	Status              string              `json:"status"`
	IMDbID              string              `json:"imdb_id"`
	PosterPath          string              `json:"poster_path"`
	ProductionCompanies []productionCompany `json:"production_companies"`
}

// movieCredits holds the cast and crew from /3/movie/{id}/credits.
type movieCredits struct {
	Cast []movieCastEntry `json:"cast"`
	Crew []movieCrewEntry `json:"crew"`
}

// personProfile is one entry from /3/person/{id}/images profiles[].
type personProfile struct {
	FilePath    string  `json:"file_path"`
	AspectRatio float64 `json:"aspect_ratio"`
	VoteAverage float64 `json:"vote_average"`
}

// personImagesResult is the response from /3/person/{id}/images.
type personImagesResult struct {
	Profiles []personProfile `json:"profiles"`
}

// taggedImageEntry is one item from /3/person/{id}/tagged_images results[].
// We only need the geometry to pick landscape-format backdrop images for the banner slot.
type taggedImageEntry struct {
	FilePath    string  `json:"file_path"`
	AspectRatio float64 `json:"aspect_ratio"`
}

// taggedImagesResult is the response from /3/person/{id}/tagged_images.
type taggedImagesResult struct {
	Results []taggedImageEntry `json:"results"`
}

type movieCastEntry struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Order       int    `json:"order"`
	ProfilePath string `json:"profile_path"`
}

type movieCrewEntry struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	ProfilePath string `json:"profile_path"`
}

// ---- resolve ----

func (c *tmdbClient) resolve(ctx context.Context, h hintBody, entityType string) ([]candidate, error) {
	switch entityType {
	case "video":
		return c.resolveMovie(ctx, h)
	case "studio":
		return c.resolveStudio(ctx, h)
	default:
		return c.resolvePerson(ctx, h)
	}
}

func (c *tmdbClient) resolvePerson(ctx context.Context, h hintBody) ([]candidate, error) {
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
	if h.Query == "" {
		return []candidate{}, nil
	}
	return c.searchPerson(ctx, h.Query)
}

// releaseFilenameRe matches release-group video filenames (e.g.
// "Dune.2021.2160p.WEB-DL...") so we can extract a clean title and year before
// sending to TMDB's search API. Requires 3+ dots in the query to avoid
// false-positives on ordinary search strings that happen to contain a year.
var releaseFilenameRe = regexp.MustCompile(`^(.+?)\.((?:19|20)\d{2})(?:\.|$)`)

// parseReleaseFilename extracts (title, year) from a dotted release-group
// filename. Returns (original, "") when the pattern is not recognised.
func parseReleaseFilename(q string) (title, year string) {
	if strings.Count(q, ".") < 3 {
		return q, ""
	}
	m := releaseFilenameRe.FindStringSubmatch(q)
	if m == nil {
		return q, ""
	}
	return strings.ReplaceAll(m[1], ".", " "), m[2]
}

func (c *tmdbClient) resolveMovie(ctx context.Context, h hintBody) ([]candidate, error) {
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
			det, err := c.fetchMovieDetails(ctx, n)
			if err != nil {
				return nil, err
			}
			return []candidate{{
				ExternalID:     fmt.Sprintf("tmdb:%d", det.ID),
				Namespace:      "tmdb",
				Label:          det.Title,
				Confidence:     1.0,
				Disambiguation: movieDisambiguate(det),
				ProfileURL:     tmdbMovieURL(det.ID, det.Title),
			}}, nil
		case "imdb":
			cands, err := c.findMovieByIMDB(ctx, val)
			if err != nil {
				return nil, err
			}
			if len(cands) > 0 {
				return cands, nil
			}
		}
	}
	if h.Query == "" {
		return []candidate{}, nil
	}
	title, year := parseReleaseFilename(h.Query)
	return c.searchMovie(ctx, title, year)
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
			ProfileURL:     tmdbPersonURL(p.ID, p.Name),
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
			ProfileURL:     tmdbPersonURL(p.ID, p.Name),
		})
	}
	return out, nil
}

func (c *tmdbClient) searchMovie(ctx context.Context, query, year string) ([]candidate, error) {
	var result movieSearchResult
	params := url.Values{
		"query":    {query},
		"language": {c.language},
		"page":     {"1"},
	}
	if year != "" {
		params.Set("primary_release_year", year)
	}
	if err := c.get(ctx, "/3/search/movie", params, &result); err != nil {
		return nil, err
	}
	out := make([]candidate, 0, len(result.Results))
	for i, m := range result.Results {
		if i >= 10 {
			break
		}
		out = append(out, candidate{
			ExternalID:     fmt.Sprintf("tmdb:%d", m.ID),
			Namespace:      "tmdb",
			Label:          m.Title,
			Confidence:     rankConfidence(i, m.Popularity),
			Disambiguation: movieYear(m.ReleaseDate),
			ProfileURL:     tmdbMovieURL(m.ID, m.Title),
		})
	}
	return out, nil
}

func (c *tmdbClient) findMovieByIMDB(ctx context.Context, imdbID string) ([]candidate, error) {
	var result findResult
	if err := c.get(ctx, "/3/find/"+url.PathEscape(imdbID), url.Values{
		"external_source": {"imdb_id"},
		"language":        {c.language},
	}, &result); err != nil {
		return nil, err
	}
	out := make([]candidate, 0, len(result.MovieResults))
	for _, m := range result.MovieResults {
		dis := ""
		if len(m.ReleaseDate) >= 4 {
			dis = m.ReleaseDate[:4]
		}
		out = append(out, candidate{
			ExternalID:     fmt.Sprintf("tmdb:%d", m.ID),
			Namespace:      "tmdb",
			Label:          m.Title,
			Confidence:     0.95,
			Disambiguation: dis,
			ProfileURL:     tmdbMovieURL(m.ID, m.Title),
		})
	}
	return out, nil
}

func (c *tmdbClient) fetchMovieDetails(ctx context.Context, id int) (movieDetails, error) {
	var det movieDetails
	err := c.get(ctx, fmt.Sprintf("/3/movie/%d", id), url.Values{
		"language": {c.language},
	}, &det)
	return det, err
}

func (c *tmdbClient) fetchMovieCredits(ctx context.Context, id int) (movieCredits, error) {
	var cred movieCredits
	err := c.get(ctx, fmt.Sprintf("/3/movie/%d/credits", id), url.Values{
		"language": {c.language},
	}, &cred)
	return cred, err
}

// fetchPersonImages returns all profile photos for a person from /3/person/{id}/images.
// TMDB returns them sorted by vote_average descending so the first is the best.
func (c *tmdbClient) fetchPersonImages(ctx context.Context, id int) (personImagesResult, error) {
	var res personImagesResult
	err := c.get(ctx, fmt.Sprintf("/3/person/%d/images", id), url.Values{}, &res)
	return res, err
}

// fetchTaggedImages returns backdrop-tagged images for a person from /3/person/{id}/tagged_images.
// These are movie/show images the person appears in; backdrops (aspect_ratio ≥ 1.5) serve as banners.
func (c *tmdbClient) fetchTaggedImages(ctx context.Context, id int) (taggedImagesResult, error) {
	var res taggedImagesResult
	err := c.get(ctx, fmt.Sprintf("/3/person/%d/tagged_images", id), url.Values{
		"language": {c.language},
		"page":     {"1"},
	}, &res)
	return res, err
}

func buildMovieEnrichResponse(det movieDetails, credits movieCredits) enrichResponse {
	fields := make(map[string][]string)
	if v := strings.TrimSpace(det.Title); v != "" {
		fields["title"] = []string{v}
	}
	if v := strings.TrimSpace(det.Overview); v != "" {
		fields["overview"] = []string{trimAtSentence(v, 4000)}
	}
	if det.ReleaseDate != "" {
		fields["release_date"] = []string{det.ReleaseDate}
	}
	if det.Runtime > 0 {
		fields["runtime"] = []string{strconv.Itoa(det.Runtime)}
	}
	if len(det.Genres) > 0 {
		genres := make([]string, 0, len(det.Genres))
		for _, g := range det.Genres {
			if g.Name != "" {
				genres = append(genres, g.Name)
			}
		}
		if len(genres) > 0 {
			fields["genres"] = genres
		}
	}
	if v := strings.TrimSpace(det.Tagline); v != "" {
		fields["tagline"] = []string{v}
	}
	// The "Website" link points to this movie's TMDB page, not det.Homepage (the
	// studio's official/marketing site — often short-lived or region-gated). TMDB is
	// the provider's own durable record and the more useful destination.
	fields["homepage"] = []string{tmdbMovieURL(det.ID, det.Title)}
	if det.OriginalLanguage != "" {
		fields["original_language"] = []string{det.OriginalLanguage}
	}
	if ot := strings.TrimSpace(det.OriginalTitle); ot != "" && ot != strings.TrimSpace(det.Title) {
		fields["original_title"] = []string{ot}
	}
	if det.Status != "" {
		fields["status"] = []string{det.Status}
	}
	if det.IMDbID != "" {
		fields["imdb_id"] = []string{det.IMDbID}
	}
	if det.PosterPath != "" {
		fields["poster_url"] = []string{"https://image.tmdb.org/t/p/original" + det.PosterPath}
	}
	// production_companies → studio (multi-valued), plus a self-describing sidecar
	// carrying each company's TMDB id for studio-entity de-dup (HOLODEX-122, ADR-054).
	// studioExternalIDsField is a "_"-prefixed internal key: the core persists it but
	// never displays or resolves it; RelinkVideoStudios parses it into a name→id map.
	// Each sidecar value is "tmdb:<id> <name>" — the id token has no space, so the name
	// is the unambiguous remainder (robust to the core's value reordering/curation).
	if len(det.ProductionCompanies) > 0 {
		studios := make([]string, 0, len(det.ProductionCompanies))
		companyIDs := make([]string, 0, len(det.ProductionCompanies))
		for _, pc := range det.ProductionCompanies {
			if pc.Name == "" {
				continue
			}
			studios = append(studios, pc.Name)
			if pc.ID > 0 {
				companyIDs = append(companyIDs, fmt.Sprintf("tmdb:%d %s", pc.ID, pc.Name))
			}
		}
		if len(studios) > 0 {
			fields["studio"] = studios
		}
		if len(companyIDs) > 0 {
			fields[studioExternalIDsField] = companyIDs
		}
	}
	// cast → actors (top 10 by billing order; TMDB returns them pre-sorted)
	actors := make([]string, 0, 10)
	for i, m := range credits.Cast {
		if i >= 10 {
			break
		}
		if m.Name != "" {
			actors = append(actors, m.Name)
		}
	}
	if len(actors) > 0 {
		fields["actors"] = actors
	}
	// crew → director (job == "Director")
	var directors []string
	for _, m := range credits.Crew {
		if m.Job == "Director" && m.Name != "" {
			directors = append(directors, m.Name)
		}
	}
	if len(directors) > 0 {
		fields["director"] = directors
	}
	return enrichResponse{Fields: fields, People: buildPeopleCredits(credits)}
}

// maxCastCredits caps the top-billed cast entries emitted in people[] (contract §4.5
// caps ~50 overall; F32's own spec keeps this slice tighter — "top ~20 billed +
// director/key crew").
const maxCastCredits = 20

// maxCrewCredits caps the director credits emitted in people[] — a co-director list
// is normally 1-2 entries, but nothing in the TMDB API guarantees that (an anthology
// film can credit a double-digit number of segment directors), and every other
// list-building loop in this file (maxCastCredits, maxPersonPhotos, the search-result
// caps) has an explicit local bound. This one gets the same discipline rather than
// resting on an assumption about typical data shape.
const maxCrewCredits = 10

// buildPeopleCredits builds the structured people[] credits (contract §4.5) from a
// movie's cast/crew: the top maxCastCredits billed actors, then up to maxCrewCredits
// director(s) — the same actor/director subset the flat fields above extract, with
// external_id + headshot attached. A cast/crew member with no TMDB person id is
// skipped (external_id is required, ADR-055) rather than emitted id-less.
func buildPeopleCredits(credits movieCredits) []personCredit {
	var people []personCredit
	for i, m := range credits.Cast {
		if i >= maxCastCredits {
			break
		}
		if m.Name == "" || m.ID == 0 {
			continue
		}
		people = append(people, personCredit{
			Name:       m.Name,
			Role:       "actor",
			ExternalID: fmt.Sprintf("tmdb:%d", m.ID),
			Order:      m.Order,
			Headshot:   headshotFor(m.ProfilePath),
		})
	}
	crewCount := 0
	for _, m := range credits.Crew {
		if crewCount >= maxCrewCredits {
			break
		}
		if m.Job != "Director" || m.Name == "" || m.ID == 0 {
			continue
		}
		people = append(people, personCredit{
			Name:       m.Name,
			Role:       "director",
			ExternalID: fmt.Sprintf("tmdb:%d", m.ID),
			Headshot:   headshotFor(m.ProfilePath),
		})
		crewCount++
	}
	return people
}

// headshotFor builds a people[] headshot asset from a TMDB profile_path, or nil when
// absent — the contract's headshot field is optional, omitted when there is none.
func headshotFor(profilePath string) *assetEntry {
	if profilePath == "" {
		return nil
	}
	return &assetEntry{Kind: "photo", URL: "https://image.tmdb.org/t/p/original" + profilePath}
}

// tmdbMovieURL builds the canonical TMDB web page URL for a movie. TMDB looks the
// movie up by numeric id and redirects to the canonical slug, so the trailing slug
// we derive from the title is cosmetic — a missing or approximate slug (e.g. from a
// non-Latin title that slugifies to empty) still resolves to the right page.
func tmdbMovieURL(id int, title string) string {
	return tmdbEntityURL("movie", id, title)
}

// tmdbPersonURL builds the canonical TMDB web page URL for a person — the people
// analogue of tmdbMovieURL (same id-resolves, slug-is-cosmetic behaviour).
func tmdbPersonURL(id int, name string) string {
	return tmdbEntityURL("person", id, name)
}

// tmdbEntityURL assembles https://www.themoviedb.org/{kind}/{id}-{slug}, omitting
// the slug when the name slugifies to empty (TMDB still resolves on the id alone).
func tmdbEntityURL(kind string, id int, name string) string {
	if s := slugify(name); s != "" {
		return fmt.Sprintf("https://www.themoviedb.org/%s/%d-%s", kind, id, s)
	}
	return fmt.Sprintf("https://www.themoviedb.org/%s/%d", kind, id)
}

// slugNonAlnum matches runs of non-slug characters, each collapsed to one hyphen.
var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// slugify folds Latin diacritics to ASCII (é→e, ñ→n, ü→u), lowercases, and reduces s
// to [a-z0-9] runs joined by single hyphens, approximating TMDB's URL slugs (which
// are cosmetic — see tmdbEntityURL). A non-Latin string (e.g. CJK) slugifies to "".
func slugify(s string) string {
	// transform.Chain is stateful, so build it per call rather than sharing one
	// transformer across the sidecar's concurrent request goroutines (data race).
	fold := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	if folded, _, err := transform.String(fold, s); err == nil {
		s = folded
	}
	return strings.Trim(slugNonAlnum.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

func movieYear(releaseDate string) string {
	if len(releaseDate) >= 4 {
		return releaseDate[:4]
	}
	return ""
}

func movieDisambiguate(det movieDetails) string {
	var parts []string
	if y := movieYear(det.ReleaseDate); y != "" {
		parts = append(parts, y)
	}
	for i, g := range det.Genres {
		if i >= 2 {
			break
		}
		if g.Name != "" {
			parts = append(parts, g.Name)
		}
	}
	return strings.Join(parts, " · ")
}

// ---- enrich ----

func (c *tmdbClient) enrich(ctx context.Context, externalID, entityType string) (enrichResponse, error) {
	switch entityType {
	case "video":
		return c.enrichMovie(ctx, externalID)
	case "studio":
		return c.enrichStudio(ctx, externalID)
	default:
		return c.enrichPerson(ctx, externalID)
	}
}

func (c *tmdbClient) enrichPerson(ctx context.Context, externalID string) (enrichResponse, error) {
	ns, val, ok := splitID(externalID)
	if !ok || ns != "tmdb" {
		return enrichResponse{}, fmt.Errorf("%w: %q", errNotFound, externalID)
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return enrichResponse{}, fmt.Errorf("%w: %q", errNotFound, externalID)
	}
	// Fetch details, profile images, and tagged backdrops concurrently.
	// Details are required; the image calls are best-effort (failure → empty result).
	type detResult struct {
		det personDetails
		err error
	}
	type imgResult struct {
		imgs personImagesResult
		err  error
	}
	type tagResult struct {
		tags taggedImagesResult
		err  error
	}
	detCh := make(chan detResult, 1)
	imgCh := make(chan imgResult, 1)
	tagCh := make(chan tagResult, 1)
	go func() {
		det, err := c.fetchDetails(ctx, n)
		detCh <- detResult{det, err}
	}()
	go func() {
		imgs, err := c.fetchPersonImages(ctx, n)
		imgCh <- imgResult{imgs, err}
	}()
	go func() {
		tags, err := c.fetchTaggedImages(ctx, n)
		tagCh <- tagResult{tags, err}
	}()
	dr := <-detCh
	ir := <-imgCh
	tr := <-tagCh
	if dr.err != nil {
		return enrichResponse{}, dr.err
	}
	// ir.err and tr.err are ignored (best-effort): the image/tagged-image calls are
	// optional, so a failure simply yields an empty result and no assets of that type.
	return buildEnrichResponse(dr.det, ir.imgs, tr.tags), nil
}

func (c *tmdbClient) enrichMovie(ctx context.Context, externalID string) (enrichResponse, error) {
	ns, val, ok := splitID(externalID)
	if !ok || ns != "tmdb" {
		return enrichResponse{}, fmt.Errorf("%w: %q", errNotFound, externalID)
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return enrichResponse{}, fmt.Errorf("%w: %q", errNotFound, externalID)
	}
	// Fetch movie details and credits concurrently to minimise round-trip latency.
	type detResult struct {
		det movieDetails
		err error
	}
	type credResult struct {
		cred movieCredits
		err  error
	}
	detCh := make(chan detResult, 1)
	credCh := make(chan credResult, 1)
	go func() {
		det, err := c.fetchMovieDetails(ctx, n)
		detCh <- detResult{det, err}
	}()
	go func() {
		cred, err := c.fetchMovieCredits(ctx, n)
		credCh <- credResult{cred, err}
	}()
	dr := <-detCh
	if dr.err != nil {
		return enrichResponse{}, dr.err
	}
	cr := <-credCh
	if cr.err != nil {
		return enrichResponse{}, cr.err
	}
	return buildMovieEnrichResponse(dr.det, cr.cred), nil
}

func (c *tmdbClient) fetchDetails(ctx context.Context, id int) (personDetails, error) {
	var det personDetails
	err := c.get(ctx, fmt.Sprintf("/3/person/%d", id), url.Values{
		"language": {c.language},
	}, &det)
	return det, err
}

// maxPersonPhotos caps the total number of image assets returned per person enrich.
const maxPersonPhotos = 20

func buildEnrichResponse(det personDetails, imgs personImagesResult, tags taggedImagesResult) enrichResponse {
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
	// The "Website" link points to this person's TMDB page, not det.Homepage (their
	// personal/agency site — often stale or absent). TMDB is the durable record.
	fields["website"] = []string{tmdbPersonURL(det.ID, det.Name)}
	var aliases []string
	for _, a := range det.AlsoKnownAs {
		if a = strings.TrimSpace(a); a != "" {
			aliases = append(aliases, a)
		}
	}
	if len(aliases) > 0 {
		fields["aliases"] = aliases
	}
	// Non-canonical field surfaced via the F39 hint advertised in /describe: TMDB's
	// primary department (e.g. "Acting", "Directing"). Holodex auto-registers it as a
	// display-only "Known for" row — no per-operator mapping needed.
	if dept := strings.TrimSpace(det.KnownForDepartment); dept != "" {
		fields["known_for_department"] = []string{dept}
	}

	// Build assets: profile photos + one banner backdrop (if available).
	//
	// Profile photos come from /3/person/{id}/images (sorted by vote_average desc).
	// The first becomes the headshot; additional ones become gallery items.
	// Fall back to profile_path from the details response when the images call failed
	// or returned nothing (e.g. person has exactly one photo and it matches profile_path).
	//
	// The banner comes from /3/person/{id}/tagged_images: the first landscape-format
	// result (aspect_ratio ≥ 1.5) maps to the banner slot. Best-effort — many people
	// have no tagged backdrops.
	profiles := imgs.Profiles
	if len(profiles) == 0 && det.ProfilePath != "" {
		profiles = []personProfile{{FilePath: det.ProfilePath}}
	}

	var assets []assetEntry
	for _, p := range profiles {
		if len(assets) >= maxPersonPhotos {
			break // cap reached — stop
		}
		if p.FilePath == "" {
			continue // skip a malformed entry but keep scanning for valid ones
		}
		// The first asset we actually keep is the headshot; the rest are gallery.
		kind := "gallery"
		if len(assets) == 0 {
			kind = "headshot"
		}
		assets = append(assets, assetEntry{
			Kind: kind,
			URL:  "https://image.tmdb.org/t/p/original" + p.FilePath,
		})
	}
	for _, t := range tags.Results {
		if len(assets) >= maxPersonPhotos {
			break
		}
		if t.FilePath != "" && t.AspectRatio >= 1.5 {
			assets = append(assets, assetEntry{
				Kind: "banner",
				URL:  "https://image.tmdb.org/t/p/original" + t.FilePath,
			})
			break // one banner slot
		}
	}

	return enrichResponse{Fields: fields, Assets: assets}
}

// ---- studio (company) ----

// companySearchResult is the response from /3/search/company.
type companySearchResult struct {
	Results []companyEntry `json:"results"`
}

// companyEntry is one production company from /3/search/company results[].
type companyEntry struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

// companyDetails is the response from /3/company/{id}.
type companyDetails struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Homepage      string `json:"homepage"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

func (c *tmdbClient) resolveStudio(ctx context.Context, h hintBody) ([]candidate, error) {
	// Embedded-ID path: fast and deterministic (a video's _studio_external_ids sidecar
	// hands the company's tmdb id straight through, ADR-054).
	for _, id := range h.ExternalIDs {
		ns, val, ok := splitID(id)
		if !ok || ns != "tmdb" {
			continue
		}
		n, err := strconv.Atoi(val)
		if err != nil {
			continue
		}
		det, err := c.fetchCompanyDetails(ctx, n)
		if err != nil {
			return nil, err
		}
		return []candidate{{
			ExternalID:     fmt.Sprintf("tmdb:%d", det.ID),
			Namespace:      "tmdb",
			Label:          det.Name,
			Confidence:     1.0,
			Disambiguation: det.OriginCountry,
			ProfileURL:     tmdbEntityURL("company", det.ID, det.Name),
		}}, nil
	}
	if h.Query == "" {
		return []candidate{}, nil
	}
	return c.searchCompany(ctx, h.Query)
}

// searchCompany matches production companies by name (/3/search/company). Company
// search takes no language param and returns no popularity, so confidence is purely
// rank-based; origin country is the disambiguation hint in the picker.
func (c *tmdbClient) searchCompany(ctx context.Context, query string) ([]candidate, error) {
	var result companySearchResult
	err := c.get(ctx, "/3/search/company", url.Values{
		"query": {query},
		"page":  {"1"},
	}, &result)
	if err != nil {
		return nil, err
	}
	out := make([]candidate, 0, len(result.Results))
	for i, co := range result.Results {
		if i >= 10 {
			break
		}
		out = append(out, candidate{
			ExternalID:     fmt.Sprintf("tmdb:%d", co.ID),
			Namespace:      "tmdb",
			Label:          co.Name,
			Confidence:     rankConfidence(i, 0),
			Disambiguation: co.OriginCountry,
			ProfileURL:     tmdbEntityURL("company", co.ID, co.Name),
		})
	}
	return out, nil
}

func (c *tmdbClient) enrichStudio(ctx context.Context, externalID string) (enrichResponse, error) {
	ns, val, ok := splitID(externalID)
	if !ok || ns != "tmdb" {
		return enrichResponse{}, fmt.Errorf("%w: %q", errNotFound, externalID)
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return enrichResponse{}, fmt.Errorf("%w: %q", errNotFound, externalID)
	}
	det, err := c.fetchCompanyDetails(ctx, n)
	if err != nil {
		return enrichResponse{}, err
	}
	return buildCompanyEnrichResponse(det), nil
}

func (c *tmdbClient) fetchCompanyDetails(ctx context.Context, id int) (companyDetails, error) {
	var det companyDetails
	err := c.get(ctx, fmt.Sprintf("/3/company/%d", id), url.Values{}, &det)
	return det, err
}

// buildCompanyEnrichResponse maps TMDB company details onto Holodex studio fields.
// The logo is a downloaded image asset (F51, ADR-079) — mirroring how a person's
// photo arrives — not a resolved image_url field.
func buildCompanyEnrichResponse(det companyDetails) enrichResponse {
	fields := make(map[string][]string)
	if v := strings.TrimSpace(det.Description); v != "" {
		fields["description"] = []string{trimAtSentence(v, 4000)}
	}
	if v := strings.TrimSpace(det.OriginCountry); v != "" {
		fields["country"] = []string{v}
	}
	// Prefer the company's official homepage; fall back to its durable TMDB page when
	// absent (mirrors the person/movie website behaviour — a link is always present).
	if v := strings.TrimSpace(det.Homepage); v != "" {
		fields["website"] = []string{v}
	} else {
		fields["website"] = []string{tmdbEntityURL("company", det.ID, det.Name)}
	}
	var assets []assetEntry
	if det.LogoPath != "" {
		assets = append(assets, assetEntry{
			Kind: "logo",
			URL:  "https://image.tmdb.org/t/p/original" + det.LogoPath,
		})
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

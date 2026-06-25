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
	ProfilePath  string   `json:"profile_path"`
	AlsoKnownAs  []string `json:"also_known_as"`
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
	Name  string `json:"name"`
	Order int    `json:"order"`
}

type movieCrewEntry struct {
	Name string `json:"name"`
	Job  string `json:"job"`
}

// ---- resolve ----

func (c *tmdbClient) resolve(ctx context.Context, h hintBody, entityType string) ([]candidate, error) {
	if entityType == "video" {
		return c.resolveMovie(ctx, h)
	}
	return c.resolvePerson(ctx, h)
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
	// production_companies → studio (multi-valued)
	if len(det.ProductionCompanies) > 0 {
		studios := make([]string, 0, len(det.ProductionCompanies))
		for _, pc := range det.ProductionCompanies {
			if pc.Name != "" {
				studios = append(studios, pc.Name)
			}
		}
		if len(studios) > 0 {
			fields["studio"] = studios
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
	return enrichResponse{Fields: fields}
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

// asciiFold decomposes accented Latin letters and drops the combining marks
// (é→e, ñ→n, ü→u) so slugs match TMDB's transliterated form rather than leaving a
// gap. Non-decomposable runes (e.g. CJK) pass through untouched and are dropped by
// slugify's [a-z0-9] filter.
var asciiFold = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// slugify folds Latin diacritics to ASCII, lowercases, and reduces s to [a-z0-9]
// runs joined by single hyphens, approximating TMDB's URL slugs (which are cosmetic
// — see tmdbEntityURL).
func slugify(s string) string {
	if folded, _, err := transform.String(asciiFold, s); err == nil {
		s = folded
	}
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		default:
			if b.Len() > 0 && !prevHyphen {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
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
	if entityType == "video" {
		return c.enrichMovie(ctx, externalID)
	}
	return c.enrichPerson(ctx, externalID)
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

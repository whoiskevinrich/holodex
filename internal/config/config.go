// Package config loads Holodex configuration with the precedence defined in
// ADR-014: CLI flags > environment variables > config file (holodex.yaml) >
// built-in defaults. This package handles defaults -> YAML -> env; CLI flag
// overlay is applied by the caller (cmd/holodex).
package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Paths
	MediaPath    string `yaml:"media_path"`
	DataPath     string `yaml:"data_path"`
	DatabasePath string `yaml:"database_path"`

	// Server
	Host     string `yaml:"host"` // bind address; empty = all interfaces (Docker default)
	Port     int    `yaml:"port"`
	LogLevel string `yaml:"log_level"`

	// AdminToken gates the owner-only admin surface (F21.7, ADR-030). Empty =
	// open (single-user, zero-config); set = the X-Admin-Token header is required
	// on /admin routes. Recommended whenever the server is reachable beyond
	// loopback.
	AdminToken string `yaml:"admin_token"`

	// SessionSecret optionally overrides the owner session-cookie signing key
	// (ADR-046). Empty = derive the key from AdminToken, so rotating the token
	// invalidates all sessions; set it to rotate sessions independently.
	SessionSecret string `yaml:"session_secret"`

	// Scanner (ADR-011, ADR-018)
	ScanIntervalSeconds int  `yaml:"scan_interval_seconds"`
	ScanWorkers         int  `yaml:"scan_workers"`
	FollowSymlinks      bool `yaml:"follow_symlinks"`
	ScanMaxDepth        int  `yaml:"scan_max_depth"`
	ScanMinAgeSeconds   int  `yaml:"scan_min_age_seconds"`

	// Soft-delete + purge (F24, ADR-037). A soft-deleted item lingers in Trash for
	// DeleteGracePeriodSeconds (0 disables auto-purge — manual only), then the purge
	// sweep (every PurgeIntervalSeconds) hard-deletes it; DeleteRemoveFiles controls
	// whether purge also unlinks the file (false = DB-only, for read-only mounts).
	DeleteGracePeriodSeconds int  `yaml:"delete_grace_period_seconds"`
	DeleteRemoveFiles        bool `yaml:"delete_remove_files"`
	PurgeIntervalSeconds     int  `yaml:"purge_interval_seconds"`

	// Thumbnails (ADR-009)
	ThumbnailEnabled     bool   `yaml:"thumbnail_enabled"`
	ThumbnailBackfill    string `yaml:"thumbnail_backfill"` // "eager" | "lazy"
	ThumbnailWorkers     int    `yaml:"thumbnail_workers"`
	ThumbnailNice        bool   `yaml:"thumbnail_nice"`
	ThumbnailSeekPercent int    `yaml:"thumbnail_seek_percent"`
	ThumbnailWidth       int    `yaml:"thumbnail_width"`
	ThumbnailPath        string `yaml:"-"` // derived: DataPath/thumbnails
	// PosterWidth caps the larger detail-page poster tier (F53, HOLODEX-253) —
	// a sibling derivative of the thumbnail, written alongside it at extraction
	// time. Default 1200 (3x ThumbnailWidth's 400 default).
	PosterWidth int `yaml:"poster_width"`

	// Person images (F25, ADR-038). Bytes live at DataPath/person-images/{personID}/
	// {id}.jpg; the path is derived like ThumbnailPath. The bounds guard untrusted
	// uploads: PersonImageMaxBytes caps the request body, PersonImageMaxDimension the
	// stored (downscaled) longest side.
	PersonImagePath         string `yaml:"-"` // derived: DataPath/person-images
	PersonImageMaxBytes     int64  `yaml:"person_image_max_bytes"`
	PersonImageMaxDimension int    `yaml:"person_image_max_dimension"`
	// PersonGalleryMax bounds the per-person 'extra' gallery (F25). Core roles
	// (headshot/banner/poster) are never counted against it. The owner can still
	// exceed it deliberately via an explicit over-cap upload; enrichment never does.
	PersonGalleryMax int `yaml:"person_gallery_max"`

	// Studio images (F51, ADR-079; generalizes HOLODEX-130/ADR-057's single logo cache
	// to icon/logo/poster). Bytes live at DataPath/studio-images/{studioID}/{id}.jpg,
	// derived like PersonImagePath. The bounds mirror the person ones.
	StudioImagePath         string `yaml:"-"` // derived: DataPath/studio-images
	StudioImageMaxBytes     int64  `yaml:"studio_image_max_bytes"`
	StudioImageMaxDimension int    `yaml:"studio_image_max_dimension"`

	// Film images (F56/HOLODEX-280, ADR-086; poster/thumb). Bytes live at
	// DataPath/film-images/{filmID}/{id}.jpg, derived like StudioImagePath. The
	// bounds mirror the studio ones.
	FilmImagePath         string `yaml:"-"` // derived: DataPath/film-images
	FilmImageMaxBytes     int64  `yaml:"film_image_max_bytes"`
	FilmImageMaxDimension int    `yaml:"film_image_max_dimension"`

	// Self-hosted provider brand icon (HOLODEX-134, ADR-059). One normalized icon per
	// provider at DataPath/provider-icons/{id}.jpg, derived like StudioImagePath.
	// ProviderIconMaxDimension bounds the stored (downscaled) longest side — icons are
	// tiny glyphs, so this is small.
	ProviderIconPath         string `yaml:"-"` // derived: DataPath/provider-icons
	ProviderIconMaxDimension int    `yaml:"provider_icon_max_dimension"`

	// Cache (ADR-008)
	CacheBackend     string `yaml:"cache_backend"`
	CacheMaxMemoryMB int    `yaml:"cache_max_memory_mb"`
	RedisURL         string `yaml:"redis_url"`

	// MCP (Phase 2, ADR-005)
	MCPEnabled   bool   `yaml:"mcp_enabled"`
	MCPTransport string `yaml:"mcp_transport"`
	MCPPort      int    `yaml:"mcp_port"`

	// Metadata field mapping (Phase 2, ADR-013). Path to metadata-mappings.yaml;
	// a missing file simply means no configurable fields.
	MetadataMappingsPath string `yaml:"metadata_mappings_path"`

	// Metadata source plugins (Phase 3, F22, ADR-033). Path to metadata-sources.yaml
	// declaring the sidecar providers; a missing file simply means no providers.
	MetadataSourcesPath string `yaml:"metadata_sources_path"`

	// CardLayout controls the aspect ratio used to display media cards in browse lists.
	// "wide" (default) shows 16:9 thumbnails, suited to personal/AMV libraries.
	// "poster" shows 2:3 cards, suited to film libraries with poster-format cover art.
	CardLayout string `yaml:"card_layout"`

	// FilmsEnabled gates the Films entity (F56, ADR-085): default false (opt-in),
	// matching MCPEnabled's precedent. Server-side and real, not cosmetic — /films
	// routes are unregistered, films are excluded from FTS search/MCP, and the
	// film resolver source (a video's decided "provider:film:<id>" field) is
	// suspended (not deregistered) while false, via injection non-participation
	// (ADR-085 §5) — never write-destructive. Migrations run regardless of this
	// flag; it gates behavior, not schema.
	FilmsEnabled bool `yaml:"films_enabled"`

	// DefaultSource governs the undecided per-field source of truth (F36, ADR-051).
	// "file" (default) makes the file/baseline layer win when no per-field decision
	// exists — a provider is a candidate, never an automatic winner (the F31
	// refresh-masking fix). "mapping" restores the legacy first-non-empty mapping
	// order for film-centric instances that want provider-first display.
	DefaultSource string `yaml:"default_source"`

	// ProviderTrustOrder ranks metadata providers for the *undecided* winner among
	// them on a replace field (F36 P1-2, ADR-051 §8). When several matched providers
	// supply a value and no per-field decision exists, the first-listed provider
	// wins — but the file/baseline layer still wins overall under default_source:
	// file (RD4), and a per-field decision always overrides. Unlisted providers keep
	// mapping order behind the listed ones; empty means today's mapping-order
	// fallback among providers. Env var PROVIDER_TRUST_ORDER is comma-separated.
	ProviderTrustOrder []string `yaml:"provider_trust_order"`

	// WritebackConcurrency bounds the durable batch-writeback queue's worker pool
	// (F30, ADR-048). Default 1 fully serializes file writes to protect the
	// filesystem; raise only on fast storage. Clamped to ≥1.
	WritebackConcurrency int `yaml:"writeback_concurrency"`

	// ExtractionAutoApplyEnabled gates whether a filename-extraction candidate
	// that clears F48.3/F48.4's confidence-and-exact-match gate actually
	// enqueues a write, or only logs what it would have done (F48, ADR-067
	// Action Item 2: "implement F48.3-F48.4 behind a feature flag; disable
	// auto-apply (log-only) until this ADR lands"). ADR-067 is Proposed, not
	// Accepted, as of Phase 2 — default false. ADR-060's generic runtime-settings
	// mechanism doesn't exist yet, so this is a plain boot-time env flag like
	// every other Config field, not a DB-backed owner setting.
	ExtractionAutoApplyEnabled bool `yaml:"extraction_auto_apply_enabled"`

	// FilenamePatternsPath is the owner-configurable, ordered filename
	// token-grammar pattern list (F48.1a, ADR-067). A missing file means no
	// patterns are configured — every file falls through to tag-only
	// resolution unchanged (F48.1b). Reloadable at runtime via the existing
	// POST /admin/reload-config (F48.1a's "editable at runtime" requirement),
	// mirroring MetadataMappingsPath.
	FilenamePatternsPath string `yaml:"filename_patterns_path"`
}

// Defaults returns the built-in configuration (the lowest-precedence layer).
func Defaults() Config {
	return Config{
		DataPath:            "./data",
		Port:                7800,
		LogLevel:            "info",
		ScanIntervalSeconds: 300,
		ScanWorkers:         4,
		FollowSymlinks:      true,
		ScanMaxDepth:        64,
		ScanMinAgeSeconds:   5,

		// F24/ADR-037 settled defaults: 7-day grace, auto-purge on, remove files.
		DeleteGracePeriodSeconds: 604800, // 7 days
		DeleteRemoveFiles:        true,   // "delete means delete"
		PurgeIntervalSeconds:     3600,   // hourly sweep

		ThumbnailEnabled:     true,
		ThumbnailBackfill:    "eager",
		ThumbnailWorkers:     2,
		ThumbnailNice:        true,
		ThumbnailSeekPercent: 10,
		ThumbnailWidth:       400,
		PosterWidth:          1200,

		PersonImageMaxBytes:     10 << 20, // 10 MiB request-body cap on an upload
		PersonImageMaxDimension: 2000,     // downscale stored images to ≤2000px longest side
		PersonGalleryMax:        20,       // per-person 'extra' gallery cap (F25)

		StudioImageMaxBytes:      10 << 20, // 10 MiB request-body cap on an upload
		StudioImageMaxDimension:  1000,     // studio images are small; downscale to ≤1000px longest side
		ProviderIconMaxDimension: 256,      // brand icons are tiny glyphs; downscale to ≤256px (ADR-059)

		FilmImageMaxBytes:     10 << 20, // 10 MiB request-body cap on an upload
		FilmImageMaxDimension: 1500,     // posters are portrait; downscale to ≤1500px longest side

		CacheBackend:         "memory",
		CacheMaxMemoryMB:     128,
		MCPTransport:         "http",
		MCPPort:              7801,
		MetadataMappingsPath: "./metadata-mappings.yaml",
		MetadataSourcesPath:  "./metadata-sources.yaml",
		FilenamePatternsPath: "./metadata-patterns.yaml",
		CardLayout:           "wide",
		DefaultSource:        "file", // F36/RD4: file beats providers when undecided
		WritebackConcurrency: 1,
	}
}

// Load builds a Config by layering, in increasing precedence:
// defaults -> YAML file (if path non-empty and exists) -> environment.
//
// A local .env (in the working directory) is loaded first as a convenience for
// development (ADR-027): its keys feed the environment layer so a checked-out
// dev config works without exporting vars or relying on the launcher's env
// handling. Real environment variables and CLI flags still win; in production
// (Docker) no .env ships, so deployment is unaffected.
func Load(yamlPath string) (Config, error) {
	loadDotenv(".env")
	cfg := Defaults()

	if yamlPath != "" {
		if data, err := os.ReadFile(yamlPath); err == nil {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return cfg, fmt.Errorf("parse config %s: %w", yamlPath, err)
			}
		} else if !os.IsNotExist(err) {
			return cfg, fmt.Errorf("read config %s: %w", yamlPath, err)
		}
	}

	applyEnv(&cfg)
	cfg.derive()
	return cfg, nil
}

// ExposedBind reports whether Host binds beyond loopback (reachable from other
// hosts). An empty Host means "all interfaces" (the Docker default) and is
// therefore exposed; an unparseable/DNS host is treated as exposed (fail-safe).
// Drives the F21.7 fail-loud warning when the admin surface has no token.
func (c Config) ExposedBind() bool {
	switch c.Host {
	case "localhost":
		return false
	case "":
		return true
	}
	if ip := net.ParseIP(c.Host); ip != nil {
		return !ip.IsLoopback()
	}
	return true
}

// derive fills computed defaults that depend on other fields.
func (c *Config) derive() {
	if c.DatabasePath == "" {
		c.DatabasePath = filepath.Join(c.DataPath, "holodex.db")
	}
	if c.ThumbnailPath == "" {
		c.ThumbnailPath = filepath.Join(c.DataPath, "thumbnails")
	}
	if c.PersonImagePath == "" {
		c.PersonImagePath = filepath.Join(c.DataPath, "person-images")
	}
	if c.StudioImagePath == "" {
		c.StudioImagePath = filepath.Join(c.DataPath, "studio-images")
	}
	if c.FilmImagePath == "" {
		c.FilmImagePath = filepath.Join(c.DataPath, "film-images")
	}
	if c.ProviderIconPath == "" {
		c.ProviderIconPath = filepath.Join(c.DataPath, "provider-icons")
	}
}

// Overrides carries CLI-flag values — the highest-precedence layer (ADR-014,
// F9.5: CLI > env > yaml > defaults). Zero/empty fields are left untouched, so
// the caller can pass only the flags the user actually set.
type Overrides struct {
	Host      string
	Port      int
	MediaPath string
	DataPath  string
	LogLevel  string
}

// ApplyOverrides overlays CLI flags on top of the loaded config and re-derives
// computed fields (e.g. DatabasePath when DataPath changes).
func (c *Config) ApplyOverrides(o Overrides) {
	if o.Host != "" {
		c.Host = o.Host
	}
	if o.Port != 0 {
		c.Port = o.Port
	}
	if o.MediaPath != "" {
		c.MediaPath = o.MediaPath
	}
	if o.DataPath != "" {
		c.DataPath = o.DataPath
		c.DatabasePath = "" // re-derive under the new data dir
		c.ThumbnailPath = ""
		c.PersonImagePath = ""
		c.StudioImagePath = ""
		c.FilmImagePath = ""
		c.ProviderIconPath = ""
	}
	if o.LogLevel != "" {
		c.LogLevel = o.LogLevel
	}
	c.derive()
}

func applyEnv(c *Config) {
	c.MediaPath = envStr("MEDIA_PATH", c.MediaPath)
	c.DataPath = envStr("DATA_PATH", c.DataPath)
	c.DatabasePath = envStr("DATABASE_PATH", c.DatabasePath)
	c.Host = envStr("HOST", c.Host)
	c.Port = envInt("PORT", c.Port)
	c.LogLevel = envStr("LOG_LEVEL", c.LogLevel)
	c.AdminToken = envStr("ADMIN_TOKEN", c.AdminToken)
	c.SessionSecret = envStr("SESSION_SECRET", c.SessionSecret)

	c.ScanIntervalSeconds = envInt("SCAN_INTERVAL_SECONDS", c.ScanIntervalSeconds)
	c.ScanWorkers = envInt("SCAN_WORKERS", c.ScanWorkers)
	c.FollowSymlinks = envBool("FOLLOW_SYMLINKS", c.FollowSymlinks)
	c.ScanMaxDepth = envInt("SCAN_MAX_DEPTH", c.ScanMaxDepth)
	c.ScanMinAgeSeconds = envInt("SCAN_MIN_AGE_SECONDS", c.ScanMinAgeSeconds)

	c.DeleteGracePeriodSeconds = envInt("DELETE_GRACE_PERIOD_SECONDS", c.DeleteGracePeriodSeconds)
	c.DeleteRemoveFiles = envBool("DELETE_REMOVE_FILES", c.DeleteRemoveFiles)
	c.PurgeIntervalSeconds = envInt("PURGE_INTERVAL_SECONDS", c.PurgeIntervalSeconds)

	c.ThumbnailEnabled = envBool("THUMBNAIL_ENABLED", c.ThumbnailEnabled)
	c.ThumbnailBackfill = envStr("THUMBNAIL_BACKFILL", c.ThumbnailBackfill)
	c.ThumbnailWorkers = envInt("THUMBNAIL_WORKERS", c.ThumbnailWorkers)
	c.ThumbnailNice = envBool("THUMBNAIL_NICE", c.ThumbnailNice)
	c.ThumbnailSeekPercent = envInt("THUMBNAIL_SEEK_PERCENT", c.ThumbnailSeekPercent)
	c.ThumbnailWidth = envInt("THUMBNAIL_WIDTH", c.ThumbnailWidth)
	c.PosterWidth = envInt("POSTER_WIDTH", c.PosterWidth)

	c.PersonImageMaxBytes = envInt64("PERSON_IMAGE_MAX_BYTES", c.PersonImageMaxBytes)
	c.PersonImageMaxDimension = envInt("PERSON_IMAGE_MAX_DIMENSION", c.PersonImageMaxDimension)
	c.PersonGalleryMax = envInt("PERSON_GALLERY_MAX", c.PersonGalleryMax)
	if c.PersonGalleryMax < 1 {
		c.PersonGalleryMax = 20 // a non-positive cap would block all gallery adds; fall back to the default
	}
	c.StudioImageMaxBytes = envInt64("STUDIO_IMAGE_MAX_BYTES", c.StudioImageMaxBytes)
	c.StudioImageMaxDimension = envInt("STUDIO_IMAGE_MAX_DIMENSION", c.StudioImageMaxDimension)
	c.FilmImageMaxBytes = envInt64("FILM_IMAGE_MAX_BYTES", c.FilmImageMaxBytes)
	c.FilmImageMaxDimension = envInt("FILM_IMAGE_MAX_DIMENSION", c.FilmImageMaxDimension)

	c.CacheBackend = envStr("CACHE_BACKEND", c.CacheBackend)
	c.CacheMaxMemoryMB = envInt("CACHE_MAX_MEMORY_MB", c.CacheMaxMemoryMB)
	c.RedisURL = envStr("REDIS_URL", c.RedisURL)

	c.MCPEnabled = envBool("MCP_ENABLED", c.MCPEnabled)
	c.MCPTransport = envStr("MCP_TRANSPORT", c.MCPTransport)
	c.MCPPort = envInt("MCP_PORT", c.MCPPort)

	c.MetadataMappingsPath = envStr("METADATA_MAPPINGS_PATH", c.MetadataMappingsPath)
	c.MetadataSourcesPath = envStr("METADATA_SOURCES_PATH", c.MetadataSourcesPath)
	c.WritebackConcurrency = envInt("WRITEBACK_CONCURRENCY", c.WritebackConcurrency)
	if c.WritebackConcurrency < 1 {
		c.WritebackConcurrency = 1 // a non-positive worker count would stall the queue
	}
	c.ExtractionAutoApplyEnabled = envBool("EXTRACTION_AUTO_APPLY_ENABLED", c.ExtractionAutoApplyEnabled)
	c.FilenamePatternsPath = envStr("FILENAME_PATTERNS_PATH", c.FilenamePatternsPath)
	c.FilmsEnabled = envBool("FILMS_ENABLED", c.FilmsEnabled)
	c.CardLayout = envStr("CARD_LAYOUT", c.CardLayout)
	if c.CardLayout != "wide" && c.CardLayout != "poster" {
		c.CardLayout = "wide"
	}
	c.DefaultSource = envStr("DEFAULT_SOURCE", c.DefaultSource)
	if c.DefaultSource != "file" && c.DefaultSource != "mapping" {
		c.DefaultSource = "file" // F36/RD4 default; reject typos rather than silently mapping-first
	}

	if v, ok := os.LookupEnv("PROVIDER_TRUST_ORDER"); ok {
		c.ProviderTrustOrder = strings.Split(v, ",")
	}
	// Normalize whether the list came from YAML or env: trim, lower-case, drop
	// blanks, and de-dup. Lower-casing is load-bearing — the resolver only ever
	// matches against lower-cased mapping namespaces (mapping.ParseSource), so a
	// raw "TMDB" would silently never rank; folding case here also makes the
	// de-dup case-insensitive.
	c.ProviderTrustOrder = normalizeList(c.ProviderTrustOrder)
}

// normalizeList trims each entry, lower-cases it, drops empties, and removes later
// duplicates, preserving first-seen order.
func normalizeList(in []string) []string {
	if len(in) == 0 {
		return in
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.ToLower(strings.TrimSpace(s)); s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// loadDotenv reads simple KEY=VALUE lines from path into the process environment
// for any key not already set (ADR-027). This lets a local .env supply dev config
// without exporting vars; real env vars and CLI flags still take precedence, and a
// missing/unreadable file is a silent no-op (so production, which ships no .env,
// is unaffected). Supports `#` comments, blank lines, an optional `export ` prefix,
// and one layer of surrounding single/double quotes.
func loadDotenv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export "))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			continue
		}
		val = strings.TrimSpace(val)
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

func envStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

// Provider-declared outbound link templates (HOLODEX-266, ADR-083 D2): a provider's
// /describe may advertise how a namespace-qualified external id becomes a clickable
// URL, so the provider-link badge (person/studio, video later) can link out without
// frontend-hardcoded per-provider knowledge (ADR-033 "declared, not compiled in").
package enrich

import (
	"net/url"
	"strings"
)

// linkTemplatePlaceholder is the one substitution token a link template may contain
// (ADR-083 D2) — deliberately just "{id}", not the {name}/{name?} vocabulary
// query.go's search patterns use: a link template renders one already-known
// namespace-qualified id, not a set of resolved video fields.
const linkTemplatePlaceholder = "{id}"

// maxLinkTemplateLen bounds an untrusted provider-supplied template string —
// mirrors maxHintLabelLen's "short display-adjacent string" cap tier, looser than
// maxFieldLen since a URL template is naturally longer than a label.
const maxLinkTemplateLen = 512

// ValidateLinkTemplate reports whether tmpl is a well-formed link template (ADR-083
// D2): an http(s) URL containing the "{id}" placeholder exactly once, with no other
// brace-delimited token. Exported so /describe ingest (persistLinkTemplates) shares
// one rule, mirroring ValidatePattern's role for search patterns. Not an SSRF gate —
// unlike base_url/asset_hosts, a link template is never dialed server-side; it is
// rendered as an outbound <a href> the visitor's browser navigates, the same posture
// as an already-sanitized candidate profile_url (sanitizeProfileURL).
func ValidateLinkTemplate(tmpl string) bool {
	if tmpl == "" || len(tmpl) > maxLinkTemplateLen {
		return false
	}
	// A single "{" that isn't the placeholder can't happen: strings.Count(tmpl, "{")
	// == 1 means whatever occupies that one brace either is "{id}" or isn't a
	// recognized token at all, so the Contains check alone settles it.
	if strings.Count(tmpl, "{") != 1 || !strings.Contains(tmpl, linkTemplatePlaceholder) {
		return false
	}
	return validHTTPURL(strings.Replace(tmpl, linkTemplatePlaceholder, "x", 1))
}

// SanitizeLinkTemplates coerces an untrusted /describe.link_templates map to a
// storable, render-safe set (ADR-083 D2): namespace and entity-kind keys are
// lower-cased/trimmed, and every template must pass ValidateLinkTemplate — an
// invalid entry is dropped rather than stored malformed. Returns nil when nothing
// survives.
func SanitizeLinkTemplates(in map[string]map[string]string) map[string]map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]map[string]string, len(in))
	for namespace, byKind := range in {
		ns := strings.ToLower(strings.TrimSpace(namespace))
		if ns == "" {
			continue
		}
		for kind, tmpl := range byKind {
			k := strings.ToLower(strings.TrimSpace(kind))
			tmpl = strings.TrimSpace(tmpl)
			if k == "" || !ValidateLinkTemplate(tmpl) {
				continue
			}
			if out[ns] == nil {
				out[ns] = map[string]string{}
			}
			out[ns][k] = tmpl
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// BuildLink renders an already-validated link template by substituting the "{id}"
// placeholder with a path-escaped id (ADR-083 D2): the id is provider-attested but
// still untrusted input, so it is escaped before interpolation into a served URL
// rather than substituted raw.
func BuildLink(tmpl, id string) string {
	return strings.ReplaceAll(tmpl, linkTemplatePlaceholder, url.PathEscape(id))
}

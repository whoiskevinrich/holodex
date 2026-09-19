package api

import (
	"context"
	"net/http"
)

// Instance skin (F66, ADR-102). The skin is instance identity: one server-held value,
// applied to every viewer via /capabilities, set by the owner through PUT /admin/theme.
// There is no viewer preference (ADR-021 §5 is superseded).

// themeSettingKey is the settings row holding the active skin id.
const themeSettingKey = "theme.active"

// ThemeDefault is the skin an instance shows until the owner picks one (spec RD10).
const ThemeDefault = "cinematheque"

// ThemeCustomID is the id of the owner's custom palette (spec R9), selectable only
// when one is configured.
const ThemeCustomID = "custom"

// shippedThemes are the skins built into app.css (ADR-021).
var shippedThemes = map[string]bool{"cinematheque": true, "broadcast": true, "brutalist": true}

// ThemeCustom is the owner's custom palette as configured in holodex.yaml
// `theme.custom` (spec R9): a base skin for fonts/radius/flourishes plus five hex
// primaries the SPA applies as inline custom properties on <html> (ADR-102 D4). Nil
// on Handlers until S3 wires config parsing.
type ThemeCustom struct {
	Name   string            `json:"name"`
	Base   string            `json:"base"`
	Tokens map[string]string `json:"tokens"`
}

// ThemePayload is /capabilities.theme — identical for every viewer.
type ThemePayload struct {
	Active string       `json:"active"`
	Custom *ThemeCustom `json:"custom"`
}

// themePayload resolves the effective skin (spec R3): the stored value, or the
// default when no row exists; a stored "custom" with no palette configured reports
// the default and leaves the row intact so restoring the config restores the choice.
// A read failure degrades to the default — capabilities must never fail over a skin.
func (h *Handlers) themePayload(ctx context.Context) ThemePayload {
	active := ThemeDefault
	if v, ok, err := h.repo.GetSetting(ctx, themeSettingKey); err != nil {
		h.log.Warn("read theme setting", "err", err)
	} else if ok {
		active = v
	}
	if active == ThemeCustomID && h.customTheme == nil {
		active = ThemeDefault
	}
	if !shippedThemes[active] && active != ThemeCustomID {
		active = ThemeDefault
	}
	return ThemePayload{Active: active, Custom: h.customTheme}
}

// adminSetTheme is PUT /admin/theme {"theme": id} (owner-only, spec R2). The id must
// be a shipped skin or "custom"; "custom" is refused while no palette is configured.
func (h *Handlers) adminSetTheme(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Theme string `json:"theme"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	switch {
	case shippedThemes[req.Theme]:
	case req.Theme == ThemeCustomID && h.customTheme != nil:
	case req.Theme == ThemeCustomID:
		writeError(w, http.StatusBadRequest, "custom skin is not configured (theme.custom in holodex.yaml)")
		return
	default:
		writeError(w, http.StatusBadRequest, "unknown theme")
		return
	}
	if err := h.repo.PutSetting(r.Context(), themeSettingKey, req.Theme); err != nil {
		h.fail(w, "set theme", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"theme": h.themePayload(r.Context())})
}

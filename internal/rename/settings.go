package rename

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"holodex/internal/settings"
)

// Setting keys (architecture "mode, settle window and templates are settings keys").
const (
	KeyMode      = "rename.mode"
	KeySettle    = "rename.settle_days"
	KeyTemplates = "rename.templates"
)

// Modes, in order: each includes the ones before it (P0-7).
const (
	ModeOff       = "off"
	ModePlan      = "plan"
	ModeManual    = "manual"
	ModeWriteback = "writeback"
	ModeJob       = "job"
)

var modeRank = map[string]int{ModeOff: 0, ModePlan: 1, ModeManual: 2, ModeWriteback: 3, ModeJob: 4}

// ModeAtLeast reports whether mode includes min.
func ModeAtLeast(mode, min string) bool { return modeRank[mode] >= modeRank[min] }

// TemplateListError refuses a template list, with one message per row ("" for a valid row).
type TemplateListError struct {
	Rows []string
}

func (e *TemplateListError) Error() string { return "the template list has errors" }

// DecodeTemplates reads the stored template list.
func DecodeTemplates(value string) ([]string, error) {
	var texts []string
	if err := json.Unmarshal([]byte(value), &texts); err != nil {
		return nil, fmt.Errorf("templates must be a JSON array of strings: %w", err)
	}
	return texts, nil
}

// ValidateTemplates validates a template list for storage. An empty list is allowed; it fits no
// video.
func ValidateTemplates(value string, providers []string) (string, error) {
	texts, err := DecodeTemplates(value)
	if err != nil {
		return "", err
	}
	if len(texts) == 0 {
		return "[]", nil
	}
	if list, errs := ParseList(texts, providers); list == nil {
		return "", ListError(errs)
	}
	for i := range texts {
		texts[i] = strings.TrimSpace(texts[i])
	}
	b, _ := json.Marshal(texts)
	return string(b), nil
}

// SettingKeys declares the rename keys on the settings allowlist. providers is read at each write,
// so a template naming a newly configured provider validates.
func SettingKeys(providers func() []string) []settings.Key {
	return []settings.Key{
		{Name: KeyMode, Default: ModeOff, Validate: func(v string) (string, error) {
			if _, ok := modeRank[v]; !ok {
				return "", errors.New("mode must be off, plan, manual, writeback or job")
			}
			return v, nil
		}},
		{Name: KeySettle, Default: "7", Validate: func(v string) (string, error) {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil || n < 1 || n > 365 {
				return "", errors.New("the settle window is 1 to 365 days")
			}
			return strconv.Itoa(n), nil
		}},
		{Name: KeyTemplates, Default: "[]", Validate: func(v string) (string, error) {
			return ValidateTemplates(v, providers())
		}},
	}
}

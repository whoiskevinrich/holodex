package rename

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"holodex/internal/settings"
)

// Setting keys (architecture "mode, settle window and templates are settings keys").
const (
	KeyMode       = "rename.mode"
	KeySettle     = "rename.settle_days"
	KeyTemplates  = "rename.templates"
	KeyCharacters = "rename.characters"
)

// MaxStandIn is the longest stand-in, in characters (P0-4).
const MaxStandIn = 8

// CharacterTableError refuses a character table, with one message per refused character.
type CharacterTableError struct {
	Rows map[string]string
}

func (e *CharacterTableError) Error() string { return "the character table has errors" }

// ParseCharacters checks an owner's character table (P0-4): each key is one visible symbol, each
// stand-in is short and holds nothing a file name can't, and every character a file name can't
// hold has a row.
func ParseCharacters(raw map[string]string) (Characters, error) {
	rows := map[string]string{}
	chars := make(Characters, len(raw))
	for k, v := range raw {
		r, msg := checkCharacter(k)
		if msg == "" {
			msg = checkStandIn(v)
		}
		if msg != "" {
			rows[k] = msg
			continue
		}
		chars[r] = norm.NFC.String(v)
	}
	for _, r := range forbidden {
		if _, ok := raw[string(r)]; !ok {
			rows[string(r)] = fmt.Sprintf("%c can't be kept, so it needs a row.", r)
		}
	}
	if len(rows) > 0 {
		return nil, &CharacterTableError{Rows: rows}
	}
	return chars, nil
}

func checkCharacter(k string) (rune, string) {
	r, size := utf8.DecodeRuneInString(k)
	switch {
	case k == "" || r == utf8.RuneError || size != len(k):
		return 0, "Add one character."
	case unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r):
		return 0, "Add a symbol, not a letter, digit or space."
	case r == '{' || r == '}':
		return 0, "Braces are always removed."
	case !unicode.IsPrint(r):
		return 0, "Add a visible character."
	}
	return r, ""
}

func checkStandIn(v string) string {
	if !utf8.ValidString(v) {
		return "A stand-in must be valid text."
	}
	if utf8.RuneCountInString(v) > MaxStandIn {
		return fmt.Sprintf("A stand-in is at most %d characters.", MaxStandIn)
	}
	for _, r := range v {
		if r == ' ' || !removedRune(r) {
			continue
		}
		if unicode.IsPrint(r) {
			return fmt.Sprintf("A stand-in can't contain %c.", r)
		}
		return "A stand-in can't contain an invisible or control character."
	}
	return ""
}

// DecodeCharacters reads a stored character table. A table that no longer validates is an error;
// the caller falls back to the default.
func DecodeCharacters(value string) (Characters, error) {
	var raw map[string]string
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return nil, fmt.Errorf("characters must be a JSON object of strings: %w", err)
	}
	return ParseCharacters(raw)
}

// EncodeCharacters is the stored form: a JSON object, keys sorted.
func EncodeCharacters(c Characters) string {
	raw := make(map[string]string, len(c))
	for r, s := range c {
		raw[string(r)] = s
	}
	b, _ := json.Marshal(raw)
	return string(b)
}

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
		{Name: KeyCharacters, Default: EncodeCharacters(defaultCharacters), Validate: func(v string) (string, error) {
			c, err := DecodeCharacters(v)
			if err != nil {
				return "", err
			}
			return EncodeCharacters(c), nil
		}},
	}
}

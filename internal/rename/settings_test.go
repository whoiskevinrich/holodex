package rename

import (
	"errors"
	"strings"
	"testing"
)

// HOLODEX-633: the owner's character table keeps every forbidden row, takes only visible symbols,
// and refuses a stand-in that is too long or holds something a file name can't.
func TestParseCharacters(t *testing.T) {
	base := func() map[string]string {
		m := map[string]string{}
		for r, s := range defaultCharacters {
			m[string(r)] = s
		}
		return m
	}
	if c, err := ParseCharacters(base()); err != nil || len(c) != len(defaultCharacters) {
		t.Fatalf("default table = %v, %v", c, err)
	}
	for name, c := range map[string]struct {
		key, standIn string
		drop         string // a row to delete first
	}{
		"letter":           {key: "a", standIn: "b"},
		"digit":            {key: "7", standIn: ""},
		"space":            {key: " ", standIn: ""},
		"two characters":   {key: "&&", standIn: ""},
		"brace":            {key: "{", standIn: ""},
		"control":          {key: "​", standIn: ""},
		"forbidden stand":  {key: "&", standIn: "a/b"},
		"brace stand":      {key: "&", standIn: "{x}"},
		"tab stand":        {key: "&", standIn: "a\tb"},
		"long stand":       {key: "&", standIn: strings.Repeat("x", MaxStandIn+1)},
		"kept forbidden":   {key: ":", standIn: ":"},
		"dropped required": {drop: "?"},
	} {
		t.Run(name, func(t *testing.T) {
			m := base()
			if c.drop != "" {
				delete(m, c.drop)
			} else {
				m[c.key] = c.standIn
			}
			var cte *CharacterTableError
			if _, err := ParseCharacters(m); !errors.As(err, &cte) || len(cte.Rows) != 1 {
				t.Fatalf("err = %v; want one refused row", err)
			}
		})
	}
	m := base()
	m["&"] = " and "
	m["@"] = strings.Repeat("x", MaxStandIn)
	delete(m, "!")
	delete(m, ";")
	if _, err := ParseCharacters(m); err != nil {
		t.Errorf("added rows and dropped optional ones: %v", err)
	}
}

func TestCharactersRoundTrip(t *testing.T) {
	c, err := DecodeCharacters(EncodeCharacters(defaultCharacters))
	if err != nil || len(c) != len(defaultCharacters) || c[':'] != "-" || c['?'] != "" {
		t.Errorf("round trip = %v, %v", c, err)
	}
	if _, err := DecodeCharacters(`["not", "an object"]`); err == nil {
		t.Error("a JSON array decoded")
	}
}

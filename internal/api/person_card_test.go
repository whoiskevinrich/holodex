package api_test

import (
	"net/http"
	"testing"
)

// F68 (HOLODEX-431) R5: the hover card's payload. Rides personDerivedServer so
// age comes from the same derived row the profile shows (clock pinned to
// derivedFixedNow), nationality from the resolver, aliases from the identity
// spine, and `completeness` only across the owner gate — absent, not null,
// for a visitor. Unknown ids are 404, like the profile.
func TestPersonCard(t *testing.T) {
	srv, pid := personDerivedServer(t, "secret", map[string][]string{
		"birthdate": {"1990-03-14"}, "nationality": {"Brazilian"},
	})
	base := srv.URL + "/api/v1/people/" + itoa(pid) + "/card"

	code, card := getJSONTok(t, base, "")
	if code != http.StatusOK {
		t.Fatalf("visitor card = %d", code)
	}
	if card["name"] != "Maya" || card["ref"] != "person:"+itoa(pid) {
		t.Errorf("identity = %v / %v, want Maya / person:%d", card["name"], card["ref"], pid)
	}
	// 2026-07-08 − 1990-03-14 = 36 whole years — the profile's Age row, not a client subtraction.
	if age, _ := card["age"].(float64); age != 36 {
		t.Errorf("age = %v, want 36", card["age"])
	}
	if _, has := card["age_at_death"]; has {
		t.Errorf("age_at_death present for a living person: %v", card["age_at_death"])
	}
	if nat, _ := card["nationality"].([]any); len(nat) != 1 || nat[0] != "Brazilian" {
		t.Errorf("nationality = %v, want [Brazilian]", card["nationality"])
	}
	if vc, _ := card["video_count"].(float64); vc != 1 {
		t.Errorf("video_count = %v, want 1", card["video_count"])
	}
	if _, has := card["completeness"]; has {
		t.Errorf("visitor card carries completeness %v, want absent", card["completeness"])
	}
	if _, has := card["display_name"]; has {
		t.Errorf("display_name present with no name decision: %v", card["display_name"])
	}

	code, owner := getJSONTok(t, base, "secret")
	if code != http.StatusOK {
		t.Fatalf("owner card = %d", code)
	}
	if c, ok := owner["completeness"].(map[string]any); !ok {
		t.Errorf("owner card completeness = %v, want the ring bands", owner["completeness"])
	} else if _, hasReq := c["required"]; !hasReq {
		t.Errorf("owner completeness lacks required: %v", c)
	}

	if code, _ := getJSONTok(t, srv.URL+"/api/v1/people/999999/card", ""); code != http.StatusNotFound {
		t.Errorf("unknown person card = %d, want 404", code)
	}
}

// A person with no birthdate and no aliases: the card simply omits those keys —
// the frontend drops the segments rather than rendering "—" (spec edge cases).
func TestPersonCard_SparseOmitsKeys(t *testing.T) {
	srv, pid := personDerivedServer(t, "", nil)
	code, card := getJSONTok(t, srv.URL+"/api/v1/people/"+itoa(pid)+"/card", "")
	if code != http.StatusOK {
		t.Fatalf("card = %d", code)
	}
	for _, k := range []string{"age", "age_at_death", "nationality", "aliases", "external_links", "display_name", "headshot_version"} {
		if v, has := card[k]; has {
			t.Errorf("sparse card carries %s = %v, want the key absent", k, v)
		}
	}
	if fc, _ := card["film_count"].(float64); fc != 0 {
		t.Errorf("film_count = %v, want 0 (films disabled)", card["film_count"])
	}
}

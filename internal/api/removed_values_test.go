package api_test

import (
	"strings"
	"testing"
)

// F30.3g / testing-strategy §34.2: removals ride the owner's detail read only.

// removedOf returns a field's removed values ("value" or "value!" when not sent now).
func removedOf(f map[string]any) []string {
	raw, _ := f["removed"].([]any)
	var out []string
	for _, r := range raw {
		m, _ := r.(map[string]any)
		v, _ := m["value"].(string)
		if ns, _ := m["not_sent"].(bool); ns {
			v += "!"
		}
		out = append(out, v)
	}
	return out
}

func fieldIn(body map[string]any, canonical string) (map[string]any, bool) {
	raw, _ := body["resolved"].([]any)
	for _, f := range raw {
		m, _ := f.(map[string]any)
		if m["canonical"] == canonical {
			return m, true
		}
	}
	return nil, false
}

func TestRemovedValues_PersonOwnerOnly(t *testing.T) {
	srv, _, pid := personDecisionServer(t, "s3cret")
	base := srv.URL + "/api/v1/people/" + itoa(pid)

	for _, v := range []string{"Ally", "Al"} {
		if code, _ := postTok(t, base+"/curation", "s3cret", map[string]string{"field": "nicknames", "value": v, "action": "suppress"}); code != 204 {
			t.Fatalf("suppress %s: want 204, got %d", v, code)
		}
	}

	_, owner := getTok(t, base, "s3cret")
	f, ok := fieldIn(owner, "nicknames")
	if !ok {
		t.Fatal("owner read dropped a field whose every value was removed")
	}
	if got := removedOf(f); strings.Join(got, ",") != "Al,Ally" {
		t.Errorf("owner removed = %v, want [Al Ally]", got)
	}

	_, visitor := getTok(t, base, "")
	if _, ok := fieldIn(visitor, "nicknames"); ok {
		t.Error("visitor read kept a fully removed field")
	}
	for _, raw := range visitor["resolved"].([]any) {
		if _, leaked := raw.(map[string]any)["removed"]; leaked {
			t.Fatalf("visitor read carries removals: %v", raw)
		}
	}

	// Restore round trip (F30.2e): the value is back with its source, the removal gone.
	if code, _ := postTok(t, base+"/curation/clear", "s3cret", map[string]string{"field": "nicknames", "value": "Al", "action": "suppress"}); code != 204 {
		t.Fatalf("clear: want 204, got %d", code)
	}
	_, owner = getTok(t, base, "s3cret")
	f, _ = fieldIn(owner, "nicknames")
	if got := removedOf(f); strings.Join(got, ",") != "Ally" {
		t.Errorf("after restore removed = %v, want [Ally]", got)
	}
	items, _ := f["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["value"] != "Al" {
		t.Errorf("after restore items = %v, want [Al]", items)
	}
}

// The genre writeback pass rebuilds the genres row; a row whose every genre was
// removed must survive it for the owner.
func TestRemovedValues_MediaGenresAllRemovedKept(t *testing.T) {
	srv, _, id := peopleDecisionServer(t)
	base := srv.URL + "/api/v1/media/" + itoa(id)
	for _, action := range []string{"add", "suppress"} {
		if code, _ := postTok(t, base+"/curation", "", map[string]string{"field": "genres", "value": "Noir", "action": action}); code != 204 {
			t.Fatalf("%s genre: want 204, got %d", action, code)
		}
	}
	_, body := getTok(t, base, "")
	f, ok := fieldIn(body, "genres")
	if !ok {
		t.Fatal("genres row dropped although it carries a removal")
	}
	if got := removedOf(f); strings.Join(got, ",") != "Noir" {
		t.Errorf("removed = %v, want [Noir]", got)
	}
}

func TestRemovedValues_MediaActorShownOnce(t *testing.T) {
	srv, _, id := peopleDecisionServer(t)
	base := srv.URL + "/api/v1/media/" + itoa(id)

	if code, _ := postTok(t, base+"/curation", "", map[string]string{"field": "actors", "value": "Jane Roe", "action": "add"}); code != 204 {
		t.Fatalf("add actor: want 204, got %d", code)
	}
	if code, _ := postTok(t, base+"/curation", "", map[string]string{"field": "actors", "value": "Jane Roe", "action": "suppress"}); code != 204 {
		t.Fatalf("remove actor: want 204, got %d", code)
	}
	_, body := getTok(t, base, "")
	f, ok := fieldIn(body, "actors")
	if !ok {
		t.Fatal("open backend is the owner: the fully removed actors field must stay")
	}
	// The manual add still holds the value, so lifting the removal brings it back.
	if got := removedOf(f); strings.Join(got, ",") != "Jane Roe" {
		t.Errorf("removed = %v, want [Jane Roe]", got)
	}
}

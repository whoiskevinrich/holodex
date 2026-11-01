package extract

import (
	"reflect"
	"testing"
)

// N18 (§38.2): a parsed value equal to the recorded one is dropped, whatever the current resolved
// value is; one that differs from the record stays, as do fields with no record.
func TestDropEchoes(t *testing.T) {
	fields := map[string][]string{
		"title":  {"Ada"},
		"studio": {"Other"},
		"people": {"Bo", "Al"},
		"part":   {"2"},
	}
	recorded := map[string][]string{
		"title":  {"Ada"},
		"studio": {"Acme"},
		"people": {"Al", "Bo"},
	}
	got := dropEchoes(fields, recorded)
	want := map[string][]string{"studio": {"Other"}, "part": {"2"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := dropEchoes(map[string][]string{"title": {"Ada"}}, nil); len(got) != 1 {
		t.Errorf("no record dropped a value: %v", got)
	}
}

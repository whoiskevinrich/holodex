package api

import (
	"reflect"
	"strings"
	"testing"

	"holodex/internal/model"
)

// N19 (§38.2): every identity marker in a name reaches its own provider as a hint, echo or not;
// a marker outside the id alphabet or over-long yields none, and the disambiguator is no marker.
func TestVideoHintIdentityMarkers(t *testing.T) {
	providers := []string{"provider-1", "provider-2"}
	cases := map[string][]string{
		"/m/Ada {provider-1-tt42} {provider-2-9}.mkv":             {"provider-1:tt42", "provider-2:9"},
		"/m/Ada {provider-1-42} {hx-7}.mkv":                       {"provider-1:42"},
		"/m/Ada {provider-1-../../x}.mkv":                         nil,
		"/m/Ada {provider-1-" + strings.Repeat("a", 65) + "}.mkv": nil,
	}
	for path, want := range cases {
		got := videoHint(&model.Video{FilePath: path}, nil, "", "", providers).ExternalIDs
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: hints = %v, want %v", path, got, want)
		}
	}
}

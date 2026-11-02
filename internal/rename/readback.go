package rename

import (
	"context"
	"path/filepath"
	"strings"

	"holodex/internal/repo"
)

// tokenField maps a template token to the field a pattern match keys it by. Identity tokens are
// hints, never filename values, so they have none.
var tokenField = map[string]string{TokTitle: "title", TokStudio: "studio", TokPeople: "people",
	TokYear: "release_date", TokEdition: "edition", TokPart: "part"}

func byField(tokens map[string][]string) map[string][]string {
	out := map[string][]string{}
	for tok, vals := range tokens {
		if f, ok := tokenField[tok]; ok && len(vals) > 0 {
			out[f] = vals
		}
	}
	return out
}

// Templates gives the current compiled list, the people delimiter and the providers.
type Templates func() (list []*Template, delimiter string, providers []string)

// NameReader is extract.NameReader over the journal and the current templates (P0-11).
type NameReader struct {
	Repo    *repo.Repo
	Current Templates
}

func stem(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// Recorded reads a name with the template that wrote it, then the current list; recorded is the
// value per field Holodex last wrote, the echo baseline.
func (n *NameReader) Recorded(ctx context.Context, videoID int64, path string) (fields, recorded map[string][]string, ok bool, err error) {
	last, err := n.Repo.LatestDoneRename(ctx, videoID)
	if err != nil || last == nil || last.Template == "" {
		return nil, nil, false, err
	}
	list, delim, providers := n.Current()
	if wrote, err := ParseTemplate(last.Template, providers); err == nil {
		list = append([]*Template{wrote}, list...)
	}
	vals, _, ok := ReadBack(list, stem(path), delim)
	if !ok {
		return nil, nil, false, nil
	}
	return byField(vals), byField(last.TokenValues), true, nil
}

// Templates reads a name none of the owner's patterns fits, so a rebuilt library still reads its
// Holodex-written names.
func (n *NameReader) Templates(path string) (map[string][]string, bool) {
	list, delim, _ := n.Current()
	vals, _, ok := ReadBack(list, stem(path), delim)
	if !ok {
		return nil, false
	}
	return byField(vals), true
}

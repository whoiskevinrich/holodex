package extract

import (
	"context"
	"testing"

	"holodex/internal/writequeue"
)

// fakeResolver is an in-memory Resolver for entity fields.
type fakeResolver struct {
	people  map[int64]string // id -> name, exact matches
	studios map[int64]string
}

func (f *fakeResolver) pool(entityType string) map[int64]string {
	switch entityType {
	case "person":
		return f.people
	case "studio":
		return f.studios
	default:
		return nil
	}
}

func (f *fakeResolver) ExactEntityMatch(_ context.Context, entityType, name string) (int64, bool, error) {
	for id, n := range f.pool(entityType) {
		if n == name {
			return id, true, nil
		}
	}
	return 0, false, nil
}

func (f *fakeResolver) EntityNames(_ context.Context, entityType string) (map[int64]string, error) {
	return f.pool(entityType), nil
}

type fakeManualSource struct{ manual map[string]bool } // key: field

func (f *fakeManualSource) HasManualSource(_ context.Context, _ string, _ int64, fieldKey string) (bool, error) {
	return f.manual[fieldKey], nil
}

type fakeReviewStore struct {
	upserts []ExtractionReviewCall
}

type ExtractionReviewCall struct {
	VideoID           int64
	FieldKey          string
	FilenameValue     string
	TagValue          string
	Confidence        float64
	SuggestedEntityID int64
}

func (f *fakeReviewStore) UpsertExtractionReview(_ context.Context, videoID int64, fieldKey, filenameValue, tagValue string, confidence float64, suggestedEntityID int64) error {
	f.upserts = append(f.upserts, ExtractionReviewCall{videoID, fieldKey, filenameValue, tagValue, confidence, suggestedEntityID})
	return nil
}

type fakeEnqueuer struct {
	calls []struct {
		VideoID int64
		Fields  []writequeue.JobField
	}
}

func (f *fakeEnqueuer) Enqueue(_ context.Context, videoID int64, fields []writequeue.JobField) (int64, error) {
	f.calls = append(f.calls, struct {
		VideoID int64
		Fields  []writequeue.JobField
	}{videoID, fields})
	return int64(len(f.calls)), nil
}

func TestProcess_Noop_NoFilenameData(t *testing.T) {
	out, err := Process(context.Background(), Deps{}, FieldExtraction{VideoID: 1, Field: "title"})
	if err != nil || out != OutcomeNoop {
		t.Fatalf("Process() = %v, %v; want OutcomeNoop, nil", out, err)
	}
}

func TestProcess_NonEntity_AutoAppliesWhenFlagEnabled(t *testing.T) {
	enq := &fakeEnqueuer{}
	d := Deps{Queue: enq, AutoApplyEnabled: true}
	fe := FieldExtraction{VideoID: 42, Field: "title", FilenameValues: []string{"The Great Movie"}}

	out, err := Process(context.Background(), d, fe)
	if err != nil || out != OutcomeAutoApplied {
		t.Fatalf("Process() = %v, %v; want OutcomeAutoApplied, nil", out, err)
	}
	if len(enq.calls) != 1 || enq.calls[0].VideoID != 42 {
		t.Fatalf("expected exactly one enqueue call for video 42, got %+v", enq.calls)
	}
	if enq.calls[0].Fields[0].Source != Provider {
		t.Fatalf("expected write source %q, got %q", Provider, enq.calls[0].Fields[0].Source)
	}
}

// TestProcess_LogOnly_WhenFlagDisabled is the ADR-067 Action Item 2 contract:
// a candidate that clears the auto-apply gate does NOT write while the flag
// is off (default, until the ADR is Accepted).
func TestProcess_LogOnly_WhenFlagDisabled(t *testing.T) {
	enq := &fakeEnqueuer{}
	reviews := &fakeReviewStore{}
	d := Deps{Queue: enq, Reviews: reviews, AutoApplyEnabled: false}
	fe := FieldExtraction{VideoID: 42, Field: "title", FilenameValues: []string{"The Great Movie"}}

	out, err := Process(context.Background(), d, fe)
	if err != nil || out != OutcomeLoggedOnly {
		t.Fatalf("Process() = %v, %v; want OutcomeLoggedOnly, nil", out, err)
	}
	if len(enq.calls) != 0 {
		t.Fatalf("log-only mode must never enqueue a write, got %+v", enq.calls)
	}
	if len(reviews.upserts) != 0 {
		t.Fatalf("a would-have-auto-applied candidate must not create a review row either, got %+v", reviews.upserts)
	}
}

func TestProcess_ManualOverride_AlwaysQueues(t *testing.T) {
	reviews := &fakeReviewStore{}
	enq := &fakeEnqueuer{}
	d := Deps{
		ManualSource:     &fakeManualSource{manual: map[string]bool{"title": true}},
		Reviews:          reviews,
		Queue:            enq,
		AutoApplyEnabled: true,
	}
	fe := FieldExtraction{VideoID: 7, Field: "title", FilenameValues: []string{"Perfect Match"}, TagValues: []string{"Perfect Match"}}

	out, err := Process(context.Background(), d, fe)
	if err != nil || out != OutcomeQueued {
		t.Fatalf("Process() = %v, %v; want OutcomeQueued, nil", out, err)
	}
	if len(enq.calls) != 0 {
		t.Fatal("a manual override must never auto-apply, even at flag=on and perfect agreement")
	}
	if len(reviews.upserts) != 1 || reviews.upserts[0].FieldKey != "title" {
		t.Fatalf("expected one review row for title, got %+v", reviews.upserts)
	}
}

func TestProcess_EntityField_ExactMatchAutoApplies(t *testing.T) {
	resolver := &fakeResolver{people: map[int64]string{1: "Alice Smith"}}
	enq := &fakeEnqueuer{}
	d := Deps{Resolver: resolver, Queue: enq, AutoApplyEnabled: true}
	fe := FieldExtraction{VideoID: 9, Field: "people", FilenameValues: []string{"Alice Smith"}}

	out, err := Process(context.Background(), d, fe)
	if err != nil || out != OutcomeAutoApplied {
		t.Fatalf("Process() = %v, %v; want OutcomeAutoApplied, nil", out, err)
	}
	if len(enq.calls) != 1 {
		t.Fatalf("expected one enqueue call, got %+v", enq.calls)
	}
	// The extract package's "people" field key must be translated to the
	// writeback layer's "actors" before it reaches the JobField — the
	// writeback formatMap has no "people" entry (see WritebackField).
	if got := enq.calls[0].Fields[0].Field; got != "actors" {
		t.Fatalf("JobField.Field = %q, want %q (writeback vocabulary)", got, "actors")
	}
}

func TestWritebackField(t *testing.T) {
	cases := map[string]string{
		"people": "actors",
		"title":  "title",
		"studio": "studio",
	}
	for field, want := range cases {
		if got := WritebackField(field); got != want {
			t.Errorf("WritebackField(%q) = %q, want %q", field, got, want)
		}
	}
}

// TestProcess_EntityField_FuzzyMatchQueuesWithSuggestion is F48.3d end to
// end: a close-but-not-exact name never auto-applies, and the review row
// carries the Jaro-Winkler-ranked suggestion.
func TestProcess_EntityField_FuzzyMatchQueuesWithSuggestion(t *testing.T) {
	resolver := &fakeResolver{people: map[int64]string{1: "Alice Smith"}}
	reviews := &fakeReviewStore{}
	enq := &fakeEnqueuer{}
	d := Deps{Resolver: resolver, Reviews: reviews, Queue: enq, AutoApplyEnabled: true}
	fe := FieldExtraction{VideoID: 9, Field: "people", FilenameValues: []string{"Alise Smith"}}

	out, err := Process(context.Background(), d, fe)
	if err != nil || out != OutcomeQueued {
		t.Fatalf("Process() = %v, %v; want OutcomeQueued, nil", out, err)
	}
	if len(enq.calls) != 0 {
		t.Fatal("a fuzzy entity match must never auto-apply")
	}
	if len(reviews.upserts) != 1 || reviews.upserts[0].SuggestedEntityID != 1 {
		t.Fatalf("expected a review row suggesting entity 1, got %+v", reviews.upserts)
	}
}

func TestProcess_EntityField_NoMatchQueuesWithoutSuggestion(t *testing.T) {
	resolver := &fakeResolver{people: map[int64]string{1: "Bob Jones"}}
	reviews := &fakeReviewStore{}
	d := Deps{Resolver: resolver, Reviews: reviews, AutoApplyEnabled: true}
	fe := FieldExtraction{VideoID: 9, Field: "people", FilenameValues: []string{"Someone Totally Different"}}

	out, err := Process(context.Background(), d, fe)
	if err != nil || out != OutcomeQueued {
		t.Fatalf("Process() = %v, %v; want OutcomeQueued, nil", out, err)
	}
	if len(reviews.upserts) != 1 || reviews.upserts[0].SuggestedEntityID != 0 {
		t.Fatalf("expected a review row with no suggestion, got %+v", reviews.upserts)
	}
}

func TestProcess_MultiValueEntityField_WeakestLinkGate(t *testing.T) {
	// Alice resolves exactly; Bob doesn't exist at all — the field as a whole
	// must not auto-apply even though one of its two values is a perfect hit.
	resolver := &fakeResolver{people: map[int64]string{1: "Alice Smith"}}
	reviews := &fakeReviewStore{}
	enq := &fakeEnqueuer{}
	d := Deps{Resolver: resolver, Reviews: reviews, Queue: enq, AutoApplyEnabled: true}
	fe := FieldExtraction{VideoID: 9, Field: "people", FilenameValues: []string{"Alice Smith", "Bob Someone New"}}

	out, err := Process(context.Background(), d, fe)
	if err != nil || out != OutcomeQueued {
		t.Fatalf("Process() = %v, %v; want OutcomeQueued, nil", out, err)
	}
	if len(enq.calls) != 0 {
		t.Fatal("a multi-value entity field with any unresolved value must not auto-apply")
	}
}

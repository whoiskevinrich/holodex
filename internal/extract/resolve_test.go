package extract

import "testing"

func TestResolveReviewAction_Filename(t *testing.T) {
	write, err := ResolveReviewAction(ReviewActionFilename, "people", "Alice, Bob", "Someone Else", "")
	if err != nil {
		t.Fatalf("ResolveReviewAction: %v", err)
	}
	if write.Source != Provider {
		t.Fatalf("Source = %q, want %q", write.Source, Provider)
	}
	if len(write.Values) != 2 || write.Values[0] != "Alice" || write.Values[1] != "Bob" {
		t.Fatalf("Values = %v, want [Alice Bob] (split on the joinSorted delimiter)", write.Values)
	}
}

func TestResolveReviewAction_FilenameRequiresValue(t *testing.T) {
	if _, err := ResolveReviewAction(ReviewActionFilename, "title", "", "Tag Value", ""); err == nil {
		t.Fatal("want error when filenameValue is empty")
	}
}

func TestResolveReviewAction_TagWritesNothing(t *testing.T) {
	write, err := ResolveReviewAction(ReviewActionTag, "title", "Filename Value", "Tag Value", "")
	if err != nil {
		t.Fatalf("ResolveReviewAction: %v", err)
	}
	if len(write.Values) != 0 {
		t.Fatalf("Values = %v, want empty (accepting the tag writes nothing)", write.Values)
	}
}

func TestResolveReviewAction_TagRequiresValue(t *testing.T) {
	if _, err := ResolveReviewAction(ReviewActionTag, "title", "Filename Value", "", ""); err == nil {
		t.Fatal("want error when tagValue is empty")
	}
}

func TestResolveReviewAction_Manual(t *testing.T) {
	write, err := ResolveReviewAction(ReviewActionManual, "title", "Filename Value", "Tag Value", "  Custom Title  ")
	if err != nil {
		t.Fatalf("ResolveReviewAction: %v", err)
	}
	if write.Source != "manual" {
		t.Fatalf("Source = %q, want manual", write.Source)
	}
	if len(write.Values) != 1 || write.Values[0] != "Custom Title" {
		t.Fatalf("Values = %v, want [Custom Title] (trimmed)", write.Values)
	}
}

// A single-value field must NOT split on ", " — a title that legitimately
// contains a comma stays one value.
func TestResolveReviewAction_ManualScalarKeepsComma(t *testing.T) {
	write, err := ResolveReviewAction(ReviewActionManual, "title", "", "", "Lock, Stock")
	if err != nil {
		t.Fatalf("ResolveReviewAction: %v", err)
	}
	if len(write.Values) != 1 || write.Values[0] != "Lock, Stock" {
		t.Fatalf("Values = %v, want [Lock, Stock] (scalar field, no split)", write.Values)
	}
}

// The multi-value People field splits the owner's edited cast so editing one
// person can't collapse the whole cast to a single value (HOLODEX-196 #1).
func TestResolveReviewAction_ManualMultiValue(t *testing.T) {
	write, err := ResolveReviewAction(ReviewActionManual, "people", "", "", "Alice, Bob, Carol")
	if err != nil {
		t.Fatalf("ResolveReviewAction: %v", err)
	}
	if write.Source != "manual" {
		t.Fatalf("Source = %q, want manual", write.Source)
	}
	if len(write.Values) != 3 || write.Values[0] != "Alice" || write.Values[2] != "Carol" {
		t.Fatalf("Values = %v, want [Alice Bob Carol]", write.Values)
	}
}

func TestResolveReviewAction_ManualRequiresValue(t *testing.T) {
	if _, err := ResolveReviewAction(ReviewActionManual, "title", "Filename Value", "Tag Value", "   "); err == nil {
		t.Fatal("want error when manualValue is blank")
	}
}

func TestResolveReviewAction_UnknownAction(t *testing.T) {
	if _, err := ResolveReviewAction(ReviewAction("bogus"), "title", "Filename Value", "Tag Value", ""); err == nil {
		t.Fatal("want error for an unknown action")
	}
}

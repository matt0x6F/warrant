package gitnotes

import (
	"testing"
)

func TestRefForType(t *testing.T) {
	tests := []struct {
		noteType string
		want    string
	}{
		{TypeDecision, RefPrefix + "/" + TypeDecision},
		{TypeTrace, RefPrefix + "/" + TypeTrace},
		{TypeIntent, RefPrefix + "/" + TypeIntent},
		{"", ""},
		{"invalid", ""},
		{"submission", ""},
	}
	for _, tt := range tests {
		got := RefForType(tt.noteType)
		if got != tt.want {
			t.Errorf("RefForType(%q) = %q, want %q", tt.noteType, got, tt.want)
		}
	}
}

func TestAllRefs(t *testing.T) {
	refs := AllRefs()
	if len(refs) != 3 {
		t.Fatalf("AllRefs() len = %d, want 3", len(refs))
	}
	want := map[string]bool{
		RefPrefix + "/" + TypeDecision: true,
		RefPrefix + "/" + TypeTrace:    true,
		RefPrefix + "/" + TypeIntent:   true,
	}
	for _, r := range refs {
		if !want[r] {
			t.Errorf("AllRefs() unexpected ref %q", r)
		}
	}
}

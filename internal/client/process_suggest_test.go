package client

import "testing"

func TestSuggestMatchPatternNodeFromTruncatedName(t *testing.T) {
	c := ProcessCandidate{
		Name:    "node /home/kris",
		Cmdline: "node /home/kris/caltrainDiscord/index.js",
	}

	got := SuggestMatchPattern(c)
	want := "node /home/kris/caltrainDiscord/index.js"
	if got != want {
		t.Fatalf("match pattern mismatch: got %q want %q", got, want)
	}
}

func TestSuggestFriendlyNameNodeFromTruncatedName(t *testing.T) {
	c := ProcessCandidate{
		Name:    "node /home/kris",
		Cmdline: "node /home/kris/caltrainDiscord/index.js",
	}

	got := SuggestFriendlyName(c)
	want := "index"
	if got != want {
		t.Fatalf("friendly name mismatch: got %q want %q", got, want)
	}
}

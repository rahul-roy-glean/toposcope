package main

import (
	"testing"
)

func TestSnapshotCmdFlags(t *testing.T) {
	cmd := newSnapshotCmd()

	// Test default values
	f := cmd.Flags()
	scope, _ := f.GetString("scope")
	if scope != "FULL" {
		t.Errorf("default scope = %q, want FULL", scope)
	}

	// Test that flags exist
	for _, flag := range []string{"repo-path", "scope", "output", "bazel-path", "bazelrc", "cquery"} {
		if f.Lookup(flag) == nil {
			t.Errorf("missing flag: %s", flag)
		}
	}
}

func TestDiffCmdFlags(t *testing.T) {
	cmd := newDiffCmd()
	f := cmd.Flags()

	// Test default head value
	head, _ := f.GetString("head")
	if head != "HEAD" {
		t.Errorf("default head = %q, want HEAD", head)
	}

	// Test that base is required
	for _, flag := range []string{"base", "head", "repo-path", "bazel-path", "bazelrc", "cquery"} {
		if f.Lookup(flag) == nil {
			t.Errorf("missing flag: %s", flag)
		}
	}
}

func TestScoreCmdFlags(t *testing.T) {
	cmd := newScoreCmd()
	f := cmd.Flags()

	// Test default output format
	outputFmt, _ := f.GetString("output")
	if outputFmt != "text" {
		t.Errorf("default output = %q, want text", outputFmt)
	}

	for _, flag := range []string{"base", "head", "repo-path", "bazel-path", "bazelrc", "cquery", "output"} {
		if f.Lookup(flag) == nil {
			t.Errorf("missing flag: %s", flag)
		}
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"a", "b", "c"}, "a"},
		{[]string{"", "b", "c"}, "b"},
		{[]string{"", "", "c"}, "c"},
		{[]string{"", "", ""}, ""},
	}

	for _, tt := range tests {
		got := firstNonEmpty(tt.args...)
		if got != tt.want {
			t.Errorf("firstNonEmpty(%v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestMinInt(t *testing.T) {
	if minInt(3, 5) != 3 {
		t.Error("minInt(3, 5) should be 3")
	}
	if minInt(5, 3) != 3 {
		t.Error("minInt(5, 3) should be 3")
	}
	if minInt(3, 3) != 3 {
		t.Error("minInt(3, 3) should be 3")
	}
}

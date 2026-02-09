package subgraph

import (
	"testing"
	"time"
)

func TestNormalizeLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"//app/foo:lib", "//app/foo:lib"},
		{"//app/foo:foo", "//app/foo"},
		{"@//app/foo:lib", "//app/foo:lib"},
		{"@//app/foo:foo", "//app/foo"},
		{"//app/foo", "//app/foo"},
		{"//lib/bar:bar", "//lib/bar"},
		{"  //app/foo:lib  ", "//app/foo:lib"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeLabel(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLabelToPackage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"//app/foo:lib", "//app/foo"},
		{"//app/foo", "//app/foo"},
		{"//lib/bar:bar", "//lib/bar"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := labelToPackage(tt.input)
			if got != tt.want {
				t.Errorf("labelToPackage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestChunkTargets(t *testing.T) {
	// All targets fit in one chunk
	targets := []string{"//a:a", "//b:b", "//c:c"}
	chunks := chunkTargets(targets, 1000)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}

	// Force split with small maxLen
	chunks = chunkTargets(targets, 10)
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Empty targets
	chunks = chunkTargets(nil, 1000)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for nil, got %d", len(chunks))
	}
}

func TestBuildRdepsQuery(t *testing.T) {
	targets := []string{"//app/foo:lib", "//lib/bar:bar"}
	query := buildRdepsQuery(targets, 2)
	expected := "rdeps(//..., set(//app/foo:lib //lib/bar:bar), 2)"
	if query != expected {
		t.Errorf("got %q, want %q", query, expected)
	}

	// Empty targets
	query = buildRdepsQuery(nil, 2)
	if query != "//..." {
		t.Errorf("got %q, want //...", query)
	}
}

func TestParseXML(t *testing.T) {
	xmlData := []byte(`<query version="2">
  <rule class="go_library" name="//app/foo:lib">
    <list name="deps">
      <label value="//lib/bar:bar"/>
      <label value="//lib/baz:baz"/>
    </list>
    <list name="tags">
      <string value="team:platform"/>
    </list>
    <list name="visibility">
      <label value="//visibility:public"/>
    </list>
  </rule>
  <rule class="go_test" name="//app/foo:lib_test">
    <list name="deps">
      <label value="//app/foo:lib"/>
    </list>
  </rule>
</query>`)

	rules, err := parseXML(xmlData)
	if err != nil {
		t.Fatalf("parseXML: %v", err)
	}

	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}

	if rules[0].Class != "go_library" {
		t.Errorf("rule[0].Class = %q, want go_library", rules[0].Class)
	}
	if rules[0].Name != "//app/foo:lib" {
		t.Errorf("rule[0].Name = %q, want //app/foo:lib", rules[0].Name)
	}

	// Check deps
	var depsFound int
	for _, list := range rules[0].Lists {
		if list.Name == "deps" {
			depsFound = len(list.Labels)
		}
	}
	if depsFound != 2 {
		t.Errorf("got %d deps, want 2", depsFound)
	}

	// Check tags extraction
	tags := extractTags(rules[0])
	if len(tags) != 1 || tags[0] != "team:platform" {
		t.Errorf("tags = %v, want [team:platform]", tags)
	}

	// Check visibility extraction
	vis := extractVisibility(rules[0])
	if len(vis) != 1 || vis[0] != "//visibility:public" {
		t.Errorf("visibility = %v, want [//visibility:public]", vis)
	}

	// Check test detection
	if isTestRule(rules[0].Class) {
		t.Error("go_library should not be a test rule")
	}
	if !isTestRule(rules[1].Class) {
		t.Error("go_test should be a test rule")
	}
}

func TestBuildSnapshot(t *testing.T) {
	rules := []xmlRule{
		{
			Class: "go_library",
			Name:  "//app/foo:lib",
			Lists: []xmlList{
				{
					Name:   "deps",
					Labels: []xmlLabelValue{{Value: "//lib/bar:bar"}},
				},
				{
					Name: "tags",
					Strs: []xmlStrValue{{Value: "team:platform"}},
				},
				{
					Name:   "visibility",
					Labels: []xmlLabelValue{{Value: "//visibility:public"}},
				},
			},
		},
		{
			Class: "go_test",
			Name:  "//app/foo:lib_test",
			Lists: []xmlList{
				{
					Name:   "deps",
					Labels: []xmlLabelValue{{Value: "//app/foo:lib"}},
				},
			},
		},
	}

	snap := buildSnapshot(rules, "abc123", []string{"//app/foo:lib"}, time.Now())
	if snap.CommitSHA != "abc123" {
		t.Errorf("CommitSHA = %q, want abc123", snap.CommitSHA)
	}
	if !snap.Partial {
		t.Error("expected Partial to be true")
	}
	if len(snap.Nodes) != 2 {
		t.Errorf("got %d nodes, want 2", len(snap.Nodes))
	}
	if len(snap.Edges) != 2 {
		t.Errorf("got %d edges, want 2", len(snap.Edges))
	}

	// Check that //lib/bar:bar was normalized
	var found bool
	for _, edge := range snap.Edges {
		if edge.From == "//app/foo:lib" && edge.To == "//lib/bar" {
			found = true
		}
	}
	if !found {
		t.Error("expected edge //app/foo:lib -> //lib/bar (normalized)")
	}

	// Check test rule detection
	testNode := snap.Nodes["//app/foo:lib_test"]
	if testNode == nil {
		t.Fatal("missing test node")
	}
	if !testNode.IsTest {
		t.Error("expected lib_test to be detected as test")
	}
}

func TestClassifyDep(t *testing.T) {
	tests := []struct {
		attr string
		want string
	}{
		{"deps", "COMPILE"},
		{"runtime_deps", "RUNTIME"},
		{"data", "DATA"},
		{"srcs", ""},
		{"tools", ""},
	}

	for _, tt := range tests {
		t.Run(tt.attr, func(t *testing.T) {
			got := classifyDep(tt.attr)
			if got != tt.want {
				t.Errorf("classifyDep(%q) = %q, want %q", tt.attr, got, tt.want)
			}
		})
	}
}

func TestIsTestRule(t *testing.T) {
	tests := []struct {
		ruleClass string
		want      bool
	}{
		{"go_test", true},
		{"java_test", true},
		{"py_tests", true},
		{"test_suite", true},
		{"go_library", false},
		{"java_binary", false},
	}

	for _, tt := range tests {
		t.Run(tt.ruleClass, func(t *testing.T) {
			got := isTestRule(tt.ruleClass)
			if got != tt.want {
				t.Errorf("isTestRule(%q) = %v, want %v", tt.ruleClass, got, tt.want)
			}
		})
	}
}

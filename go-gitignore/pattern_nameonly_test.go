// SPDX-License-Identifier: MIT

package gitignore_test

import (
	"strings"
	"testing"

	"github.com/boyter/gocodewalker/go-gitignore"
)

// TestNameOnly checks which patterns are independent of the directory their
// GitIgnore is anchored at, since a caller that anchors the same patterns at
// every directory it walks relies on this to know when one anchoring will do.
func TestNameOnly(t *testing.T) {
	testCases := []struct {
		Patterns string
		Expected bool
	}{
		{"", true},                    // nothing to depend on a base
		{"# comment only", true},      // ditto, the comment produces no pattern
		{"*.md", true},                // suffix name
		{"node_modules", true},        // literal name
		{"build/", true},              // directory only name
		{"!keep.md", true},            // negated name
		{"*.md\n!keep.md\nfoo", true}, // every pattern a name
		{"/build", false},             // anchored, means the base directory only
		{"docs/build", false},         // path, relative to the base
		{"**/testdata", false},        // recursive, walks out from the base
		{"*.md\n/build", false},       // one dependent pattern is enough
	}

	for _, tc := range testCases {
		t.Run(tc.Patterns, func(t *testing.T) {
			patterns := gitignore.Compile(strings.NewReader(tc.Patterns), nil)
			if got := patterns.NameOnly(); got != tc.Expected {
				t.Errorf("Compile(%q).NameOnly() = %v, want %v", tc.Patterns, got, tc.Expected)
			}
		})
	}
}

// TestCompileAnchorsAnywhere checks that patterns parsed once can be anchored
// at more than one base, and that each anchoring behaves as though the patterns
// had been parsed there.
func TestCompileAnchorsAnywhere(t *testing.T) {
	patterns := gitignore.Compile(strings.NewReader("*.md\n/build"), nil)

	shallow := gitignore.NewWithPatterns(patterns, "/repo", nil)
	deep := gitignore.NewWithPatterns(patterns, "/repo/a/b", nil)

	// the name pattern gives the same answer from either anchoring
	for _, i := range []gitignore.GitIgnore{shallow, deep} {
		if m := i.MatchIsDir("/repo/a/b/readme.md", false); m == nil || !m.Ignore() {
			t.Errorf("base %q did not ignore a .md file", i.Base())
		}
	}

	// the anchored pattern only matches directly beneath its own base
	if m := deep.MatchIsDir("/repo/a/b/build", true); m == nil || !m.Ignore() {
		t.Errorf("deep base did not ignore its own build directory")
	}
	if m := shallow.MatchIsDir("/repo/a/b/build", true); m != nil {
		t.Errorf("shallow base matched a build directory below it: %v", m)
	}

	// a path outside the base is not this GitIgnore's business
	if m := shallow.MatchIsDir("/elsewhere/readme.md", false); m != nil {
		t.Errorf("matched a path outside the base: %v", m)
	}
}

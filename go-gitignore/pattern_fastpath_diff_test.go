// SPDX-License-Identifier: MIT

package gitignore_test

import (
	"bytes"
	"testing"

	"github.com/danwakefield/fnmatch"

	"github.com/boyter/gocodewalker/go-gitignore"
)

// TestNamePatternFastPathsAgreeWithFnmatch is a differential test over the
// classified matchers. A name pattern is matched against a single path
// component, so fnmatch of the pattern against the base name is the definition
// the fast paths have to reproduce; anything that classifies a pattern into the
// wrong bucket shows up here as a disagreement.
//
// The interesting cases are the ones that look like a fast path but are not:
// an escaped glob is a literal and must not be treated as a wildcard, and "**"
// is not the bare "*" that matches everything.
func TestNamePatternFastPathsAgreeWithFnmatch(t *testing.T) {
	patterns := []string{
		// exact
		"node_modules", ".DS_Store", "Makefile",
		// suffix
		"*.o", "*.pyc", "*.min.js", "*",
		// prefix
		".*", "vmlinuz*", "core.*", "a*",
		// contains
		"*.o.*", "*test*", "*a*",
		// escaped globs: literal, not wildcards
		`abc\*`, `\*`, `\?`, `a\[b`, `\*abc`,
		// genuinely complex
		"**", "*.[oa]", "foo?bar", "a?c*", "abc*def", "[abc]*", "*.[ch]",
		"?", "f*o*o", "*.{c,h}",
	}
	names := []string{
		"node_modules", ".DS_Store", "Makefile", "foo.o", "foo.obj", "foo.a",
		".gitignore", ".", "..", "vmlinuz", "vmlinuz-6.1", "core.1234",
		"abc*", "abc", "*", "?", "a[b", "atest", "test", "mytestfile",
		"foo.o.bak", "a", "ab", "f o o", "fooo", "foo?bar", "axc", "axcdef",
		"abcdef", "x.c", "x.h", "x.py", "", "ünïcødé.o", "a.o.b",
	}

	for _, p := range patterns {
		ignore := gitignore.New(bytes.NewBufferString(p+"\n"), "/base", nil)
		for _, n := range names {
			// A name pattern is applied to the base name, so a bare name is the
			// whole of the path here and the two must agree.
			want := fnmatch.Match(p, n, 0)
			got := ignore.Relative(n, false) != nil

			if got != want {
				t.Errorf("pattern %q name %q: fast path says %v, fnmatch says %v", p, n, got, want)
			}

			// The same pattern reached through a nested path must match on the
			// base name alone, which is where the base name extraction that
			// replaced filepath.Split has to agree too.
			nested := "a/b/" + n
			if n != "" {
				gotNested := ignore.Relative(nested, false) != nil
				if gotNested != want {
					t.Errorf("pattern %q nested %q: fast path says %v, fnmatch on base says %v", p, nested, gotNested, want)
				}
			}
		}
	}
}

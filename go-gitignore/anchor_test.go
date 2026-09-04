// SPDX-License-Identifier: MIT

package gitignore_test

import (
	"strings"
	"testing"

	gitignore "github.com/boyter/gocodewalker/go-gitignore"
)

// TestAnchoringAndRecursion is a table of pattern x path x expected covering
// the anchoring and "**" rules of gitignore(5). Every expectation here was
// taken from git itself rather than from the specification prose: each case
// was built as a real repository and run through
//
//	git check-ignore <path>
//
// using the exit status (0 means ignored) rather than "-v", because "-v"
// reports a matching *negative* pattern with exit status 0 as well and so
// cannot be used to decide whether a path is ignored.
//
// The cases are matched at the level of a single ignore file, which is what
// Relative does. git additionally ignores everything beneath an ignored
// directory; that inheritance belongs to the walker, which prunes such
// directories, so it is not represented here.
func TestAnchoringAndRecursion(t *testing.T) {
	_tests := []struct {
		Pattern string
		Path    string
		IsDir   bool
		Match   bool
	}{
		// a leading "/" anchors the pattern to the directory holding the
		// .gitignore, so it matches there and nowhere below
		{"/drop.go", "drop.go", false, true},
		{"/drop.go", "sub/drop.go", false, false},
		{"/drop.go", "sub/deep/drop.go", false, false},

		// without the leading "/" and with no separator anywhere in the
		// pattern, the name matches at any depth
		{"drop.go", "drop.go", false, true},
		{"drop.go", "sub/drop.go", false, true},
		{"drop.go", "sub/deep/drop.go", false, true},

		// a separator anywhere but the end also anchors the pattern, so
		// "sub/drop.go" is anchored even without a leading "/"
		{"sub/drop.go", "sub/drop.go", false, true},
		{"sub/drop.go", "deep/sub/drop.go", false, false},
		{"/sub/drop.go", "sub/drop.go", false, true},
		{"/sub/drop.go", "deep/sub/drop.go", false, false},

		// a trailing "/" restricts the pattern to directories
		{"/foo/", "foo", true, true},
		{"/foo/", "foo", false, false},
		{"/foo/", "sub/foo", true, false},
		{"foo/", "foo", true, true},
		{"foo/", "sub/foo", true, true},
		{"foo/", "sub/foo", false, false},

		// a leading "**/" matches at any depth, including zero directories
		{"**/foo", "foo", false, true},
		{"**/foo", "sub/foo", false, true},
		{"**/foo", "sub/deep/foo", false, true},

		// "a/**/b" likewise matches zero or more directories in between
		{"a/**/b", "a/b", false, true},
		{"a/**/b", "a/mid/b", false, true},
		{"a/**/b", "a/mid/deep/b", false, true},

		// a *trailing* "/**" is the opposite case: it matches everything
		// inside the directory, and so requires at least one component
		// below it. git does not ignore the directory itself.
		{"foo/**", "foo", true, false},
		{"foo/**", "foo", false, false},
		{"foo/**", "foo/inner", false, true},
		{"foo/**", "foo/inner/deeper", false, true},
		{"/sub/**", "sub", true, false},
		{"/sub/**", "sub/keep.go", false, true},
		{"**/hide/**", "subdir/hide", true, false},
		{"**/hide/**", "subdir/hide/foo", false, true},
		{"exclude/**", "exclude", true, false},
		{"exclude/**", "exclude/other_file.txt", false, true},
	}

	for _, _test := range _tests {
		_ignore := gitignore.New(
			strings.NewReader(_test.Pattern+"\n"), "/base", nil,
		)

		_got := _ignore.Relative(_test.Path, _test.IsDir) != nil
		if _got != _test.Match {
			t.Errorf(
				"pattern %q against %q (isdir %v): expected match %v, got %v",
				_test.Pattern, _test.Path, _test.IsDir, _test.Match, _got,
			)
		}

		// the same expectation must hold through Absolute, which is the
		// entry point the walker actually uses
		_abs := _ignore.Absolute("/base/"+_test.Path, _test.IsDir) != nil
		if _abs != _test.Match {
			t.Errorf(
				"pattern %q against absolute %q (isdir %v): expected match %v, got %v",
				_test.Pattern, "/base/"+_test.Path, _test.IsDir, _test.Match, _abs,
			)
		}
	}
} // TestAnchoringAndRecursion()

// TestAnchoredNegation covers negation combined with anchoring. These cases
// are stated as an ignore decision rather than as "did some pattern match",
// because a negative pattern matches while leaving the path *not* ignored, so
// only the decision is comparable to git. As above, the expectations come from
// running git check-ignore over the equivalent repository and reading its exit
// status.
func TestAnchoredNegation(t *testing.T) {
	_tests := []struct {
		Content string
		Path    string
		IsDir   bool
		Ignored bool
	}{
		// "!/foo" un-ignores only the root foo, leaving deeper ones ignored
		{"foo\n!/foo\n", "foo", false, false},
		{"foo\n!/foo\n", "sub/foo", false, true},

		// an unanchored negation un-ignores at every depth
		{"foo\n!foo\n", "foo", false, false},
		{"foo\n!foo\n", "sub/foo", false, false},

		// an anchored ignore with an unanchored negation
		{"/foo\n!foo\n", "foo", false, false},

		// order matters: the last matching pattern decides
		{"!/foo\nfoo\n", "foo", false, true},

		// a negation cannot re-include a path below an ignored *directory*,
		// but "sub/**" does not ignore "sub" itself, so this one can
		{"/sub/**\n!/sub/keep.go\n", "sub/keep.go", false, false},
		{"/sub/**\n!/sub/keep.go\n", "sub/drop.go", false, true},
	}

	for _, _test := range _tests {
		_ignore := gitignore.New(strings.NewReader(_test.Content), "/base", nil)

		_ignored := false
		if _match := _ignore.Relative(_test.Path, _test.IsDir); _match != nil {
			_ignored = _match.Ignore()
		}

		if _ignored != _test.Ignored {
			t.Errorf(
				"content %q against %q (isdir %v): expected ignored %v, got %v",
				_test.Content, _test.Path, _test.IsDir, _test.Ignored, _ignored,
			)
		}
	}
} // TestAnchoredNegation()

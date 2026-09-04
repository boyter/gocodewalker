// SPDX-License-Identifier: MIT

package gitignore

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Reference implementations
//
// These are the pre-optimisation versions of relativeToBase and MatchIsDir,
// kept verbatim so the fast paths can be differentially tested against the
// behaviour they are required to preserve.
// ---------------------------------------------------------------------------

// relativeToBaseReference is relativeToBase as it was before the prefix strip.
func relativeToBaseReference(base, path string) (string, bool) {
	_rel, _err := filepath.Rel(base, path)
	if _err != nil {
		return "", false
	}
	if _rel == ".." || strings.HasPrefix(_rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return _rel, true
}

// absoluteReference is ignore.Absolute built on the reference relativeToBase.
func absoluteReference(g GitIgnore, path string, isdir bool) Match {
	_rel, ok := relativeToBaseReference(g.Base(), path)
	if !ok {
		return nil
	}
	return g.Relative(_rel, isdir)
}

// matchIsDirReference is ignore.MatchIsDir as it was before the cache skip: it
// always resolved the path to absolute form, and it reaches Relative through
// the reference relativeToBase, so it is independent of both optimisations.
func matchIsDirReference(g GitIgnore, path string, isdir bool) Match {
	path = filepath.ToSlash(path)
	_path, _err := filepath.Abs(path)
	if _err != nil {
		return nil
	}
	_path = filepath.ToSlash(_path)
	return absoluteReference(g, _path, isdir)
}

// matchString renders a Match for comparison, distinguishing no match from a
// match on the empty pattern.
func matchString(m Match) string {
	if m == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s@%s", m.String(), m.Position())
}

// ---------------------------------------------------------------------------
// 1. relativeToBase edge cases
// ---------------------------------------------------------------------------

// relativeToBaseCases is the shared table of awkward (base, path) pairs. Every
// one is checked against the reference implementation rather than against a
// hand written expectation, so the table can be extended without anyone having
// to work out what filepath.Rel does first.
var relativeToBaseCases = []struct {
	name string
	base string
	path string
}{
	// the ordinary case the fast path exists for
	{"simple child", "/a/b", "/a/b/c"},
	{"deep child", "/a/b", "/a/b/c/d/e/f.go"},
	{"single element base", "/a", "/a/b"},

	// textual prefix that is not a path prefix; this is what the separator
	// test after the prefix test is guarding against
	{"sibling sharing prefix", "/a/foo", "/a/foobar"},
	{"sibling sharing prefix deep", "/a/foo", "/a/foobar/baz.go"},
	{"one char longer", "/a/foo", "/a/foo2"},

	// base and path identical, or path shorter
	{"identical", "/a/b", "/a/b"},
	{"path shorter than base", "/a/b/c", "/a/b"},
	{"path is parent", "/a/b", "/a"},

	// escaping base
	{"sibling directory", "/a/b", "/a/c/d"},
	{"unrelated root", "/a/b", "/x/y"},
	{"escape via dotdot", "/a/b", "/a/b/../c"},
	{"escape to root", "/a/b", "/"},

	// degenerate bases
	{"empty base", "", "/a/b"},
	{"root base", "/", "/a/b"},
	{"root base root path", "/", "/"},
	{"base is dot", ".", "a/b"},
	{"relative base relative path", "a/b", "a/b/c"},
	{"relative base absolute path", "a/b", "/a/b/c"},
	{"absolute base relative path", "/a/b", "a/b/c"},

	// trailing and doubled separators on base
	{"trailing slash base", "/a/b/", "/a/b/c"},
	{"trailing slash base doubled", "/a/b/", "/a/b//c"},
	{"double slash base", "/a//b", "/a//b/c"},
	{"double slash base clean path", "/a//b", "/a/b/c"},
	{"base all slashes", "//", "//a"},

	// unclean bases that still have path as a literal prefix
	{"dot element in base", "/a/./b", "/a/./b/c"},
	{"dotdot element in base", "/a/b/..", "/a/b/../c"},

	// unclean remainders
	{"dot element in remainder", "/a/b", "/a/b/./c"},
	{"dotdot element in remainder", "/a/b", "/a/b/c/../d"},
	{"doubled separator in remainder", "/a/b", "/a/b//c"},
	{"trailing separator in remainder", "/a/b", "/a/b/c/"},
	{"remainder is empty", "/a/b", "/a/b/"},
	{"remainder is dot", "/a/b", "/a/b/."},
	{"remainder is dotdot", "/a/b", "/a/b/.."},
	{"remainder is dotdot deep", "/a/b", "/a/b/../.."},

	// elements that merely begin with a dot, which are ordinary names
	{"hidden file", "/a/b", "/a/b/.gitignore"},
	{"hidden dir", "/a/b", "/a/b/.github/workflows/ci.yml"},
	{"dotdotdot element", "/a/b", "/a/b/.../c"},
	{"element ending in dot", "/a/b", "/a/b/c./d"},

	// empty path
	{"empty path", "/a/b", ""},
	{"both empty", "", ""},

	// spaces, and characters that are ordinary in a path
	{"spaces", "/a/b", "/a/b/some file.go"},
	{"backslash in element", "/a/b", `/a/b/we\ird`},
	{"colon in element", "/a/b", "/a/b/c:d"},
}

// TestRelativeToBaseMatchesReference checks the optimised relativeToBase
// against the pre-optimisation implementation over the awkward cases.
func TestRelativeToBaseMatchesReference(t *testing.T) {
	for _, _test := range relativeToBaseCases {
		t.Run(_test.name, func(t *testing.T) {
			_gotRel, _gotOK := relativeToBase(_test.base, _test.path)
			_wantRel, _wantOK := relativeToBaseReference(_test.base, _test.path)

			if _gotOK != _wantOK {
				t.Fatalf(
					"relativeToBase(%q, %q): ok = %v, want %v (rel %q, want %q)",
					_test.base, _test.path, _gotOK, _wantOK, _gotRel, _wantRel,
				)
			}
			if _wantOK && _gotRel != _wantRel {
				t.Fatalf(
					"relativeToBase(%q, %q): rel = %q, want %q",
					_test.base, _test.path, _gotRel, _wantRel,
				)
			}
		})
	}
}

// TestRelativeToBaseFastPathActuallyFires guards against the fast path silently
// becoming dead code, which would leave the differential tests passing while
// testing nothing. It is not a correctness test.
func TestRelativeToBaseFastPathActuallyFires(t *testing.T) {
	if !fastRelSupported {
		t.Skip("fast path is compiled out on " + runtime.GOOS)
	}
	if _, ok := relativeToBase("/a/b", "/a/b/c/d.go"); !ok {
		t.Fatal("expected a match for a plain child path")
	}
	// the conditions that must hold for the strip to be taken
	if !strings.HasPrefix("/a/b/c/d.go", "/a/b") {
		t.Fatal("prefix test does not hold for the sample")
	}
	if !isCleanRelative("c/d.go") {
		t.Fatal("isCleanRelative rejected a clean remainder; fast path is dead")
	}
}

// TestIsCleanRelative checks isCleanRelative directly, since it is what decides
// whether the strip is safe.
func TestIsCleanRelative(t *testing.T) {
	_clean := []string{
		"a", "a/b", "a/b/c.go", ".gitignore", "...", "a./b", ".a/b",
		"a/.b/c", "..a", "a..", "a/..b/c", "-", "a b/c d",
	}
	_unclean := []string{
		"", "/", "/a", "a/", "a//b", ".", "..", "./a", "../a", "a/.",
		"a/..", "a/./b", "a/../b", "a/b/", "//a", "a//", "./", "../",
	}

	for _, p := range _clean {
		if !isCleanRelative(p) {
			t.Errorf("isCleanRelative(%q) = false, want true", p)
		}
		// cross check against filepath.Clean, which is the property we rely on
		if filepath.ToSlash(filepath.Clean(p)) != p {
			t.Errorf("isCleanRelative(%q) accepted a path Clean rewrites to %q",
				p, filepath.Clean(p))
		}
	}
	for _, p := range _unclean {
		if isCleanRelative(p) {
			t.Errorf("isCleanRelative(%q) = true, want false", p)
		}
	}
}

// TestIsCleanRelativeAgreesWithClean is the general statement of the property
// isCleanRelative is asserting: it accepts p only when p is a relative path
// that filepath.Clean leaves alone.
func TestIsCleanRelativeAgreesWithClean(t *testing.T) {
	_elements := []string{"a", "b", "", ".", "..", "...", ".a", "a."}

	var _paths []string
	for _, e1 := range _elements {
		_paths = append(_paths, e1)
		for _, e2 := range _elements {
			_paths = append(_paths, e1+"/"+e2)
			for _, e3 := range _elements {
				_paths = append(_paths, e1+"/"+e2+"/"+e3)
			}
		}
	}

	for _, p := range _paths {
		// the property relied on is stronger than "Clean leaves it alone":
		// "." and "../a" are both clean, but a remainder containing a "." or
		// ".." element does not simply append to base, so it must not be
		// stripped
		_elements := strings.Split(p, "/")
		_want := p != "" && !filepath.IsAbs(p) &&
			filepath.ToSlash(filepath.Clean(p)) == p &&
			!slices.Contains(_elements, ".") &&
			!slices.Contains(_elements, "..")
		if got := isCleanRelative(p); got != _want {
			t.Errorf("isCleanRelative(%q) = %v, want %v (Clean gives %q)",
				p, got, _want, filepath.Clean(p))
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Differential test over a real tree with real .gitignore files
// ---------------------------------------------------------------------------

// treeCorpus walks root and returns every .gitignore found (as a GitIgnore
// based at its own directory) along with every path in the tree.
func treeCorpus(t *testing.T, root string, maxPaths int) ([]GitIgnore, []struct {
	Path  string
	IsDir bool
}) {
	t.Helper()

	var _ignores []GitIgnore
	var _paths []struct {
		Path  string
		IsDir bool
	}

	_abs, _err := filepath.Abs(root)
	if _err != nil {
		t.Fatalf("resolving %q: %v", root, _err)
	}

	_ = filepath.WalkDir(_abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// unreadable directories are not what is under test
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if len(_paths) >= maxPaths {
			return filepath.SkipAll
		}

		_paths = append(_paths, struct {
			Path  string
			IsDir bool
		}{path, d.IsDir()})

		if !d.IsDir() && d.Name() == ".gitignore" {
			if _ignore, _e := NewFromFile(path); _e == nil && _ignore != nil {
				_ignores = append(_ignores, _ignore)
			}
		}
		return nil
	})

	return _ignores, _paths
}

// runTreeDifferential is the body shared by the real tree differential tests.
// For every path in the tree, against every .gitignore in the tree, the
// optimised MatchIsDir must return exactly what the pre-optimisation
// implementation returns. Paths are tested in absolute form, in relative form,
// and in a deliberately unclean form.
func runTreeDifferential(t *testing.T, root string, maxPaths int) {
	t.Helper()

	_ignores, _paths := treeCorpus(t, root, maxPaths)
	if len(_ignores) == 0 {
		t.Skipf("no .gitignore files found under %s", root)
	}
	if len(_paths) == 0 {
		t.Skipf("no paths found under %s", root)
	}
	t.Logf("%d .gitignore files, %d paths from %s", len(_ignores), len(_paths), root)

	_wd, _err := os.Getwd()
	if _err != nil {
		t.Fatalf("getwd: %v", _err)
	}

	// the forms each path is tested in
	_forms := []struct {
		name string
		make func(abs string) (string, bool)
	}{
		{"absolute", func(abs string) (string, bool) { return abs, true }},
		{"relative", func(abs string) (string, bool) {
			// only meaningful when the path really is under the working
			// directory, since MatchIsDir resolves relative paths against it
			_rel, _e := filepath.Rel(_wd, abs)
			if _e != nil || strings.HasPrefix(_rel, "..") {
				return "", false
			}
			return _rel, true
		}},
		{"unclean dot", func(abs string) (string, bool) {
			_dir, _file := filepath.Split(abs)
			if _dir == "" || _file == "" {
				return "", false
			}
			return _dir + "./" + _file, true
		}},
		{"unclean dotdot", func(abs string) (string, bool) {
			_dir, _file := filepath.Split(abs)
			if _dir == "" || _file == "" {
				return "", false
			}
			return _dir + "x/../" + _file, true
		}},
		{"doubled separator", func(abs string) (string, bool) {
			_dir, _file := filepath.Split(abs)
			if _dir == "" || _file == "" {
				return "", false
			}
			return _dir + "/" + _file, true
		}},
		{"trailing separator", func(abs string) (string, bool) {
			return abs + string(filepath.Separator), true
		}},
	}

	var _compared int
	for _, _ignore := range _ignores {
		for _, _p := range _paths {
			for _, _form := range _forms {
				_input, ok := _form.make(_p.Path)
				if !ok {
					continue
				}

				_got := _ignore.MatchIsDir(_input, _p.IsDir)
				_want := matchIsDirReference(_ignore, _input, _p.IsDir)
				_compared++

				if matchString(_got) != matchString(_want) {
					t.Fatalf(
						"MatchIsDir(%q, %v) [%s] against base %q:\n got %s\nwant %s",
						_input, _p.IsDir, _form.name, _ignore.Base(),
						matchString(_got), matchString(_want),
					)
				}
			}
		}
	}
	t.Logf("%d comparisons, all identical", _compared)
}

// TestTreeDifferentialSelf runs the differential over this repository, which
// has real .gitignore files and is always present.
func TestTreeDifferentialSelf(t *testing.T) {
	// walk up to the repository root
	_root, _err := os.Getwd()
	if _err != nil {
		t.Fatalf("getwd: %v", _err)
	}
	for i := 0; i < 5; i++ {
		if _, _e := os.Stat(filepath.Join(_root, ".git")); _e == nil {
			break
		}
		_root = filepath.Dir(_root)
	}
	runTreeDifferential(t, _root, 20000)
}

// TestTreeDifferentialExternal runs the differential over a large external tree
// when one is named, which is how this was validated against the Linux kernel.
//
//	GITIGNORE_DIFF_TREE=/path/to/linux go test -run TestTreeDifferentialExternal ./go-gitignore
func TestTreeDifferentialExternal(t *testing.T) {
	_root := os.Getenv("GITIGNORE_DIFF_TREE")
	if _root == "" {
		t.Skip("set GITIGNORE_DIFF_TREE to a tree to run the external differential")
	}
	_max := 200000
	runTreeDifferential(t, _root, _max)
}

// TestTreeDifferentialSyntheticNames builds a tree whose names are chosen to
// stress the fast path (names that are prefixes of their siblings, names made
// of dots, names with separators adjacent to them) and runs the same
// differential over it. A real tree does not reliably contain these.
func TestTreeDifferentialSyntheticNames(t *testing.T) {
	_root := t.TempDir()

	// names deliberately chosen so that one is a textual prefix of another
	_dirs := []string{
		"foo", "foobar", "foo2", "foo.bar", ".hidden", "...", "a",
		"foo/nested", "foobar/nested", "foo/foo", "a/aa/aaa",
	}
	for _, d := range _dirs {
		if _err := os.MkdirAll(filepath.Join(_root, d), 0o755); _err != nil {
			t.Fatalf("mkdir %s: %v", d, _err)
		}
	}

	_files := []string{
		"foo/x.go", "foobar/x.go", "foo2/x.go", "foo/nested/x.go",
		"foo/foo/foo.go", "a/aa/aaa/deep.txt", ".hidden/x.go", ".../x.go",
		"foo.bar/x.log", "foo/y.log", "foobar/y.log", "a/z.tmp",
	}
	for _, f := range _files {
		if _err := os.WriteFile(filepath.Join(_root, f), []byte("x\n"), 0o644); _err != nil {
			t.Fatalf("write %s: %v", f, _err)
		}
	}

	// .gitignore files at several depths, so that a path is tested against
	// bases both at and above its own directory
	_gitignores := map[string]string{
		".":          "*.log\n!keep.log\n/foo2\nnested/\n",
		"foo":        "y.log\n*.go\n!x.go\n",
		"foobar":     "x.go\n",
		"a":          "*.tmp\naa/\n",
		"a/aa":       "!deep.txt\n*.txt\n",
		".hidden":    "*\n",
		"foo.bar":    "*.log\n",
		"foo/foo":    "foo.go\n",
		"foo/nest":   "", // does not exist, ignored below
		"a/aa/aaa":   "deep.txt\n",
		"foo2":       "x.go\n",
		"foo/nested": "x.go\n!x.go\n",
	}
	for _dir, _content := range _gitignores {
		_full := filepath.Join(_root, _dir)
		if _info, _e := os.Stat(_full); _e != nil || !_info.IsDir() {
			continue
		}
		if _err := os.WriteFile(filepath.Join(_full, ".gitignore"), []byte(_content), 0o644); _err != nil {
			t.Fatalf("write gitignore in %s: %v", _dir, _err)
		}
	}

	runTreeDifferential(t, _root, 20000)
}

// TestTreeDifferentialSymlinks covers the question of whether the strip changes
// anything for symlinked paths. filepath.Rel is purely lexical and resolves no
// links, and so is the strip, so the two must still agree; this pins that down.
func TestTreeDifferentialSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on Windows")
	}

	_root := t.TempDir()
	for _, d := range []string{"real", "real/sub", "other"} {
		if _err := os.MkdirAll(filepath.Join(_root, d), 0o755); _err != nil {
			t.Fatalf("mkdir %s: %v", d, _err)
		}
	}
	for _, f := range []string{"real/a.log", "real/sub/b.log", "other/c.log"} {
		if _err := os.WriteFile(filepath.Join(_root, f), []byte("x\n"), 0o644); _err != nil {
			t.Fatalf("write %s: %v", f, _err)
		}
	}
	if _err := os.WriteFile(filepath.Join(_root, ".gitignore"), []byte("*.log\n!sub/*.log\n"), 0o644); _err != nil {
		t.Fatalf("write gitignore: %v", _err)
	}

	// a symlinked directory pointing sideways, and a symlink pointing outside
	// the base entirely
	if _err := os.Symlink(filepath.Join(_root, "other"), filepath.Join(_root, "link")); _err != nil {
		t.Fatalf("symlink: %v", _err)
	}
	_outside := t.TempDir()
	if _err := os.WriteFile(filepath.Join(_outside, "d.log"), []byte("x\n"), 0o644); _err != nil {
		t.Fatalf("write outside: %v", _err)
	}
	if _err := os.Symlink(_outside, filepath.Join(_root, "escape")); _err != nil {
		t.Fatalf("symlink escape: %v", _err)
	}

	_ignore, _err := NewFromFile(filepath.Join(_root, ".gitignore"))
	if _err != nil {
		t.Fatalf("loading gitignore: %v", _err)
	}

	_cases := []struct {
		path  string
		isdir bool
	}{
		{filepath.Join(_root, "link"), true},
		{filepath.Join(_root, "link", "c.log"), false},
		{filepath.Join(_root, "escape"), true},
		{filepath.Join(_root, "escape", "d.log"), false},
		{filepath.Join(_outside, "d.log"), false},
		{filepath.Join(_root, "real", "sub", "b.log"), false},
		// the lexically resolved form of the same file, which is a different
		// string but the same file on disk
		{filepath.Join(_root, "real", "..", "other", "c.log"), false},
	}

	for _, _c := range _cases {
		_got := _ignore.MatchIsDir(_c.path, _c.isdir)
		_want := matchIsDirReference(_ignore, _c.path, _c.isdir)
		if matchString(_got) != matchString(_want) {
			t.Errorf("MatchIsDir(%q, %v): got %s, want %s",
				_c.path, _c.isdir, matchString(_got), matchString(_want))
		}
	}
}

// ---------------------------------------------------------------------------
// 3. Relative path handling, which is what the cache skip must not disturb
// ---------------------------------------------------------------------------

// TestMatchIsDirRelativePathsUnaffected covers the case the cache skip could
// plausibly have broken: gocodewalker does not absolutise its walk root, so a
// relative root produces relative paths here, and those must still be resolved
// against the working directory rather than handed to Absolute as they are.
func TestMatchIsDirRelativePathsUnaffected(t *testing.T) {
	_root := t.TempDir()
	// t.TempDir can sit behind a symlink (/tmp -> /private/tmp on macOS), which
	// would make the resolved working directory differ from _root textually
	if _resolved, _err := filepath.EvalSymlinks(_root); _err == nil {
		_root = _resolved
	}

	if _err := os.MkdirAll(filepath.Join(_root, "sub"), 0o755); _err != nil {
		t.Fatalf("mkdir: %v", _err)
	}
	for _, f := range []string{"a.log", "b.go", "sub/c.log"} {
		if _err := os.WriteFile(filepath.Join(_root, f), []byte("x\n"), 0o644); _err != nil {
			t.Fatalf("write %s: %v", f, _err)
		}
	}
	if _err := os.WriteFile(filepath.Join(_root, ".gitignore"), []byte("*.log\n"), 0o644); _err != nil {
		t.Fatalf("write gitignore: %v", _err)
	}

	_ignore, _err := NewFromFile(filepath.Join(_root, ".gitignore"))
	if _err != nil {
		t.Fatalf("loading gitignore: %v", _err)
	}

	// chdir so that the relative paths below resolve to files under the base.
	// os.Chdir rather than t.Chdir, which needs a newer Go than this module
	// declares; nothing here runs in parallel, so the process wide change is
	// safe as long as it is put back.
	_prev, _err := os.Getwd()
	if _err != nil {
		t.Fatalf("getwd: %v", _err)
	}
	if _err := os.Chdir(_root); _err != nil {
		t.Fatalf("chdir: %v", _err)
	}
	t.Cleanup(func() { _ = os.Chdir(_prev) })

	_cases := []struct {
		path   string
		isdir  bool
		ignore bool
	}{
		{"a.log", false, true},
		{"b.go", false, false},
		{"sub/c.log", false, true},
		{"./a.log", false, true},
		{"sub/../a.log", false, true},
	}

	for _, _c := range _cases {
		_match := _ignore.MatchIsDir(_c.path, _c.isdir)
		_got := _match != nil && _match.Ignore()
		if _got != _c.ignore {
			t.Errorf("MatchIsDir(%q): ignore = %v, want %v (match %s)",
				_c.path, _got, _c.ignore, matchString(_match))
		}

		// and it must still agree with the pre-optimisation implementation
		_want := matchIsDirReference(_ignore, _c.path, _c.isdir)
		if matchString(_match) != matchString(_want) {
			t.Errorf("MatchIsDir(%q): got %s, want %s",
				_c.path, matchString(_match), matchString(_want))
		}
	}
}

// ---------------------------------------------------------------------------
// 4. Fuzzing
// ---------------------------------------------------------------------------

// FuzzRelativeToBase asserts the only property that matters: for any base and
// any path, the optimised implementation returns exactly what the
// pre-optimisation implementation returns.
func FuzzRelativeToBase(f *testing.F) {
	for _, _test := range relativeToBaseCases {
		f.Add(_test.base, _test.path)
	}
	// a few extra seeds aimed at the prefix boundary
	f.Add("/a", "/a/b")
	f.Add("/a", "/ab")
	f.Add("/", "//")
	f.Add("/a/", "//")

	f.Fuzz(func(t *testing.T, base, path string) {
		_gotRel, _gotOK := relativeToBase(base, path)
		_wantRel, _wantOK := relativeToBaseReference(base, path)

		if _gotOK != _wantOK {
			t.Fatalf("relativeToBase(%q, %q): ok = %v, want %v (rel %q, want %q)",
				base, path, _gotOK, _wantOK, _gotRel, _wantRel)
		}
		if _wantOK && _gotRel != _wantRel {
			t.Fatalf("relativeToBase(%q, %q): rel = %q, want %q",
				base, path, _gotRel, _wantRel)
		}
	})
}

// ---------------------------------------------------------------------------
// 5. Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkRelativeToBase(b *testing.B) {
	_base := "/home/user/projects/linux"
	_path := "/home/user/projects/linux/drivers/net/ethernet/intel/e1000/e1000_main.c"

	b.Run("optimised", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, ok := relativeToBase(_base, _path); !ok {
				b.Fatal("expected a match")
			}
		}
	})
	b.Run("reference", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, ok := relativeToBaseReference(_base, _path); !ok {
				b.Fatal("expected a match")
			}
		}
	})
}

func BenchmarkMatchIsDir(b *testing.B) {
	_root := b.TempDir()
	if _err := os.WriteFile(filepath.Join(_root, ".gitignore"), []byte("*.o\n*.log\nbuild/\n"), 0o644); _err != nil {
		b.Fatalf("write gitignore: %v", _err)
	}
	_ignore, _err := NewFromFile(filepath.Join(_root, ".gitignore"))
	if _err != nil {
		b.Fatalf("loading gitignore: %v", _err)
	}

	// a spread of distinct paths, as a walk sees, so the cache in the reference
	// implementation grows the way it does in a real walk
	_paths := make([]string, 4096)
	for i := range _paths {
		_paths[i] = filepath.ToSlash(filepath.Join(_root,
			fmt.Sprintf("drivers/net/ethernet/vendor%d/driver%d.c", i%64, i)))
	}

	b.Run("optimised", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ignore.MatchIsDir(_paths[i%len(_paths)], false)
		}
	})
	b.Run("reference", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			matchIsDirReference(_ignore, _paths[i%len(_paths)], false)
		}
	})
}

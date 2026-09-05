// SPDX-License-Identifier: MIT

package gocodewalker

import (
	"errors"
	"maps"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFindRepositoryRoot(t *testing.T) {
	// We expect this to walk back from file to root
	curdir, _ := os.Getwd()
	root := FindRepositoryRoot(curdir)

	if strings.HasSuffix(root, "file") {
		t.Error("Expected to walk back to root")
	}
}

func TestFindRepositoryRootWorktree(t *testing.T) {
	tmp := t.TempDir()

	// Parent repo with a real .git directory.
	parent := filepath.Join(tmp, "parent")
	if err := os.MkdirAll(filepath.Join(parent, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Nested worktree at parent/.agent-shell/wt with .git as a regular file
	// (as `git worktree add` produces).
	worktree := filepath.Join(parent, ".agent-shell", "wt")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	gitFile := filepath.Join(worktree, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: "+parent+"/.git/worktrees/wt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Subdirectory inside the worktree, so we exercise the upward walk.
	worktreeSub := filepath.Join(worktree, "sub", "dir")
	if err := os.MkdirAll(worktreeSub, 0o755); err != nil {
		t.Fatal(err)
	}

	// Plain directory with no repo marker anywhere up the tree (under tmp).
	orphan := filepath.Join(tmp, "orphan")
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		start string
		want  string
	}{
		{"parent repo root", parent, parent},
		{"worktree root", worktree, worktree},
		{"nested dir inside worktree", worktreeSub, worktree},
		{"no marker falls back to start", orphan, orphan},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindRepositoryRoot(tt.start)
			// Resolve symlinks so /var vs /private/var on macOS doesn't trip the comparison.
			gotR, _ := filepath.EvalSymlinks(got)
			wantR, _ := filepath.EvalSymlinks(tt.want)
			if gotR != wantR {
				t.Errorf("FindRepositoryRoot(%q) = %q, want %q", tt.start, got, tt.want)
			}
		})
	}
}

// TestWalkWildcardIgnoreThenReinclude reproduces issue #24: a wildcard ignore
// "/*/" that re-includes a directory via negation "!/keep/" must still walk the
// re-included directory's nested subdirectories. The leading-slash "/*/" pattern
// is anchored to the .gitignore directory and only matches first-level entries,
// so "keep/sub" must not be ignored (matching `git ls-files`).
func TestWalkWildcardIgnoreThenReinclude(t *testing.T) {
	tmp := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("/*/\n!/keep/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "keep", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "drop"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "keep", "a.rs"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "keep", "sub", "c.rs"), []byte("c"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "drop", "d.rs"), []byte("d"), 0o644); err != nil {
		t.Fatal(err)
	}

	fileListQueue := make(chan *File, 1000)
	walker := NewFileWalker(tmp, fileListQueue)
	if err := walker.Start(); err != nil {
		t.Fatal(err)
	}

	got := map[string]bool{}
	for f := range fileListQueue {
		rel, err := filepath.Rel(tmp, f.Location)
		if err != nil {
			t.Fatal(err)
		}
		got[filepath.ToSlash(rel)] = true
	}

	// Expected matches `git ls-files`: keep/a.rs and keep/sub/c.rs kept,
	// everything under drop/ ignored.
	if !got["keep/a.rs"] {
		t.Errorf("expected keep/a.rs to be walked, got %v", got)
	}
	if !got["keep/sub/c.rs"] {
		t.Errorf("expected keep/sub/c.rs to be walked (issue #24), got %v", got)
	}
	if got["drop/d.rs"] {
		t.Errorf("expected drop/d.rs to be ignored by /*/, got %v", got)
	}
}

func TestNewFileWalker(t *testing.T) {
	fileListQueue := make(chan *File, 10_000) // NB we set buffered to ensure we get everything
	curdir, _ := os.Getwd()
	walker := NewFileWalker(curdir, fileListQueue)
	_ = walker.Start()

	count := 0
	for range fileListQueue {
		count++
	}

	if count == 0 {
		t.Error("Expected to find at least one file")
	}
}

func TestNewFileWalkerEmptyEverything(t *testing.T) {
	fileListQueue := make(chan *File, 10_000) // NB we set buffered to ensure we get everything
	walker := NewFileWalker("", fileListQueue)

	called := false
	walker.SetErrorHandler(func(err error) bool {
		called = true
		return true
	})
	_ = walker.Start()

	count := 0
	for range fileListQueue {
		count++
	}

	if count != 0 {
		t.Error("Expected to find nothing")
	}

	if called {
		t.Error("expected to not be called")
	}
}

func TestNewFileWalkerEmptyEverythingParallel(t *testing.T) {
	fileListQueue := make(chan *File, 10_000) // NB we set buffered to ensure we get everything
	walker := NewParallelFileWalker([]string{}, fileListQueue)

	called := false
	walker.SetErrorHandler(func(err error) bool {
		called = true
		return true
	})
	_ = walker.Start()

	count := 0
	for range fileListQueue {
		count++
	}

	if count != 0 {
		t.Error("Expected to find nothing")
	}

	if called {
		t.Error("expected to not be called")
	}
}

func TestNewParallelFileWalker(t *testing.T) {
	fileListQueue := make(chan *File, 10_000) // NB we set buffered to ensure we get everything
	curdir, _ := os.Getwd()
	walker := NewParallelFileWalker([]string{curdir, curdir}, fileListQueue)
	_ = walker.Start()

	count := 0
	for range fileListQueue {
		count++
	}

	if count == 0 {
		t.Error("Expected to find at least one file")
	}
}

func TestNewFileWalkerStuff(t *testing.T) {
	fileListQueue := make(chan *File, 10_000) // NB we set buffered to ensure we get everything
	curdir, _ := os.Getwd()
	walker := NewFileWalker(curdir, fileListQueue)

	if walker.Walking() != false {
		t.Error("should not be walking yet")
	}

	walker.Terminate()
	_ = walker.Start()

	count := 0
	for range fileListQueue {
		count++
	}

	if count != 0 {
		t.Error("Expected to find no files")
	}
}

func TestNewFileWalkerFsOpenErrorHandler(t *testing.T) {
	osOpen := func(name string) (*os.File, error) {
		return nil, errors.New("error was handled")
	}

	walker := NewFileWalker(".", make(chan *File, 1000))
	walker.osOpen = osOpen

	wasCalled := false
	errorHandler := func(e error) bool {
		if e.Error() == "error was handled" {
			wasCalled = true
		}
		return false
	}
	walker.SetErrorHandler(errorHandler)
	err := walker.Start()

	if !wasCalled {
		t.Error("expected error to be called")
	}
	if err == nil {
		t.Error("expected error got nil")
	}
}

func TestNewFileWalkerNotDirectory(t *testing.T) {
	osOpen := func(name string) (*os.File, error) {
		f, _ := os.CreateTemp("", ".ignore")
		return f, nil
	}

	walker := NewFileWalker(".", make(chan *File, 10))
	walker.osOpen = osOpen
	walker.SetErrorHandler(func(e error) bool { return false })

	err := walker.Start()
	if !strings.Contains(err.Error(), "not a directory") {
		t.Error("expected not a directory got", err.Error())
	}
}

func randSeq(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func TestNewFileWalkerIgnoreFileCases(t *testing.T) {
	type testcase struct {
		Name       string
		Case       func() *FileWalker
		ExpectCall bool
	}

	testCases := []testcase{
		{
			Name: ".ignorefile ignore",
			Case: func() *FileWalker {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, ".ignore"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				// what we want to test is here
				walker.IgnoreIgnoreFile = true
				return walker
			},
			ExpectCall: false,
		},
		{
			Name: ".ignorefile include",
			Case: func() *FileWalker {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, ".ignore"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.IgnoreIgnoreFile = false
				return walker
			},
			ExpectCall: true,
		},
		{
			Name: ".gitignore ignore",
			Case: func() *FileWalker {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, ".gitignore"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.IgnoreGitIgnore = true
				return walker
			},
			ExpectCall: false,
		},
		{
			Name: ".gitignore include",
			Case: func() *FileWalker {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, ".gitignore"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.IgnoreGitIgnore = false
				return walker
			},
			ExpectCall: true,
		},
		{
			Name: "custom ignore file ignore",
			Case: func() *FileWalker {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "custom.ignore"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.CustomIgnore = []string{}
				return walker
			},
			ExpectCall: false,
		},
		{
			Name: "custom ignore file include",
			Case: func() *FileWalker {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "custom.ignore"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.CustomIgnore = []string{"custom.ignore"}
				return walker
			},
			ExpectCall: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			called := false
			osReadFile := func(name string) ([]byte, error) {
				called = true
				return nil, nil
			}

			walker := tc.Case()
			walker.osReadFile = osReadFile
			_ = walker.Start()

			if tc.ExpectCall {
				if !called {
					t.Errorf("expected to be called but was not!")
				}
			} else {
				if called {
					t.Errorf("expected to be ignored but was not!")
				}
			}
		})
	}
}

// NB the CustomIgnorePatterns relative cases here fail under go test -count=2
// and above. That is not this test's fault: go-gitignore keeps a process wide
// matchIsDirCache mapping a raw path to an absolute one, so the relative path
// "test.md" stays pinned to the temp directory of the first run and no longer
// resolves under the second run's base. See the walker notes for the details.
func TestNewFileWalkerFileCases(t *testing.T) {
	type testcase struct {
		Name     string
		Case     func() (*FileWalker, chan *File)
		Expected int
	}

	testCases := []testcase{
		{
			Name: "ExcludeListExtensions 0",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.txt"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.ExcludeListExtensions = []string{"txt"}
				return walker, fileListQueue
			},
			Expected: 0,
		},
		{
			Name: "ExcludeListExtensions 1",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.txt"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.ExcludeListExtensions = []string{"md"}
				return walker, fileListQueue
			},
			Expected: 1,
		},
		{
			Name: "AllowListExtensions 1",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.txt"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.AllowListExtensions = []string{"txt"}
				return walker, fileListQueue
			},
			Expected: 1,
		},
		{
			Name: "ExcludeListExtensions 0 Multiple",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.txt"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.ExcludeListExtensions = []string{"md", "go", "txt"}
				return walker, fileListQueue
			},
			Expected: 0,
		},
		{
			Name: "AllowListExtensions 0",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.AllowListExtensions = []string{"txt"}
				return walker, fileListQueue
			},
			Expected: 0,
		},
		{
			Name: "AllowListExtensions 1 Multiple",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.AllowListExtensions = []string{"txt", "md"}
				return walker, fileListQueue
			},
			Expected: 1,
		},
		{
			Name: "IncludeFilenameRegex 1",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.IncludeFilenameRegex = []*regexp.Regexp{regexp.MustCompile(".*")}
				return walker, fileListQueue
			},
			Expected: 1,
		},
		{
			Name: "IncludeFilenameRegex 0",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.IncludeFilenameRegex = []*regexp.Regexp{regexp.MustCompile("test.go")}
				return walker, fileListQueue
			},
			Expected: 0,
		},
		{
			Name: "ExcludeFilenameRegex 0",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.ExcludeFilenameRegex = []*regexp.Regexp{regexp.MustCompile(".*")}
				return walker, fileListQueue
			},
			Expected: 0,
		},
		{
			Name: "ExcludeFilenameRegex 1",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.ExcludeFilenameRegex = []*regexp.Regexp{regexp.MustCompile("nothing")}
				return walker, fileListQueue
			},
			Expected: 1,
		},
		{
			Name: "IncludeFilenameRegex 1",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.IncludeFilenameRegex = []*regexp.Regexp{regexp.MustCompile(".*")}
				return walker, fileListQueue
			},
			Expected: 1,
		},
		{
			Name: "IncludeFilenameRegex 0",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.IncludeFilenameRegex = []*regexp.Regexp{regexp.MustCompile("nothing")}
				return walker, fileListQueue
			},
			Expected: 0,
		},
		{
			Name: "CustomIgnorePatterns 0",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.CustomIgnorePatterns = []string{"*.md"}
				return walker, fileListQueue
			},
			Expected: 0,
		},
		{
			Name: "CustomIgnorePatterns 1",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.CustomIgnorePatterns = []string{"*.go"}
				return walker, fileListQueue
			},
			Expected: 1,
		},
		{
			Name: "CustomIgnorePatterns relative 0",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.md"))
				_ = os.Chdir(d)

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(".", fileListQueue)

				walker.CustomIgnorePatterns = []string{"*.md"}
				return walker, fileListQueue
			},
			Expected: 0,
		},
		{
			Name: "CustomIgnorePatterns relative 1",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.md"))
				_ = os.Chdir(d)

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(".", fileListQueue)

				walker.CustomIgnorePatterns = []string{"*.go"}
				return walker, fileListQueue
			},
			Expected: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// some cases chdir into a temp directory to exercise relative paths
			// and never chdir back, which leaves the whole process sitting in a
			// temp directory for every test that runs after them. Put it back.
			// NB this is not enough on its own to make go test -count=2 pass,
			// see the note on TestNewFileWalkerFileCases for why
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Chdir(cwd); err != nil {
					t.Fatal(err)
				}
			}()

			osReadFile := func(name string) ([]byte, error) {
				return nil, nil
			}

			walker, fileListQueue := tc.Case()
			walker.osReadFile = osReadFile
			_ = walker.Start()

			c := 0
			for range fileListQueue {
				c++
			}

			if c != tc.Expected {
				t.Errorf("expected %v but got %v", tc.Expected, c)
			}
		})
	}
}

func TestNewFileWalkerDirectoryCases(t *testing.T) {
	type testcase struct {
		Name     string
		Case     func() (*FileWalker, chan *File)
		Expected int
	}

	testCases := []testcase{
		{
			Name: "ExcludeDirectory 0",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "stuff")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "/test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.ExcludeDirectory = []string{"stuff"}
				return walker, fileListQueue
			},
			Expected: 0,
		},
		{
			Name: "ExcludeDirectory 1",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "stuff")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "/test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.ExcludeDirectory = []string{"notmatching"}
				return walker, fileListQueue
			},
			Expected: 1,
		},
		{
			Name: "ExcludeDirectory multi-level 1",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "stuff")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "/test.md"))
				d3 := filepath.Join(d2, "multi")
				_ = os.Mkdir(d3, 0777)
				_, _ = os.Create(filepath.Join(d3, "/test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.ExcludeDirectory = []string{"stuff/multi"}
				return walker, fileListQueue
			},
			Expected: 1,
		},
		{
			Name: "ExcludeDirectory multi-level 2",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "stuff")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "/test.md"))
				d3 := filepath.Join(d2, "multi")
				_ = os.Mkdir(d3, 0777)
				_, _ = os.Create(filepath.Join(d3, "/test.md"))

				d4 := filepath.Join(d2, "another/stuff/multi")
				_ = os.MkdirAll(d4, 0777)
				_, _ = os.Create(filepath.Join(d4, "/test.md"))

				d5 := filepath.Join(d2, "another/sstuff/multi")
				_ = os.MkdirAll(d5, 0777)
				_, _ = os.Create(filepath.Join(d5, "/test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.ExcludeDirectory = []string{"stuff/multi"}
				return walker, fileListQueue
			},
			Expected: 2,
		},
		{
			Name: "IncludeDirectory 1",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "stuff")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "/test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.IncludeDirectory = []string{"stuff"}
				return walker, fileListQueue
			},
			Expected: 1,
		},
		{
			Name: "IncludeDirectory 0",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "stuff")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "/test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.IncludeDirectory = []string{"otherthing"}
				return walker, fileListQueue
			},
			Expected: 0,
		},
		{
			Name: "IncludeDirectoryRegex 0",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "stuff")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "/test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.IncludeDirectoryRegex = []*regexp.Regexp{regexp.MustCompile("nothing")}
				return walker, fileListQueue
			},
			Expected: 0,
		},
		{
			Name: "IncludeDirectoryRegex 1",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "stuff")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "/test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.IncludeDirectoryRegex = []*regexp.Regexp{regexp.MustCompile("stuff")}
				return walker, fileListQueue
			},
			Expected: 1,
		},
		{
			Name: "ExcludeDirectoryRegex 0",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "stuff")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "/test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.ExcludeDirectoryRegex = []*regexp.Regexp{regexp.MustCompile("stuff")}
				return walker, fileListQueue
			},
			Expected: 0,
		},
		{
			Name: "ExcludeDirectoryRegex 0",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "stuff")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "/test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.ExcludeDirectoryRegex = []*regexp.Regexp{regexp.MustCompile(".*")}
				return walker, fileListQueue
			},
			Expected: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			osReadFile := func(name string) ([]byte, error) {
				return nil, nil
			}

			walker, fileListQueue := tc.Case()
			walker.osReadFile = osReadFile
			_ = walker.Start()

			c := 0
			for range fileListQueue {
				c++
			}

			if c != tc.Expected {
				t.Errorf("expected %v but got %v", tc.Expected, c)
			}
		})
	}
}

func TestNewFileWalkerBinary(t *testing.T) {
	type testcase struct {
		Name     string
		Case     func() (*FileWalker, chan *File)
		Expected int
	}

	testCases := []testcase{
		{
			Name: "Binary File 0",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "stuff")
				_ = os.Mkdir(d2, 0777)

				nullByte := []byte{0}
				_ = os.WriteFile(filepath.Join(d2, "null.txt"), nullByte, 0644)

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.IgnoreBinaryFiles = true
				return walker, fileListQueue
			},
			Expected: 0,
		},
		{
			Name: "Binary File 2",
			Case: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "stuff")
				_ = os.Mkdir(d2, 0777)

				d3 := filepath.Join(d2, "more_stuff")
				_ = os.Mkdir(d3, 0777)

				nullByte := []byte{0}
				_ = os.WriteFile(filepath.Join(d3, "null.txt"), nullByte, 0644)
				_ = os.WriteFile(filepath.Join(d3, "null2.txt"), nullByte, 0644)

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)

				walker.IgnoreBinaryFiles = true
				return walker, fileListQueue
			},
			Expected: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			osReadFile := func(name string) ([]byte, error) {
				return nil, nil
			}

			walker, fileListQueue := tc.Case()
			walker.osReadFile = osReadFile
			_ = walker.Start()

			c := 0
			for range fileListQueue {
				c++
			}

			if c != tc.Expected {
				t.Errorf("expected %v but got %v", tc.Expected, c)
			}
		})
	}
}

func TestGetExtension(t *testing.T) {
	got := GetExtension("something.c")
	expected := "c"

	if got != expected {
		t.Errorf("Expected %s got %s", expected, got)
	}
}

func TestGetExtensionNoExtension(t *testing.T) {
	got := GetExtension("something")
	expected := "something"

	if got != expected {
		t.Errorf("Expected %s got %s", expected, got)
	}
}

func TestGetExtensionMultipleDots(t *testing.T) {
	got := GetExtension(".travis.yml")
	expected := "yml"

	if got != expected {
		t.Errorf("Expected %s got %s", expected, got)
	}
}

func TestGetExtensionMultipleExtensions(t *testing.T) {
	got := GetExtension("something.go.yml")
	expected := "yml"

	if got != expected {
		t.Errorf("Expected %s got %s", expected, got)
	}
}

func TestGetExtensionStartsWith(t *testing.T) {
	got := GetExtension(".gitignore")
	expected := ".gitignore"

	if got != expected {
		t.Errorf("Expected %s got %s", expected, got)
	}
}

func TestGetExtensionTypeScriptDefinition(t *testing.T) {
	got := GetExtension("test.d.ts")
	expected := "ts"

	if got != expected {
		t.Errorf("Expected %s got %s", expected, got)
	}
}

func TestGetExtensionRegression(t *testing.T) {
	got := GetExtension("DeviceDescription.stories.tsx")
	expected := "tsx"

	if got != expected {
		t.Errorf("Expected %s got %s", expected, got)
	}
}

func TestSkipHandlerDefaultNoOp(t *testing.T) {
	d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
	_, _ = os.Create(filepath.Join(d, "test.txt"))

	fileListQueue := make(chan *File, 10)
	walker := NewFileWalker(d, fileListQueue)
	walker.ExcludeFilename = []string{"test.txt"}
	// no SetSkipHandler call — default no-op should work fine
	_ = walker.Start()

	count := 0
	for range fileListQueue {
		count++
	}

	if count != 0 {
		t.Error("Expected 0 files")
	}
}

func TestSkipHandlerDefaultNoOpParallel(t *testing.T) {
	d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
	_, _ = os.Create(filepath.Join(d, "test.txt"))

	fileListQueue := make(chan *File, 10)
	walker := NewParallelFileWalker([]string{d}, fileListQueue)
	walker.ExcludeFilename = []string{"test.txt"}
	_ = walker.Start()

	count := 0
	for range fileListQueue {
		count++
	}

	if count != 0 {
		t.Error("Expected 0 files")
	}
}

func TestSkipHandlerFileCases(t *testing.T) {
	type skipRecord struct {
		path   string
		name   string
		isDir  bool
		reason SkipReason
	}

	type testcase struct {
		Name           string
		Setup          func() (*FileWalker, chan *File)
		ExpectedSkips  int
		ExpectedReason SkipReason
		ExpectedIsDir  bool
	}

	testCases := []testcase{
		{
			Name: "ExcludeFilename skip",
			Setup: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "excluded.txt"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)
				walker.ExcludeFilename = []string{"excluded.txt"}
				return walker, fileListQueue
			},
			ExpectedSkips:  1,
			ExpectedReason: SkipReasonExcludeFilename,
			ExpectedIsDir:  false,
		},
		{
			Name: "IncludeFilename skip",
			Setup: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "other.txt"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)
				walker.IncludeFilename = []string{"wanted.txt"}
				return walker, fileListQueue
			},
			ExpectedSkips:  1,
			ExpectedReason: SkipReasonIncludeFilename,
			ExpectedIsDir:  false,
		},
		{
			Name: "ExcludeFilenameRegex skip",
			Setup: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.log"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)
				walker.ExcludeFilenameRegex = []*regexp.Regexp{regexp.MustCompile(`\.log$`)}
				return walker, fileListQueue
			},
			ExpectedSkips:  1,
			ExpectedReason: SkipReasonExcludeFilenameRegex,
			ExpectedIsDir:  false,
		},
		{
			Name: "IncludeFilenameRegex skip",
			Setup: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.txt"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)
				walker.IncludeFilenameRegex = []*regexp.Regexp{regexp.MustCompile(`\.go$`)}
				return walker, fileListQueue
			},
			ExpectedSkips:  1,
			ExpectedReason: SkipReasonIncludeFilenameRegex,
			ExpectedIsDir:  false,
		},
		{
			Name: "AllowListExtension skip",
			Setup: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)
				walker.AllowListExtensions = []string{"go"}
				return walker, fileListQueue
			},
			ExpectedSkips:  1,
			ExpectedReason: SkipReasonAllowListExtension,
			ExpectedIsDir:  false,
		},
		{
			Name: "ExcludeListExtension skip",
			Setup: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.txt"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)
				walker.ExcludeListExtensions = []string{"txt"}
				return walker, fileListQueue
			},
			ExpectedSkips:  1,
			ExpectedReason: SkipReasonExcludeListExtension,
			ExpectedIsDir:  false,
		},
		{
			Name: "LocationExcludePattern file skip",
			Setup: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.txt"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)
				walker.LocationExcludePattern = []string{"test.txt"}
				return walker, fileListQueue
			},
			ExpectedSkips:  1,
			ExpectedReason: SkipReasonLocationExcludePattern,
			ExpectedIsDir:  false,
		},
		{
			Name: "Binary file skip",
			Setup: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_ = os.WriteFile(filepath.Join(d, "binary.bin"), []byte{0}, 0644)

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)
				walker.IgnoreBinaryFiles = true
				return walker, fileListQueue
			},
			ExpectedSkips:  1,
			ExpectedReason: SkipReasonBinary,
			ExpectedIsDir:  false,
		},
		{
			Name: "CustomIgnorePatterns file skip",
			Setup: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				_, _ = os.Create(filepath.Join(d, "test.md"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)
				walker.CustomIgnorePatterns = []string{"*.md"}
				return walker, fileListQueue
			},
			ExpectedSkips:  1,
			ExpectedReason: SkipReasonCustomIgnore,
			ExpectedIsDir:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			walker, fileListQueue := tc.Setup()

			var skips []skipRecord
			walker.SetSkipHandler(func(path string, name string, isDir bool, reason SkipReason) {
				skips = append(skips, skipRecord{path: path, name: name, isDir: isDir, reason: reason})
			})
			walker.osReadFile = func(name string) ([]byte, error) { return nil, nil }
			_ = walker.Start()

			// drain channel
			for range fileListQueue {
			}

			if len(skips) != tc.ExpectedSkips {
				t.Errorf("expected %d skips but got %d", tc.ExpectedSkips, len(skips))
				return
			}

			if len(skips) > 0 {
				if skips[0].reason != tc.ExpectedReason {
					t.Errorf("expected reason %q but got %q", tc.ExpectedReason, skips[0].reason)
				}
				if skips[0].isDir != tc.ExpectedIsDir {
					t.Errorf("expected isDir=%v but got isDir=%v", tc.ExpectedIsDir, skips[0].isDir)
				}
			}
		})
	}
}

func TestSkipHandlerDirectoryCases(t *testing.T) {
	type skipRecord struct {
		path   string
		name   string
		isDir  bool
		reason SkipReason
	}

	type testcase struct {
		Name           string
		Setup          func() (*FileWalker, chan *File)
		ExpectedReason SkipReason
	}

	testCases := []testcase{
		{
			Name: "ExcludeDirectory skip",
			Setup: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "vendor")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "file.go"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)
				walker.ExcludeDirectory = []string{"vendor"}
				return walker, fileListQueue
			},
			ExpectedReason: SkipReasonExcludeDirectory,
		},
		{
			Name: "IncludeDirectory skip",
			Setup: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "other")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "file.go"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)
				walker.IncludeDirectory = []string{"wanted"}
				return walker, fileListQueue
			},
			ExpectedReason: SkipReasonIncludeDirectory,
		},
		{
			Name: "ExcludeDirectoryRegex skip",
			Setup: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "build")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "file.go"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)
				walker.ExcludeDirectoryRegex = []*regexp.Regexp{regexp.MustCompile("^build$")}
				return walker, fileListQueue
			},
			ExpectedReason: SkipReasonExcludeDirectoryRegex,
		},
		{
			Name: "IncludeDirectoryRegex skip",
			Setup: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "other")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "file.go"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)
				walker.IncludeDirectoryRegex = []*regexp.Regexp{regexp.MustCompile("^src$")}
				return walker, fileListQueue
			},
			ExpectedReason: SkipReasonIncludeDirectoryRegex,
		},
		{
			Name: "LocationExcludePattern directory skip",
			Setup: func() (*FileWalker, chan *File) {
				d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
				d2 := filepath.Join(d, "skipme")
				_ = os.Mkdir(d2, 0777)
				_, _ = os.Create(filepath.Join(d2, "file.go"))

				fileListQueue := make(chan *File, 10)
				walker := NewFileWalker(d, fileListQueue)
				walker.LocationExcludePattern = []string{"skipme"}
				return walker, fileListQueue
			},
			ExpectedReason: SkipReasonLocationExcludePattern,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			walker, fileListQueue := tc.Setup()

			var dirSkips []skipRecord
			walker.SetSkipHandler(func(path string, name string, isDir bool, reason SkipReason) {
				if isDir {
					dirSkips = append(dirSkips, skipRecord{path: path, name: name, isDir: isDir, reason: reason})
				}
			})
			walker.osReadFile = func(name string) ([]byte, error) { return nil, nil }
			_ = walker.Start()

			// drain channel
			for range fileListQueue {
			}

			if len(dirSkips) == 0 {
				t.Errorf("expected at least one directory skip but got none")
				return
			}

			if dirSkips[0].reason != tc.ExpectedReason {
				t.Errorf("expected reason %q but got %q", tc.ExpectedReason, dirSkips[0].reason)
			}
			if !dirSkips[0].isDir {
				t.Error("expected isDir=true")
			}
		})
	}
}

// TestSkipReasonAfterGitignoreNegationFile verifies that when a nested .gitignore
// un-ignores a file (negation pattern), the skipReason is not stale from the
// parent gitignore. If a subsequent filter then skips the file, skipReason must
// reflect that filter, not the earlier gitignore.
func TestSkipReasonAfterGitignoreNegationFile(t *testing.T) {
	type skipRecord struct {
		name   string
		isDir  bool
		reason SkipReason
	}

	t.Run("negated file not skipped", func(t *testing.T) {
		// root/.gitignore ignores *.log
		// root/sub/.gitignore un-ignores *.log via !*.log
		// root/sub/app.log should NOT be skipped
		d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
		_ = os.WriteFile(filepath.Join(d, ".gitignore"), []byte("*.log\n"), 0644)

		sub := filepath.Join(d, "sub")
		_ = os.Mkdir(sub, 0777)
		_ = os.WriteFile(filepath.Join(sub, ".gitignore"), []byte("!*.log\n"), 0644)
		_, _ = os.Create(filepath.Join(sub, "app.log"))

		fileListQueue := make(chan *File, 10)
		walker := NewFileWalker(d, fileListQueue)
		walker.IncludeHidden = true

		var skips []skipRecord
		walker.SetSkipHandler(func(path string, name string, isDir bool, reason SkipReason) {
			skips = append(skips, skipRecord{name: name, isDir: isDir, reason: reason})
		})
		_ = walker.Start()

		var files []string
		for f := range fileListQueue {
			files = append(files, f.Filename)
		}

		// app.log should appear in output — the negation un-ignored it
		found := false
		for _, f := range files {
			if f == "app.log" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected app.log to be included after gitignore negation, got files=%v skips=%v", files, skips)
		}

		// app.log should NOT appear in skips
		for _, s := range skips {
			if s.name == "app.log" {
				t.Errorf("app.log should not be skipped but was skipped with reason %q", s.reason)
			}
		}
	})

	t.Run("negated then excluded by later filter has correct reason", func(t *testing.T) {
		// root/.gitignore ignores *.log
		// root/sub/.gitignore un-ignores *.log via !*.log
		// ExcludeListExtensions catches .log files
		// skipReason must be ExcludeListExtension, NOT Gitignore
		d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
		_ = os.WriteFile(filepath.Join(d, ".gitignore"), []byte("*.log\n"), 0644)

		sub := filepath.Join(d, "sub")
		_ = os.Mkdir(sub, 0777)
		_ = os.WriteFile(filepath.Join(sub, ".gitignore"), []byte("!*.log\n"), 0644)
		_, _ = os.Create(filepath.Join(sub, "app.log"))

		fileListQueue := make(chan *File, 10)
		walker := NewFileWalker(d, fileListQueue)
		walker.ExcludeListExtensions = []string{"log"}
		walker.IncludeHidden = true

		var skips []skipRecord
		walker.SetSkipHandler(func(path string, name string, isDir bool, reason SkipReason) {
			skips = append(skips, skipRecord{name: name, isDir: isDir, reason: reason})
		})
		_ = walker.Start()
		for range fileListQueue {
		}

		// find the skip for app.log
		for _, s := range skips {
			if s.name == "app.log" {
				if s.reason != SkipReasonExcludeListExtension {
					t.Errorf("expected reason %q but got %q (stale gitignore reason leaked)", SkipReasonExcludeListExtension, s.reason)
				}
				return
			}
		}
		t.Error("expected app.log to be skipped by ExcludeListExtensions but it was not skipped at all")
	})

	t.Run("negated then excluded by ExcludeFilename has correct reason", func(t *testing.T) {
		// root/.gitignore ignores *.txt
		// root/sub/.gitignore un-ignores *.txt via !*.txt
		// ExcludeFilename catches test.txt
		// skipReason must be ExcludeFilename, NOT Gitignore
		d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
		_ = os.WriteFile(filepath.Join(d, ".gitignore"), []byte("*.txt\n"), 0644)

		sub := filepath.Join(d, "sub")
		_ = os.Mkdir(sub, 0777)
		_ = os.WriteFile(filepath.Join(sub, ".gitignore"), []byte("!*.txt\n"), 0644)
		_, _ = os.Create(filepath.Join(sub, "test.txt"))

		fileListQueue := make(chan *File, 10)
		walker := NewFileWalker(d, fileListQueue)
		walker.ExcludeFilename = []string{"test.txt"}
		walker.IncludeHidden = true

		var skips []skipRecord
		walker.SetSkipHandler(func(path string, name string, isDir bool, reason SkipReason) {
			skips = append(skips, skipRecord{name: name, isDir: isDir, reason: reason})
		})
		_ = walker.Start()
		for range fileListQueue {
		}

		for _, s := range skips {
			if s.name == "test.txt" && !s.isDir {
				if s.reason != SkipReasonExcludeFilename {
					t.Errorf("expected reason %q but got %q (stale gitignore reason leaked)", SkipReasonExcludeFilename, s.reason)
				}
				return
			}
		}
		t.Error("expected test.txt to be skipped by ExcludeFilename but it was not skipped at all")
	})
}

// TestSkipReasonAfterGitignoreNegationDirectory verifies the same stale-reason
// protection for directories: when a nested .gitignore un-ignores a directory,
// subsequent filters must report their own reason, not the earlier gitignore.
func TestSkipReasonAfterGitignoreNegationDirectory(t *testing.T) {
	type skipRecord struct {
		name   string
		isDir  bool
		reason SkipReason
	}

	t.Run("negated directory not skipped", func(t *testing.T) {
		// root/.gitignore ignores vendor/
		// root/sub/.gitignore un-ignores vendor/ via !vendor/
		// root/sub/vendor/ should NOT be skipped
		d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
		_ = os.WriteFile(filepath.Join(d, ".gitignore"), []byte("vendor/\n"), 0644)

		sub := filepath.Join(d, "sub")
		_ = os.Mkdir(sub, 0777)
		_ = os.WriteFile(filepath.Join(sub, ".gitignore"), []byte("!vendor/\n"), 0644)

		vendor := filepath.Join(sub, "vendor")
		_ = os.Mkdir(vendor, 0777)
		_, _ = os.Create(filepath.Join(vendor, "lib.go"))

		fileListQueue := make(chan *File, 10)
		walker := NewFileWalker(d, fileListQueue)
		walker.IncludeHidden = true

		var dirSkips []skipRecord
		walker.SetSkipHandler(func(path string, name string, isDir bool, reason SkipReason) {
			if isDir {
				dirSkips = append(dirSkips, skipRecord{name: name, isDir: isDir, reason: reason})
			}
		})
		_ = walker.Start()

		var files []string
		for f := range fileListQueue {
			files = append(files, f.Filename)
		}

		// lib.go should appear — vendor/ was un-ignored
		found := false
		for _, f := range files {
			if f == "lib.go" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected lib.go inside vendor/ after gitignore negation, got files=%v dirSkips=%v", files, dirSkips)
		}

		for _, s := range dirSkips {
			if s.name == "vendor" {
				t.Errorf("vendor/ should not be skipped but was skipped with reason %q", s.reason)
			}
		}
	})

	t.Run("negated then excluded by ExcludeDirectory has correct reason", func(t *testing.T) {
		// root/.gitignore ignores vendor/
		// root/sub/.gitignore un-ignores vendor/ via !vendor/
		// ExcludeDirectory catches vendor
		// skipReason must be ExcludeDirectory, NOT Gitignore
		d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
		_ = os.WriteFile(filepath.Join(d, ".gitignore"), []byte("vendor/\n"), 0644)

		sub := filepath.Join(d, "sub")
		_ = os.Mkdir(sub, 0777)
		_ = os.WriteFile(filepath.Join(sub, ".gitignore"), []byte("!vendor/\n"), 0644)

		vendor := filepath.Join(sub, "vendor")
		_ = os.Mkdir(vendor, 0777)
		_, _ = os.Create(filepath.Join(vendor, "lib.go"))

		fileListQueue := make(chan *File, 10)
		walker := NewFileWalker(d, fileListQueue)
		walker.ExcludeDirectory = []string{"vendor"}
		walker.IncludeHidden = true

		var dirSkips []skipRecord
		walker.SetSkipHandler(func(path string, name string, isDir bool, reason SkipReason) {
			if isDir {
				dirSkips = append(dirSkips, skipRecord{name: name, isDir: isDir, reason: reason})
			}
		})
		_ = walker.Start()
		for range fileListQueue {
		}

		for _, s := range dirSkips {
			if s.name == "vendor" {
				if s.reason != SkipReasonExcludeDirectory {
					t.Errorf("expected reason %q but got %q (stale gitignore reason leaked)", SkipReasonExcludeDirectory, s.reason)
				}
				return
			}
		}
		t.Error("expected vendor/ to be skipped by ExcludeDirectory but it was not skipped at all")
	})
}

func TestSkipHandlerNilIsIgnored(t *testing.T) {
	d, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
	_, _ = os.Create(filepath.Join(d, "test.txt"))

	fileListQueue := make(chan *File, 10)
	walker := NewFileWalker(d, fileListQueue)
	walker.ExcludeFilename = []string{"test.txt"}
	walker.SetSkipHandler(nil) // should be a no-op, keeping the default
	_ = walker.Start()

	count := 0
	for range fileListQueue {
		count++
	}

	if count != 0 {
		t.Error("Expected 0 files")
	}
}

func TestCRLFGitignore(t *testing.T) {
	dir := t.TempDir()

	content := "vendor/\r\n*.log\r\nbuild/\r\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	os.MkdirAll(filepath.Join(dir, "vendor", "pkg"), 0755)
	os.WriteFile(filepath.Join(dir, "vendor", "pkg", "lib.go"), []byte("package p"), 0644)
	os.WriteFile(filepath.Join(dir, "debug.log"), []byte("log"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	queue := make(chan *File, 100)
	walker := NewFileWalker(dir, queue)
	go walker.Start()

	var found []string
	for f := range queue {
		rel, _ := filepath.Rel(dir, f.Location)
		found = append(found, filepath.ToSlash(rel))
	}

	foundMain := false
	for _, p := range found {
		if p == "main.go" {
			foundMain = true
		}
		if strings.HasPrefix(p, "vendor/") {
			t.Errorf("vendor/ should be gitignored but got: %s", p)
		}
		if strings.HasSuffix(p, ".log") {
			t.Errorf("*.log should be gitignored but got: %s", p)
		}
	}
	if !foundMain {
		t.Error("expected main.go to be found but it was not")
	}
}

func TestWindowsPathNormalization(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("build/\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "build"), 0755)
	os.WriteFile(filepath.Join(dir, "build", "out.bin"), []byte("bin"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	queue := make(chan *File, 100)
	walker := NewFileWalker(dir, queue)
	go walker.Start()

	var found []string
	for f := range queue {
		rel, _ := filepath.Rel(dir, f.Location)
		found = append(found, filepath.ToSlash(rel))
	}

	foundMain := false
	for _, p := range found {
		if p == "main.go" {
			foundMain = true
		}
		if strings.HasPrefix(p, "build/") {
			t.Errorf("build/ should be gitignored but got: %s", p)
		}
	}
	if !foundMain {
		t.Error("expected main.go to be found but it was not")
	}
}
func TestGitInfoExclude(t *testing.T) {
	testDir, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
	_ = os.MkdirAll(filepath.Join(testDir, ".git", "info"), 0755)
	_ = os.WriteFile(filepath.Join(testDir, ".git", "info", "exclude"), []byte("secret.txt\n"), 0644)
	_, _ = os.Create(filepath.Join(testDir, "secret.txt"))
	_, _ = os.Create(filepath.Join(testDir, "visible.txt"))

	fileListQueue := make(chan *File, 10)
	walker := NewFileWalker(testDir, fileListQueue)
	walker.IgnoreGitIgnore = false
	_ = walker.Start()

	count := 0
	for range fileListQueue {
		count++
	}

	if count != 1 {
		t.Errorf("expected 1 file but got %d", count)
	}
}
func TestGitInfoExcludeNoGitDir(t *testing.T) {
	testDir, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
	_, _ = os.Create(filepath.Join(testDir, "visible.txt"))

	fileListQueue := make(chan *File, 10)
	walker := NewFileWalker(testDir, fileListQueue)
	walker.IgnoreGitIgnore = false
	_ = walker.Start()

	count := 0
	for range fileListQueue {
		count++
	}

	if count != 1 {
		t.Errorf("expected 1 file but got %d", count)
	}
}
func TestGitInfoExcludeIgnoredWhenGitIgnoreDisabled(t *testing.T) {
	testDir, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
	_ = os.MkdirAll(filepath.Join(testDir, ".git", "info"), 0755)
	_ = os.WriteFile(filepath.Join(testDir, ".git", "info", "exclude"), []byte("secret.txt\n"), 0644)
	_, _ = os.Create(filepath.Join(testDir, "secret.txt"))

	fileListQueue := make(chan *File, 10)
	walker := NewFileWalker(testDir, fileListQueue)
	walker.IgnoreGitIgnore = true
	_ = walker.Start()

	count := 0
	for range fileListQueue {
		count++
	}

	if count != 1 {
		t.Errorf("expected 1 file but got %d", count)
	}
}

// walkNames walks the supplied directory and returns the emitted file paths
// relative to it, sorted, so a test can state exactly what it expects to see.
func walkNames(t *testing.T, directory string) []string {
	t.Helper()

	fileListQueue := make(chan *File, 100)
	walker := NewFileWalker(directory, fileListQueue)
	go func() { _ = walker.Start() }()

	var found []string
	for f := range fileListQueue {
		rel, err := filepath.Rel(directory, f.Location)
		if err != nil {
			t.Fatal(err)
		}
		found = append(found, filepath.ToSlash(rel))
	}
	sort.Strings(found)
	return found
}

// TestGitInfoExcludeOnlyAppliesAtRepositoryRoot checks that the exclude file is
// still found and applied at a repository root, and that a directory below the
// root which happens to hold its own .git/info/exclude has that file applied to
// it too, since the walker treats any directory holding a .git entry as a root.
// This is the case the whole exclude branch exists for, so it must keep working
// now that the file is only looked for when a .git entry is actually present.
func TestGitInfoExcludeOnlyAppliesAtRepositoryRoot(t *testing.T) {
	testDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(testDir, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testDir, ".git", "info", "exclude"), []byte("secret.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(testDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"secret.txt", "keep.txt", "sub/secret.txt", "sub/keep.txt"} {
		if err := os.WriteFile(filepath.Join(testDir, filepath.FromSlash(name)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := walkNames(t, testDir)
	want := []string{"keep.txt", "sub/keep.txt"}
	if !slices.Equal(got, want) {
		t.Errorf("expected %v but got %v", want, got)
	}
}

// TestGitInfoExcludeWorktree covers a checkout made by git worktree add, where
// .git is a regular file pointing at a per worktree git directory whose info/
// actually lives in the common directory named by its commondir file. The
// walker used to join info/exclude onto the .git file itself, so the read
// always failed and a worktree silently ignored the exclude file of the
// repository it belongs to. It now follows the pointer the way git does.
func TestGitInfoExcludeWorktree(t *testing.T) {
	tmp := t.TempDir()

	// the main checkout, which owns info/exclude
	main := filepath.Join(tmp, "main")
	if err := os.MkdirAll(filepath.Join(main, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(main, ".git", "info", "exclude"), []byte("secret.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// the per worktree git directory, pointing back at the common directory
	worktreeGitDir := filepath.Join(main, ".git", "worktrees", "wt")
	if err := os.MkdirAll(worktreeGitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktreeGitDir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// the worktree checkout itself
	worktree := filepath.Join(tmp, "wt")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+worktreeGitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"secret.txt", "keep.txt"} {
		if err := os.WriteFile(filepath.Join(worktree, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := walkNames(t, worktree)
	want := []string{"keep.txt"}
	if !slices.Equal(got, want) {
		t.Errorf("expected %v but got %v", want, got)
	}
}

// TestGitInfoExcludeSubmodule covers a submodule checkout, where .git is a
// regular file naming a git directory by a relative path and that directory
// owns info/ directly, with no commondir indirection.
func TestGitInfoExcludeSubmodule(t *testing.T) {
	tmp := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmp, ".git", "modules", "vendored", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".git", "modules", "vendored", "info", "exclude"), []byte("secret.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	submodule := filepath.Join(tmp, "vendored")
	if err := os.MkdirAll(submodule, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(submodule, ".git"), []byte("gitdir: ../.git/modules/vendored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"secret.txt", "keep.txt"} {
		if err := os.WriteFile(filepath.Join(submodule, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := walkNames(t, submodule)
	want := []string{"keep.txt"}
	if !slices.Equal(got, want) {
		t.Errorf("expected %v but got %v", want, got)
	}
}

// TestGitInfoExcludeUnreadableGitFile checks that a .git file which is not a
// gitdir pointer does not break the walk or wrongly exclude anything.
func TestGitInfoExcludeUnreadableGitFile(t *testing.T) {
	testDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(testDir, ".git"), []byte("not a pointer at all\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := walkNames(t, testDir)
	want := []string{"keep.txt"}
	if !slices.Equal(got, want) {
		t.Errorf("expected %v but got %v", want, got)
	}
}

// TestGitInfoExcludeSymlinkedGitDir checks that a .git which is a symlink to a
// real git directory keeps working. A symlink reports as not a directory in the
// listing just like a worktree pointer file does, so it takes the gitdir parse
// path, fails to parse, and has to fall back to joining onto the link itself.
func TestGitInfoExcludeSymlinkedGitDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks on windows needs privileges we should not assume")
	}

	tmp := t.TempDir()

	realGitDir := filepath.Join(tmp, "realgit")
	if err := os.MkdirAll(filepath.Join(realGitDir, "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realGitDir, "info", "exclude"), []byte("secret.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	testDir := filepath.Join(tmp, "tree")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realGitDir, filepath.Join(testDir, ".git")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"secret.txt", "keep.txt"} {
		if err := os.WriteFile(filepath.Join(testDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := walkNames(t, testDir)
	want := []string{"keep.txt"}
	if !slices.Equal(got, want) {
		t.Errorf("expected %v but got %v", want, got)
	}
}

// TestGitInfoExcludeGitDirEnvAnchoredAtRoot checks that when $GIT_DIR is set the
// exclude file it names is anchored at the walk root only. It used to be read
// again in every directory and anchored at whichever directory was being walked,
// so a root anchored pattern such as /b.go excluded b.go at every level rather
// than only at the root.
func TestGitInfoExcludeGitDirEnvAnchoredAtRoot(t *testing.T) {
	tmp := t.TempDir()

	gitDir := filepath.Join(tmp, "gitdir")
	if err := os.MkdirAll(filepath.Join(gitDir, "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "info", "exclude"), []byte("/b.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	testDir := filepath.Join(tmp, "tree")
	if err := os.MkdirAll(filepath.Join(testDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"b.go", "keep.go", "sub/b.go"} {
		if err := os.WriteFile(filepath.Join(testDir, filepath.FromSlash(name)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("GIT_DIR", gitDir)

	got := walkNames(t, testDir)
	want := []string{"keep.go", "sub/b.go"}
	if !slices.Equal(got, want) {
		t.Errorf("expected %v but got %v", want, got)
	}
}

// TestGitInfoExcludeGitDirEnvOverridesLocal checks that $GIT_DIR still wins over
// a .git entry in the directory being walked, which is what it has always done.
func TestGitInfoExcludeGitDirEnvOverridesLocal(t *testing.T) {
	tmp := t.TempDir()

	gitDir := filepath.Join(tmp, "gitdir")
	if err := os.MkdirAll(filepath.Join(gitDir, "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "info", "exclude"), []byte("fromenv.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	testDir := filepath.Join(tmp, "tree")
	if err := os.MkdirAll(filepath.Join(testDir, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testDir, ".git", "info", "exclude"), []byte("fromlocal.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fromenv.txt", "fromlocal.txt", "keep.txt"} {
		if err := os.WriteFile(filepath.Join(testDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("GIT_DIR", gitDir)

	got := walkNames(t, testDir)
	want := []string{"fromlocal.txt", "keep.txt"}
	if !slices.Equal(got, want) {
		t.Errorf("expected %v but got %v", want, got)
	}
}

// TestSetConcurrencyIsHonoured guards against the walker sizing its semaphore
// from the package level default rather than the value the caller supplied,
// which silently ignored every call to SetConcurrency.
func TestSetConcurrencyIsHonoured(t *testing.T) {
	for _, want := range []int{1, 3, 64} {
		fileListQueue := make(chan *File, 10_000)
		curdir, _ := os.Getwd()
		walker := NewFileWalker(curdir, fileListQueue)
		walker.SetConcurrency(want)

		go func() { _ = walker.Start() }()
		for range fileListQueue {
		}

		if got := cap(walker.countingSemaphore); got != want {
			t.Errorf("SetConcurrency(%d) gave a semaphore of %d", want, got)
		}
	}
}

// TestSetConcurrencyBoundsWalkingGoroutines checks that the number of
// directories being walked at once never exceeds the configured concurrency,
// so that a pathological tree cannot produce unbounded goroutine growth.
func TestSetConcurrencyBoundsWalkingGoroutines(t *testing.T) {
	root := t.TempDir()
	// wide and shallow, so there is always more work available than there are
	// slots and the walker has every opportunity to over-fork
	for i := 0; i < 200; i++ {
		d := filepath.Join(root, "d"+strconv.Itoa(i), "nested")
		if err := os.MkdirAll(d, 0777); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(d, "f.txt"), "")
	}

	const concurrency = 4

	var mu sync.Mutex
	var inFlight, peak int
	fileListQueue := make(chan *File, 10_000)
	walker := NewFileWalker(root, fileListQueue)
	walker.SetConcurrency(concurrency)
	// the skip handler runs on the walking goroutines, so it is a cheap way to
	// sample how many of them are inside a directory at the same time
	walker.SetSkipHandler(func(string, string, bool, SkipReason) {})
	walker.osReadFile = func(name string) ([]byte, error) {
		mu.Lock()
		inFlight++
		if inFlight > peak {
			peak = inFlight
		}
		mu.Unlock()
		time.Sleep(time.Millisecond)
		mu.Lock()
		inFlight--
		mu.Unlock()
		return os.ReadFile(name)
	}
	// give the walker something to read in every directory so osReadFile is hit
	walker.CustomIgnore = []string{"f.txt"}

	go func() { _ = walker.Start() }()
	for range fileListQueue {
	}

	mu.Lock()
	got := peak
	mu.Unlock()

	if got > concurrency {
		t.Errorf("expected at most %d directories walked at once, saw %d", concurrency, got)
	}
}

// TestTerminateFromDepth checks that Terminate stops a walk promptly from any
// depth rather than only at the root, that it reports ErrTerminateWalk, and
// that it neither deadlocks on the output channel nor leaks goroutines.
func TestTerminateFromDepth(t *testing.T) {
	root := t.TempDir()
	// deep enough that the walk is well below the top level when we terminate
	dir := root
	for i := 0; i < 60; i++ {
		dir = filepath.Join(dir, "d"+strconv.Itoa(i))
		if err := os.MkdirAll(dir, 0777); err != nil {
			t.Fatal(err)
		}
		for j := 0; j < 20; j++ {
			writeFile(t, filepath.Join(dir, "f"+strconv.Itoa(j)+".txt"), "")
		}
	}

	before := runtime.NumGoroutine()

	fileListQueue := make(chan *File)
	walker := NewFileWalker(root, fileListQueue)
	walker.SetConcurrency(4)

	errChan := make(chan error, 1)
	go func() { errChan <- walker.Start() }()

	// read a few files then pull the plug from well inside the tree
	count := 0
	for range fileListQueue {
		count++
		if count == 5 {
			walker.Terminate()
		}
	}

	var err error
	select {
	case err = <-errChan:
	case <-time.After(30 * time.Second):
		t.Fatal("Start did not return after Terminate, the walk deadlocked")
	}

	if !errors.Is(err, ErrTerminateWalk) {
		t.Errorf("expected ErrTerminateWalk after Terminate, got %v", err)
	}

	if count == 60*20 {
		t.Error("expected the walk to stop early, but every file was emitted")
	}

	// the walking goroutines should all have unwound by the time Start returns
	for i := 0; i < 100; i++ {
		if runtime.NumGoroutine() <= before+2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("goroutines leaked after Terminate: %d before, %d after", before, runtime.NumGoroutine())
}

// TestParallelIgnoreInheritance checks that a directory is matched against
// exactly its own ancestors' ignore rules and never a sibling's.
//
// The ignore rules are accumulated with append and passed down the recursion.
// Appending to a slice that has spare capacity writes into the shared backing
// array, so once siblings are walked concurrently two of them can append their
// own .gitignore into the very same slot. The ancestor chain here is three
// deep on purpose: that leaves the slice at len 3 cap 4 by the time it reaches
// the siblings, which is exactly the case where the spare slot exists.
func TestParallelIgnoreInheritance(t *testing.T) {
	const siblings = 24

	for attempt := 0; attempt < 5; attempt++ {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, ".gitignore"), "never-match-one\n")
		a := filepath.Join(root, "a")
		b := filepath.Join(a, "b")
		if err := os.MkdirAll(b, 0777); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(a, ".gitignore"), "never-match-two\n")
		writeFile(t, filepath.Join(b, ".gitignore"), "never-match-three\n")

		// each sibling ignores a file only it has, so a sibling that ends up
		// applying another's rules emits a file it should have ignored
		for i := 0; i < siblings; i++ {
			s := filepath.Join(b, "s"+strconv.Itoa(i))
			if err := os.MkdirAll(s, 0777); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(s, ".gitignore"), "secret"+strconv.Itoa(i)+".txt\n")
			writeFile(t, filepath.Join(s, "secret"+strconv.Itoa(i)+".txt"), "")
			writeFile(t, filepath.Join(s, "keep.txt"), "")
		}

		fileListQueue := make(chan *File, 10_000)
		walker := NewFileWalker(root, fileListQueue)
		walker.SetConcurrency(16)
		go func() { _ = walker.Start() }()

		seen := map[string]bool{}
		for f := range fileListQueue {
			seen[filepath.ToSlash(f.Location)] = true
		}

		for i := 0; i < siblings; i++ {
			s := filepath.ToSlash(filepath.Join(b, "s"+strconv.Itoa(i)))
			if !seen[s+"/keep.txt"] {
				t.Fatalf("s%d/keep.txt was not emitted, it is not ignored by any rule", i)
			}
			if seen[s+"/secret"+strconv.Itoa(i)+".txt"] {
				t.Fatalf("s%d/secret%d.txt was emitted, so s%d did not see its own .gitignore", i, i, i)
			}
		}
	}
}

// TestZeroValueWalkerDoesNotDeadlock covers a FileWalker built as a struct
// literal rather than through a constructor, so semaphoreCount is 0. Sizing the
// semaphore straight from that would give an unbuffered channel that the root
// blocks on forever, so Start falls back to the package default instead.
func TestZeroValueWalkerDoesNotDeadlock(t *testing.T) {
	fileListQueue := make(chan *File, 10_000)
	curdir, _ := os.Getwd()
	walker := &FileWalker{
		fileListQueue: fileListQueue,
		directory:     curdir,
		errorsHandler: func(error) bool { return true },
		skipHandler:   func(string, string, bool, SkipReason) {},
		osOpen:        os.Open,
		osReadFile:    os.ReadFile,
		MaxDepth:      -1,
	}

	done := make(chan struct{})
	go func() {
		_ = walker.Start()
		close(done)
	}()

	count := 0
	for range fileListQueue {
		count++
	}

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("a zero value FileWalker deadlocked in Start")
	}

	if count == 0 {
		t.Error("expected to find at least one file")
	}
}

// TestWalkRelativeAndAbsoluteRootAgree pins the invariant the walk relies on to
// match ignore files cheaply: a path tested against a .gitignore is built by
// concatenating the resolved root, rather than resolved one path at a time, so
// a relative root and the absolute root it resolves to must walk to the same
// set of files. Nested .gitignore files matter here because a path deep in the
// tree is tested against every ignore file above it.
func TestWalkRelativeAndAbsoluteRootAgree(t *testing.T) {
	tmp := t.TempDir()

	for _, dir := range []string{"keep/sub", "drop", "nested/keep", "nested/skip"} {
		if err := os.MkdirAll(filepath.Join(tmp, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("drop/\n*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "nested", ".gitignore"), []byte("skip/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{
		"keep/a.rs", "keep/sub/b.rs", "keep/sub/c.log",
		"drop/d.rs", "nested/keep/e.rs", "nested/skip/f.rs",
	} {
		if err := os.WriteFile(filepath.Join(tmp, file), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	walk := func(root string) map[string]bool {
		t.Helper()
		fileListQueue := make(chan *File, 1000)
		walker := NewFileWalker(root, fileListQueue)
		if err := walker.Start(); err != nil {
			t.Fatal(err)
		}

		got := map[string]bool{}
		for f := range fileListQueue {
			rel, err := filepath.Rel(root, f.Location)
			if err != nil {
				t.Fatal(err)
			}
			got[filepath.ToSlash(rel)] = true
		}
		return got
	}

	absolute := walk(tmp)

	// The same tree reached by a relative root, which is what makes MatchIsDir
	// resolve rather than take the path as given.
	t.Chdir(filepath.Dir(tmp))
	relative := walk(filepath.Base(tmp))

	if !maps.Equal(absolute, relative) {
		t.Errorf("relative and absolute roots disagree:\n absolute %v\n relative %v", absolute, relative)
	}

	// Guard against both walks being identically wrong.
	for _, want := range []string{"keep/a.rs", "keep/sub/b.rs", "nested/keep/e.rs"} {
		if !absolute[want] {
			t.Errorf("expected %s to be walked, got %v", want, absolute)
		}
	}
	for _, notWant := range []string{"drop/d.rs", "keep/sub/c.log", "nested/skip/f.rs"} {
		if absolute[notWant] {
			t.Errorf("expected %s to be ignored, got %v", notWant, absolute)
		}
	}
}

// TestFileBatchQueueMatchesFileListQueue walks the same tree twice, once
// handing files over one at a time and once a directory at a time, and checks
// both produce exactly the same set of files. The batch queue is the walker's
// fast path, so what it must not do is change which files a walk finds.
func TestFileBatchQueueMatchesFileListQueue(t *testing.T) {
	tmp := t.TempDir()

	for _, dir := range []string{"keep/sub", "drop", "nested/keep", "nested/skip", "empty"} {
		if err := os.MkdirAll(filepath.Join(tmp, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("drop/\n*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "nested", ".gitignore"), []byte("skip/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{
		"root.rs", "keep/a.rs", "keep/sub/b.rs", "keep/sub/c.log",
		"drop/d.rs", "nested/keep/e.rs", "nested/skip/f.rs",
	} {
		if err := os.WriteFile(filepath.Join(tmp, file), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	single := map[string]bool{}
	queue := make(chan *File, 1000)
	walker := NewFileWalker(tmp, queue)
	go func() { _ = walker.Start() }()
	for f := range queue {
		single[f.Location] = true
	}

	batched := map[string]bool{}
	batchQueue := make(chan []*File, 1000)
	batchWalker := NewFileWalker(tmp, nil)
	batchWalker.SetFileBatchQueue(batchQueue)
	go func() { _ = batchWalker.Start() }()
	for b := range batchQueue {
		if len(b) == 0 {
			t.Error("an empty batch was sent, which is a wasted handover")
		}
		for _, f := range b {
			if batched[f.Location] {
				t.Errorf("%s was sent twice", f.Location)
			}
			batched[f.Location] = true
		}
	}

	if len(single) == 0 {
		t.Fatal("the per file walk found nothing, so the comparison proves nothing")
	}
	if !maps.Equal(single, batched) {
		t.Errorf("batched walk found a different set of files\nper file: %v\nbatched:  %v", single, batched)
	}
}

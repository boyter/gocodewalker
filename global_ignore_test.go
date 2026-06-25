// SPDX-License-Identifier: MIT

package gocodewalker

import (
	"os"
	"path/filepath"
	"testing"
)

// collectWalk runs a walker over dir with the supplied CustomIgnoreFiles and
// returns the set of emitted filenames keyed by their slash path relative to dir.
func collectWalk(t *testing.T, dir string, customIgnoreFiles []string) map[string]bool {
	t.Helper()
	fileListQueue := make(chan *File, 100)
	walker := NewFileWalker(dir, fileListQueue)
	walker.CustomIgnoreFiles = customIgnoreFiles
	go func() {
		if err := walker.Start(); err != nil {
			t.Errorf("walker returned error: %v", err)
		}
	}()

	got := map[string]bool{}
	for f := range fileListQueue {
		rel, _ := filepath.Rel(dir, filepath.FromSlash(f.Location))
		got[filepath.ToSlash(rel)] = true
	}
	return got
}

// writeFile creates parent directories then writes an (empty unless content) file.
func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestCustomIgnoreFilesRootAnchored verifies that a root-anchored pattern (/build)
// in a supplied global ignore file only matches at the walk root, not at every
// subdirectory, matching how a .gitignore sitting at the top of the tree behaves.
func TestCustomIgnoreFilesRootAnchored(t *testing.T) {
	root, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
	writeFile(t, filepath.Join(root, "build", "main.go"), "")
	writeFile(t, filepath.Join(root, "sub", "build", "main.go"), "")

	ignoreFile := filepath.Join(t.TempDir(), "global.ignore")
	writeFile(t, ignoreFile, "/build\n")

	got := collectWalk(t, root, []string{ignoreFile})

	if got["build/main.go"] {
		t.Errorf("expected root build/main.go to be ignored by /build, but it was emitted")
	}
	if !got["sub/build/main.go"] {
		t.Errorf("expected sub/build/main.go to be emitted (/build is root anchored), but it was ignored")
	}
}

// TestCustomIgnoreFilesNonAnchored verifies that a non-anchored pattern (*.log)
// in a supplied global ignore file matches at any depth.
func TestCustomIgnoreFilesNonAnchored(t *testing.T) {
	root, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
	writeFile(t, filepath.Join(root, "a.log"), "")
	writeFile(t, filepath.Join(root, "sub", "b.log"), "")
	writeFile(t, filepath.Join(root, "sub", "keep.go"), "")

	ignoreFile := filepath.Join(t.TempDir(), "global.ignore")
	writeFile(t, ignoreFile, "*.log\n")

	got := collectWalk(t, root, []string{ignoreFile})

	if got["a.log"] {
		t.Errorf("expected a.log to be ignored by *.log, but it was emitted")
	}
	if got["sub/b.log"] {
		t.Errorf("expected sub/b.log to be ignored by *.log at depth, but it was emitted")
	}
	if !got["sub/keep.go"] {
		t.Errorf("expected sub/keep.go to be emitted, but it was ignored")
	}
}

// TestCustomIgnoreFilesOverriddenByRepoGitignore verifies that an in-tree
// .gitignore negation (!keep.log) overrides the supplied global file, because
// discovered ignore files always win over the global ones.
func TestCustomIgnoreFilesOverriddenByRepoGitignore(t *testing.T) {
	root, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
	writeFile(t, filepath.Join(root, ".gitignore"), "!keep.log\n")
	writeFile(t, filepath.Join(root, "keep.log"), "")
	writeFile(t, filepath.Join(root, "other.log"), "")

	ignoreFile := filepath.Join(t.TempDir(), "global.ignore")
	writeFile(t, ignoreFile, "*.log\n")

	got := collectWalk(t, root, []string{ignoreFile})

	if !got["keep.log"] {
		t.Errorf("expected keep.log to be emitted (repo .gitignore !keep.log overrides global *.log), but it was ignored")
	}
	if got["other.log"] {
		t.Errorf("expected other.log to remain ignored by the global file, but it was emitted")
	}
}

// TestCustomIgnoreFilesLastSuppliedWins verifies that when multiple global ignore
// files are supplied, a later file overrides an earlier one (last supplied wins).
func TestCustomIgnoreFilesLastSuppliedWins(t *testing.T) {
	root, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
	writeFile(t, filepath.Join(root, "important.log"), "")
	writeFile(t, filepath.Join(root, "debug.log"), "")

	tmp := t.TempDir()
	fileA := filepath.Join(tmp, "a.ignore")
	fileB := filepath.Join(tmp, "b.ignore")
	writeFile(t, fileA, "*.log\n")
	writeFile(t, fileB, "!important.log\n")

	got := collectWalk(t, root, []string{fileA, fileB})

	if !got["important.log"] {
		t.Errorf("expected important.log to be emitted (later file b.ignore un-ignores it), but it was ignored")
	}
	if got["debug.log"] {
		t.Errorf("expected debug.log to remain ignored by a.ignore, but it was emitted")
	}
}

// TestCustomIgnoreFilesMissingPathTolerated verifies that a missing supplied path
// is tolerated by the error handler and the walk proceeds normally.
func TestCustomIgnoreFilesMissingPathTolerated(t *testing.T) {
	root, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
	writeFile(t, filepath.Join(root, "main.go"), "")

	missing := filepath.Join(t.TempDir(), "does-not-exist.ignore")

	got := collectWalk(t, root, []string{missing})

	if !got["main.go"] {
		t.Errorf("expected main.go to be emitted despite missing ignore file, but it was not")
	}
}

// TestCustomIgnoreFilesMissingPathErrorHandlerStops verifies that when the error
// handler returns false, a missing supplied path surfaces as an error.
func TestCustomIgnoreFilesMissingPathErrorHandlerStops(t *testing.T) {
	root, _ := os.MkdirTemp(os.TempDir(), randSeq(10))
	writeFile(t, filepath.Join(root, "main.go"), "")

	missing := filepath.Join(t.TempDir(), "does-not-exist.ignore")

	fileListQueue := make(chan *File, 100)
	walker := NewFileWalker(root, fileListQueue)
	walker.CustomIgnoreFiles = []string{missing}
	walker.SetErrorHandler(func(error) bool { return false })

	var walkErr error
	go func() {
		walkErr = walker.Start()
	}()
	for range fileListQueue {
	}

	if walkErr == nil {
		t.Errorf("expected an error when the error handler refuses a missing ignore file, got nil")
	}
}

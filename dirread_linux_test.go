// SPDX-License-Identifier: MIT
//go:build linux

package gocodewalker

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// entryShape is everything the walk actually asks an fs.DirEntry for, so it is
// what the getdents64 listing has to reproduce exactly.
type entryShape struct {
	Name  string
	IsDir bool
	Type  fs.FileMode
}

func shapes(t *testing.T, entries []fs.DirEntry) []entryShape {
	t.Helper()

	out := make([]entryShape, 0, len(entries))
	for _, e := range entries {
		out = append(out, entryShape{Name: e.Name(), IsDir: e.IsDir(), Type: e.Type()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return out
}

// readBothWays lists a directory with getdents64 and with the standard library
// and fails unless they agree.
func readBothWays(t *testing.T, directory string) []entryShape {
	t.Helper()

	walker := NewFileWalker(directory, make(chan *File, 1))

	walker.useRawDirents = true
	raw, err := walker.readDirectory(directory)
	if err != nil {
		t.Fatalf("getdents64 listing of %s: %v", directory, err)
	}

	walker.useRawDirents = false
	portable, err := walker.readDirectory(directory)
	if err != nil {
		t.Fatalf("standard library listing of %s: %v", directory, err)
	}

	got, want := shapes(t, raw), shapes(t, portable)
	if len(got) != len(want) {
		t.Fatalf("getdents64 found %d entries, standard library found %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d differs\n getdents64: %+v\n stdlib:     %+v", i, got[i], want[i])
		}
	}

	return want
}

// TestReadDirectoryMatchesStandardLibrary pins the getdents64 listing against
// the standard library one on the entry shapes that are easy to get wrong:
// symlinks must stay symlinks rather than become what they point at, and the
// d_type the kernel reports has to survive being turned into mode bits.
func TestReadDirectoryMatchesStandardLibrary(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "regular.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "subdir"), filepath.Join(dir, "link-to-dir")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "regular.txt"), filepath.Join(dir, "link-to-file")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "nowhere"), filepath.Join(dir, "link-broken")); err != nil {
		t.Fatal(err)
	}

	got := readBothWays(t, dir)
	if len(got) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(got))
	}

	// a symlink to a directory must not be reported as a directory, because the
	// walk decides whether to recurse from exactly this
	for _, e := range got {
		if strings.HasPrefix(e.Name, "link-") {
			if e.Type&fs.ModeSymlink == 0 {
				t.Errorf("%s: expected a symlink, got mode %v", e.Name, e.Type)
			}
			if e.IsDir {
				t.Errorf("%s: a symlink must not report itself as a directory", e.Name)
			}
		}
	}
}

// TestReadDirectoryOddNames covers names the dirent parser has to carry through
// byte for byte, including one that is not valid UTF-8 at all.
func TestReadDirectoryOddNames(t *testing.T) {
	dir := t.TempDir()

	names := []string{
		"with space.txt",
		"with\ttab.txt",
		"with'quote.txt",
		"with\"doublequote.txt",
		"with\\backslash.txt",
		"emoji-\U0001F600.txt",
		"chinese-中文.txt",
		"em—dash.txt",
		"bad\xff\xfename.txt",
		"-leading-dash.txt",
		".hidden",
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0600); err != nil {
			t.Fatalf("creating %q: %v", name, err)
		}
	}

	got := readBothWays(t, dir)
	if len(got) != len(names) {
		t.Fatalf("expected %d entries, got %d", len(names), len(got))
	}
}

// TestReadDirectoryBufferRefill walks a directory far larger than the dirent
// buffer, which is where a mistake in the record loop shows up: the buffer
// boundary is the one place a record can be split across two getdents64 calls.
func TestReadDirectoryBufferRefill(t *testing.T) {
	if testing.Short() {
		t.Skip("creating 20000 files is too slow for -short")
	}

	dir := t.TempDir()

	// names of a deliberately uneven length, so that records land on different
	// offsets relative to the end of the buffer rather than all on the same one
	const count = 20000
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("%0*d.txt", 3+(i%40), i)
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatalf("creating file %d: %v", i, err)
		}
	}

	got := readBothWays(t, dir)
	if len(got) != count {
		t.Fatalf("expected %d entries, got %d", count, len(got))
	}
}

// TestReadDirectoryErrorsMatch checks that a directory that cannot be listed
// fails the same way whichever listing is used, since the error text reaches
// consumers through SetErrorHandler.
func TestReadDirectoryErrorsMatch(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read a directory whatever its mode")
	}

	dir := t.TempDir()
	forbidden := filepath.Join(dir, "forbidden")
	if err := os.Mkdir(forbidden, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(forbidden, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(forbidden, 0755) })

	walker := NewFileWalker(dir, make(chan *File, 1))
	walker.SetErrorHandler(func(error) bool { return true })

	walker.useRawDirents = true
	_, rawErr := walker.readDirectory(forbidden)

	walker.useRawDirents = false
	_, portableErr := walker.readDirectory(forbidden)

	if rawErr == nil || portableErr == nil {
		t.Fatalf("expected both listings to fail, got %v and %v", rawErr, portableErr)
	}
	if rawErr.Error() != portableErr.Error() {
		t.Errorf("error text differs\n getdents64: %s\n stdlib:     %s", rawErr, portableErr)
	}
}

// TestRawDirentsDisabledWhenOpenReplaced makes sure a caller who supplies their
// own opener still gets it used, since getdents64 would bypass it entirely.
func TestRawDirentsDisabledWhenOpenReplaced(t *testing.T) {
	if !rawDirentsUsable(os.Open) {
		t.Fatal("expected the default opener to allow getdents64")
	}

	replaced := func(name string) (*os.File, error) { return os.Open(name) }
	if rawDirentsUsable(replaced) {
		t.Error("expected a replaced opener to force the standard library listing")
	}
	if rawDirentsUsable(nil) {
		t.Error("expected a nil opener to force the standard library listing")
	}
}

// BenchmarkReadDirectory guards the dirent buffer size. Reading a directory
// with getdents64 is only worth doing while it allocates no more than the
// standard library does: a larger buffer trades fewer syscalls for a pool that
// every garbage collection empties, and at 64 KiB that was measured at 70%
// more allocated and 8% slower. If a change to direntBufferSize makes the raw
// arm below allocate more than the portable one, that trade has gone the wrong
// way again.
func BenchmarkReadDirectory(b *testing.B) {
	dir := b.TempDir()
	for i := 0; i < 500; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("file%04d.go", i)), nil, 0600); err != nil {
			b.Fatal(err)
		}
	}

	walker := NewFileWalker(dir, make(chan *File, 1))

	for _, arm := range []struct {
		name string
		raw  bool
	}{
		{"getdents64", true},
		{"stdlib", false},
	} {
		walker.useRawDirents = arm.raw
		b.Run(arm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := walker.readDirectory(dir); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestDirentFileModeUnknown pins the one d_type the parser cannot answer by
// itself. Filesystems that report DT_UNKNOWN are rare enough that the common
// ones used for testing never exercise the fallback, so it is checked directly.
func TestDirentFileModeUnknown(t *testing.T) {
	if _, known := direntFileMode(0 /* DT_UNKNOWN */); known {
		t.Error("DT_UNKNOWN must not be reported as a known type, or the lstat fallback never runs")
	}

	for _, c := range []struct {
		dtype uint8
		mode  fs.FileMode
	}{
		{1 /* DT_FIFO */, fs.ModeNamedPipe},
		{2 /* DT_CHR */, fs.ModeDevice | fs.ModeCharDevice},
		{4 /* DT_DIR */, fs.ModeDir},
		{6 /* DT_BLK */, fs.ModeDevice},
		{8 /* DT_REG */, 0},
		{10 /* DT_LNK */, fs.ModeSymlink},
		{12 /* DT_SOCK */, fs.ModeSocket},
	} {
		got, known := direntFileMode(c.dtype)
		if !known || got != c.mode {
			t.Errorf("d_type %d: got mode %v known %v, want %v true", c.dtype, got, known, c.mode)
		}
	}
}

// TestLstatFileMode covers the DT_UNKNOWN fallback itself, including the entry
// that is deleted between being listed and being asked about, which must be
// skipped rather than fail the walk.
func TestLstatFileMode(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "regular.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "subdir"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name string
		mode fs.FileMode
	}{
		{"regular.txt", 0},
		{"subdir", fs.ModeDir},
		{"link", fs.ModeSymlink},
	} {
		mode, exists, err := lstatFileMode(dir, c.name)
		if err != nil || !exists {
			t.Fatalf("%s: exists %v err %v", c.name, exists, err)
		}
		if mode != c.mode {
			t.Errorf("%s: got mode %v, want %v", c.name, mode, c.mode)
		}
	}

	// an entry that has gone is not an error, it is simply not there any more
	_, exists, err := lstatFileMode(dir, "vanished.txt")
	if err != nil {
		t.Errorf("a missing entry must not be an error, got %v", err)
	}
	if exists {
		t.Error("expected a missing entry to report exists false")
	}
}

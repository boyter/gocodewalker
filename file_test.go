// SPDX-License-Identifier: MIT

package gocodewalker

import (
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestFindRepositoryRoot(t *testing.T) {
	// We expect this to walk back from file to root
	curdir, _ := os.Getwd()
	root := FindRepositoryRoot(curdir)

	if strings.HasSuffix(root, "file") {
		t.Error("Expected to walk back to root")
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

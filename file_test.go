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

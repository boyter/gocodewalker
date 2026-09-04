package gitignore_test

import (
	"strings"
	"testing"

	gitignore "github.com/boyter/gocodewalker/go-gitignore"
)

// benchmarks for the "any" ('**') matcher, which is the only path the
// trailing-"**" change touches.
func BenchmarkAnyMatch(b *testing.B) {
	_cases := []struct {
		Name    string
		Pattern string
		Path    string
		IsDir   bool
	}{
		// trailing "**": the case the change short circuits
		{"trailing_hit_shallow", "build/**", "build/x.o", false},
		{"trailing_hit_deep", "build/**", "build/a/b/c/d/e/f/g/x.o", false},
		{"trailing_miss_dir", "build/**", "build", true},
		{"trailing_miss_other", "build/**", "src/a/b/c/d/e/f/g/x.o", false},
		// leading "**": untouched, must not regress
		{"leading_hit_deep", "**/foo", "a/b/c/d/e/f/g/foo", false},
		{"leading_miss_deep", "**/foo", "a/b/c/d/e/f/g/bar", false},
		// embedded "**": untouched, the most recursive case
		{"embedded_hit", "a/**/b", "a/c/d/e/f/g/b", false},
		{"embedded_miss", "a/**/b", "a/c/d/e/f/g/z", false},
		// multiple "**", worst case for the recursion
		{"multi_miss", "a/**/b/**/c/**", "a/x/y/b/z/w/q/r/s/t", false},
	}

	for _, _case := range _cases {
		_ignore := gitignore.New(
			strings.NewReader(_case.Pattern+"\n"), "/base", nil,
		)
		b.Run(_case.Name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ignore.Relative(_case.Path, _case.IsDir)
			}
		})
	}
}

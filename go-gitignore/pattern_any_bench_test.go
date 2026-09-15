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
		// the "**" patterns the Kubernetes tree actually carries, against a
		// real path from that tree. These are the overwhelmingly common case
		// in practice: a miss on the first component, evaluated once per
		// pattern for every entry walked.
		{"k8s_hg_miss", "**/.hg*", "staging/src/k8s.io/api/core/v1/types.go", false},
		{"k8s_settings_miss", ".settings/**", "staging/src/k8s.io/api/core/v1/types.go", false},
		{"k8s_tmp_miss", "tmp/**/*", "staging/src/k8s.io/api/core/v1/types.go", false},
		{"k8s_idea_miss", ".idea/**", "staging/src/k8s.io/api/core/v1/types.go", false},
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

// BenchmarkAnyMatchRealWorld evaluates the full set of '**' patterns carried by
// the Kubernetes tree's .gitignore files against real paths from that tree,
// which is what the walker does for every entry it visits.
func BenchmarkAnyMatchRealWorld(b *testing.B) {
	_patterns := strings.Join([]string{
		".settings/**",
		"**/.hg",
		"**/.hg*",
		"tmp/**/*",
		".idea/**",
	}, "\n") + "\n"

	_paths := []string{
		"staging/src/k8s.io/api/core/v1/types.go",
		"pkg/kubelet/kubelet.go",
		"vendor/github.com/spf13/cobra/command.go",
		"test/e2e/framework/pod/wait.go",
		"LICENSE",
		"cluster/gce/config-default.sh",
	}

	_ignore := gitignore.New(strings.NewReader(_patterns), "/base", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, _path := range _paths {
			_ignore.Relative(_path, false)
		}
	}
}

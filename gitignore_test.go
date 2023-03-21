// SPDX-License-Identifier: MIT OR Unlicense

package gocodewalker

import (
	"github.com/boyter/gocodewalker/go-gitignore"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitIgnore(t *testing.T) {
	testcases := []string{
		`/`,
		`\`,
		`"`,
		`.`,
	}

	abs, _ := filepath.Abs(".")
	for _, te := range testcases {
		gitignore.New(strings.NewReader(te), abs, nil)
	}
}

func FuzzTestGitIgnore(f *testing.F) {
	testcases := []string{
		"",
		`\`,
		`'`,
		`#`,
		"/",
		"README.md",
		`README.md
/`,
		`*.[oa]
*.html
*.min.js

!foo*.html
foo-excl.html

vmlinux*

\!important!.txt

log/*.log
!/log/foo.log

**/logdir/log
**/foodir/bar
exclude/**

!findthis*

**/hide/**
subdir/subdir2/

/rootsubdir/

dirpattern/

README.md
`,
	}

	for _, tc := range testcases {
		f.Add(tc) // Use f.Add to provide a seed corpus
	}

	abs, _ := filepath.Abs(".")
	f.Fuzz(func(t *testing.T, c string) {
		gitignore.New(strings.NewReader(c), abs, nil)
	})
}

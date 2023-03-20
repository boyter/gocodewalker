package gitignore_test

import (
	"path/filepath"
	"strings"

	"github.com/denormal/go-gitignore"
)

type token struct {
	Type           gitignore.TokenType
	Name           string
	Token          string
	Line           int
	Column         int
	NewLine        int // token offset for newline end of line
	CarriageReturn int // token offset for carriage return end of line
} // token{}

type match struct {
	Path    string // test path
	Pattern string // matching pattern (if any)
	Ignore  bool   // whether the path is ignored or included
	Exclude bool   // whether the match comes from the GIT_DIR/info/exclude file
} // match{}

func (m match) Local() string {
	_path := m.Path
	if m.IsDir() {
		_path = strings.TrimSuffix(m.Path, "/")
	}

	// generate the local representation of the match path
	return filepath.Join(strings.Split(_path, "/")...)
} // Local()

func (m match) IsDir() bool {
	return strings.HasSuffix(m.Path, "/")
} // IsDir()

type position struct {
	File   string
	Line   int
	Column int
	Offset int
	String string
} // position{}

// define the constants for the unit tests
const (
	// define the example .gitignore file contents
	_GITIGNORE = `
# example .gitignore

!*.go

*.o
*.a

/ignore/this/path/

# the following line has trailing whitespace
/and/**/all/**/these/**  	 
!/but/not/this\ 

we support   spaces

/**/this.is.not/a ** valid/pattern
/**/nor/is/***/this
/nor/is***this
northis** 	 x

but \this\ is / valid\#
\

so	is this#
and this is #3 ok too
 / //
`

	// define the example .gitignore file contents for the Match tests
	// these tests have been taken from
	//		https://github.com/sdobz/backup/gitignore
	//
	// https://github.com/sdobz/backup/blob/master/gitignore/gitignore_test.go
	_GITMATCH = `

*.[oa]
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

# arch/foo/kernel/.gitignore
!arch/foo/kernel/vmlinux*

# htmldoc/.gitignore
!htmldoc/*.html

# git-sample-3/.gitignore
git-sample-3/*
!git-sample-3/foo
git-sample-3/foo/*
!git-sample-3/foo/bar

Documentation/*.pdf
Documentation/**/p*.pdf
`

	// define the number of good & bad patterns in the .gitignore above
	_GITPATTERNS    = 12
	_GITBADPATTERNS = 4

	// define the number of good & bad patterns in the match .gitignore above
	_GITMATCHPATTERNS    = 24
	_GITBADMATCHPATTERNS = 0

	// define the number of good and bad patterns returned when the
	// gitignore.Parser error handler returns false upon receiving an error
	_GITPATTERNSFALSE    = 7
	_GITBADPATTERNSFALSE = 1

	// define the base path for a git repository
	_GITBASE = "/my/git/repository"

	// define the directory mask for any directories created during testing
	_GITMASK = 0700

	// define a .gitignore that will trigger lexer errors
	_GITINVALID = "" +
		"# the following two lines will trigger repeated lexer errors\n" +
		"x\rx\rx\rx\n" +
		"\rx\rx\rx\n" +
		"!\rx\n" +
		"/my/valid/pattern\n" +
		"!\n" +
		"** *\n" +
		"/\r"

	// define the number of invalid patterns and errors
	_GITINVALIDERRORS        = 10
	_GITINVALIDERRORSFALSE   = 1
	_GITINVALIDPATTERNS      = 1
	_GITINVALIDPATTERNSFALSE = 0

	// define the expected number of errors during repository matching
	_GITREPOSITORYERRORS      = 38
	_GITREPOSITORYERRORSFALSE = 1

	// define a .gitignore file the contains just whitespace & comments
	_GITIGNORE_WHITESPACE = `
# this is an empty .gitignore file
#	- the following lines contains just whitespace
 
  		  	
`
)

var (
	// define the positions of the bad patterns
	_GITBADPOSITION = []gitignore.Position{
		gitignore.Position{File: "", Line: 17, Column: 19, Offset: 189},
		gitignore.Position{File: "", Line: 18, Column: 14, Offset: 219},
		gitignore.Position{File: "", Line: 19, Column: 8, Offset: 233},
		gitignore.Position{File: "", Line: 20, Column: 8, Offset: 248},
	}

	// define the positions of the good patterns
	_GITPOSITION = []gitignore.Position{
		gitignore.Position{File: "", Line: 4, Column: 1, Offset: 23},
		gitignore.Position{File: "", Line: 6, Column: 1, Offset: 30},
		gitignore.Position{File: "", Line: 7, Column: 1, Offset: 34},
		gitignore.Position{File: "", Line: 9, Column: 1, Offset: 39},
		gitignore.Position{File: "", Line: 12, Column: 1, Offset: 104},
		gitignore.Position{File: "", Line: 13, Column: 1, Offset: 132},
		gitignore.Position{File: "", Line: 15, Column: 1, Offset: 150},
		gitignore.Position{File: "", Line: 22, Column: 1, Offset: 256},
		gitignore.Position{File: "", Line: 23, Column: 1, Offset: 280},
		gitignore.Position{File: "", Line: 25, Column: 1, Offset: 283},
		gitignore.Position{File: "", Line: 26, Column: 1, Offset: 295},
		gitignore.Position{File: "", Line: 27, Column: 1, Offset: 317},
	}

	// define the token stream for the _GITIGNORE .gitignore
	_GITTOKENS = []token{
		// 1:
		{gitignore.EOL, "EOL", "\n", 1, 1, 0, 0},
		// 2: # example .gitignore contents
		{gitignore.COMMENT, "COMMENT", "# example .gitignore", 2, 1, 1, 2},
		{gitignore.EOL, "EOL", "\n", 2, 21, 21, 22},
		// 3:
		{gitignore.EOL, "EOL", "\n", 3, 1, 22, 24},
		// 4: !*.go
		{gitignore.NEGATION, "NEGATION", "!", 4, 1, 23, 26},
		{gitignore.PATTERN, "PATTERN", "*.go", 4, 2, 24, 27},
		{gitignore.EOL, "EOL", "\n", 4, 6, 28, 31},
		// 5:
		{gitignore.EOL, "EOL", "\n", 5, 1, 29, 33},
		// 6: *.o
		{gitignore.PATTERN, "PATTERN", "*.o", 6, 1, 30, 35},
		{gitignore.EOL, "EOL", "\n", 6, 4, 33, 38},
		// 7: *.a
		{gitignore.PATTERN, "PATTERN", "*.a", 7, 1, 34, 40},
		{gitignore.EOL, "EOL", "\n", 7, 4, 37, 43},
		// 8:
		{gitignore.EOL, "EOL", "\n", 8, 1, 38, 45},
		// 9: /ignore/this/path/
		{gitignore.SEPARATOR, "SEPARATOR", "/", 9, 1, 39, 47},
		{gitignore.PATTERN, "PATTERN", "ignore", 9, 2, 40, 48},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 9, 8, 46, 54},
		{gitignore.PATTERN, "PATTERN", "this", 9, 9, 47, 55},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 9, 13, 51, 59},
		{gitignore.PATTERN, "PATTERN", "path", 9, 14, 52, 60},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 9, 18, 56, 64},
		{gitignore.EOL, "EOL", "\n", 9, 19, 57, 65},
		// 10:
		{gitignore.EOL, "EOL", "\n", 10, 1, 58, 67},
		// 11: # the following line has trailing whitespace
		{gitignore.COMMENT, "COMMENT",
			"# the following line has trailing whitespace",
			11, 1, 59, 69},
		{gitignore.EOL, "EOL", "\n", 11, 45, 103, 113},
		// 12: /and/**/all/**/these/**
		{gitignore.SEPARATOR, "SEPARATOR", "/", 12, 1, 104, 115},
		{gitignore.PATTERN, "PATTERN", "and", 12, 2, 105, 116},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 12, 5, 108, 119},
		{gitignore.ANY, "ANY", "**", 12, 6, 109, 120},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 12, 8, 111, 122},
		{gitignore.PATTERN, "PATTERN", "all", 12, 9, 112, 123},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 12, 12, 115, 126},
		{gitignore.ANY, "ANY", "**", 12, 13, 116, 127},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 12, 15, 118, 129},
		{gitignore.PATTERN, "PATTERN", "these", 12, 16, 119, 130},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 12, 21, 124, 135},
		{gitignore.ANY, "ANY", "**", 12, 22, 125, 136},
		{gitignore.WHITESPACE, "WHITESPACE", "  \t ", 12, 24, 127, 138},
		{gitignore.EOL, "EOL", "\n", 12, 28, 131, 142},
		// 13: !/but/not/this\
		{gitignore.NEGATION, "NEGATION", "!", 13, 1, 132, 144},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 13, 2, 133, 145},
		{gitignore.PATTERN, "PATTERN", "but", 13, 3, 134, 146},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 13, 6, 137, 149},
		{gitignore.PATTERN, "PATTERN", "not", 13, 7, 138, 150},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 13, 10, 141, 153},
		{gitignore.PATTERN, "PATTERN", "this\\ ", 13, 11, 142, 154},
		{gitignore.EOL, "EOL", "\n", 13, 17, 148, 160},
		// 14:
		{gitignore.EOL, "EOL", "\n", 14, 1, 149, 162},
		// 15: we support   spaces
		{gitignore.PATTERN, "PATTERN", "we", 15, 1, 150, 164},
		{gitignore.WHITESPACE, "WHITESPACE", " ", 15, 3, 152, 166},
		{gitignore.PATTERN, "PATTERN", "support", 15, 4, 153, 167},
		{gitignore.WHITESPACE, "WHITESPACE", "   ", 15, 11, 160, 174},
		{gitignore.PATTERN, "PATTERN", "spaces", 15, 14, 163, 177},
		{gitignore.EOL, "EOL", "\n", 15, 20, 169, 183},
		// 16:
		{gitignore.EOL, "EOL", "\n", 16, 1, 170, 185},
		// 17: /**/this.is.not/a ** valid/pattern
		{gitignore.SEPARATOR, "SEPARATOR", "/", 17, 1, 171, 187},
		{gitignore.ANY, "ANY", "**", 17, 2, 172, 188},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 17, 4, 174, 190},
		{gitignore.PATTERN, "PATTERN", "this.is.not", 17, 5, 175, 191},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 17, 16, 186, 202},
		{gitignore.PATTERN, "PATTERN", "a", 17, 17, 187, 203},
		{gitignore.WHITESPACE, "WHITESPACE", " ", 17, 18, 188, 204},
		{gitignore.ANY, "ANY", "**", 17, 19, 189, 205},
		{gitignore.WHITESPACE, "WHITESPACE", " ", 17, 21, 191, 207},
		{gitignore.PATTERN, "PATTERN", "valid", 17, 22, 192, 208},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 17, 27, 197, 213},
		{gitignore.PATTERN, "PATTERN", "pattern", 17, 28, 198, 214},
		{gitignore.EOL, "EOL", "\n", 17, 35, 205, 221},
		// 18: /**/nor/is/***/this
		{gitignore.SEPARATOR, "SEPARATOR", "/", 18, 1, 206, 223},
		{gitignore.ANY, "ANY", "**", 18, 2, 207, 224},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 18, 4, 209, 226},
		{gitignore.PATTERN, "PATTERN", "nor", 18, 5, 210, 227},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 18, 8, 213, 230},
		{gitignore.PATTERN, "PATTERN", "is", 18, 9, 214, 231},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 18, 11, 216, 233},
		{gitignore.ANY, "ANY", "**", 18, 12, 217, 234},
		{gitignore.PATTERN, "PATTERN", "*", 18, 14, 219, 236},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 18, 15, 220, 237},
		{gitignore.PATTERN, "PATTERN", "this", 18, 16, 221, 238},
		{gitignore.EOL, "EOL", "\n", 18, 20, 225, 242},
		// 19: /nor/is***this
		{gitignore.SEPARATOR, "SEPARATOR", "/", 19, 1, 226, 244},
		{gitignore.PATTERN, "PATTERN", "nor", 19, 2, 227, 245},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 19, 5, 230, 248},
		{gitignore.PATTERN, "PATTERN", "is", 19, 6, 231, 249},
		{gitignore.ANY, "ANY", "**", 19, 8, 233, 251},
		{gitignore.PATTERN, "PATTERN", "*this", 19, 10, 235, 253},
		{gitignore.EOL, "EOL", "\n", 19, 15, 240, 258},
		// 20: northis** 	 x
		{gitignore.PATTERN, "PATTERN", "northis", 20, 1, 241, 260},
		{gitignore.ANY, "ANY", "**", 20, 8, 248, 267},
		{gitignore.WHITESPACE, "WHITESPACE", " \t ", 20, 10, 250, 269},
		{gitignore.PATTERN, "PATTERN", "x", 20, 13, 253, 272},
		{gitignore.EOL, "EOL", "\n", 20, 14, 254, 273},
		// 21:
		{gitignore.EOL, "EOL", "\n", 21, 1, 255, 275},
		// 22: but \this\ is / valid
		{gitignore.PATTERN, "PATTERN", "but", 22, 1, 256, 277},
		{gitignore.WHITESPACE, "WHITESPACE", " ", 22, 4, 259, 280},
		{gitignore.PATTERN, "PATTERN", "\\this\\ is", 22, 5, 260, 281},
		{gitignore.WHITESPACE, "WHITESPACE", " ", 22, 14, 269, 290},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 22, 15, 270, 291},
		{gitignore.WHITESPACE, "WHITESPACE", " ", 22, 16, 271, 292},
		{gitignore.PATTERN, "PATTERN", "valid\\#", 22, 17, 272, 293},
		{gitignore.EOL, "EOL", "\n", 22, 24, 279, 300},
		// 23: \
		{gitignore.PATTERN, "PATTERN", "\\", 23, 1, 280, 302},
		{gitignore.EOL, "EOL", "\n", 23, 2, 281, 303},
		// 24:
		{gitignore.EOL, "EOL", "\n", 24, 1, 282, 305},
		// 25: so is this#
		{gitignore.PATTERN, "PATTERN", "so", 25, 1, 283, 307},
		{gitignore.WHITESPACE, "WHITESPACE", "	", 25, 3, 285, 309},
		{gitignore.PATTERN, "PATTERN", "is", 25, 4, 286, 310},
		{gitignore.WHITESPACE, "WHITESPACE", " ", 25, 6, 288, 312},
		{gitignore.PATTERN, "PATTERN", "this#", 25, 7, 289, 313},
		{gitignore.EOL, "EOL", "\n", 25, 12, 294, 318},
		// 26: and this is #3 ok too
		{gitignore.PATTERN, "PATTERN", "and", 26, 1, 295, 320},
		{gitignore.WHITESPACE, "WHITESPACE", " ", 26, 4, 298, 323},
		{gitignore.PATTERN, "PATTERN", "this", 26, 5, 299, 324},
		{gitignore.WHITESPACE, "WHITESPACE", " ", 26, 9, 303, 328},
		{gitignore.PATTERN, "PATTERN", "is", 26, 10, 304, 329},
		{gitignore.WHITESPACE, "WHITESPACE", " ", 26, 12, 306, 331},
		{gitignore.PATTERN, "PATTERN", "#3", 26, 13, 307, 332},
		{gitignore.WHITESPACE, "WHITESPACE", " ", 26, 15, 309, 334},
		{gitignore.PATTERN, "PATTERN", "ok", 26, 16, 310, 335},
		{gitignore.WHITESPACE, "WHITESPACE", " ", 26, 18, 312, 337},
		{gitignore.PATTERN, "PATTERN", "too", 26, 19, 313, 338},
		{gitignore.EOL, "EOL", "\n", 26, 22, 316, 341},
		// 27: / //
		{gitignore.WHITESPACE, "WHITESPACE", " ", 27, 1, 317, 343},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 27, 2, 318, 344},
		{gitignore.WHITESPACE, "WHITESPACE", " ", 27, 3, 319, 345},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 27, 4, 320, 346},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 27, 5, 321, 347},
		{gitignore.EOL, "EOL", "\n", 27, 6, 322, 348},

		{gitignore.EOF, "EOF", "", 28, 1, 323, 350},
	}

	// define match tests and their expected results
	_GITMATCHES = []match{
		{"!important!.txt", "\\!important!.txt", true, false},
		{"arch/", "", false, false},
		{"arch/foo/", "", false, false},
		{"arch/foo/kernel/", "", false, false},
		{"arch/foo/kernel/vmlinux.lds.S", "!arch/foo/kernel/vmlinux*", false, false},
		{"arch/foo/vmlinux.lds.S", "vmlinux*", true, false},
		{"bar/", "", false, false},
		{"bar/testfile", "", false, false},
		{"dirpattern", "", false, false},
		{"my/other/path/to/dirpattern", "", false, false},
		{"my/path/to/dirpattern/", "dirpattern/", true, false},
		{"my/path/to/dirpattern/some_file.txt", "", false, false},
		{"Documentation/", "", false, false},
		{"Documentation/foo-excl.html", "foo-excl.html", true, false},
		{"Documentation/foo.html", "!foo*.html", false, false},
		{"Documentation/gitignore.html", "*.html", true, false},
		{"Documentation/test.a.html", "*.html", true, false},
		{"exclude/", "exclude/**", true, false},
		{"exclude/dir1/", "exclude/**", true, false},
		{"exclude/dir1/dir2/", "exclude/**", true, false},
		{"exclude/dir1/dir2/dir3/", "exclude/**", true, false},
		{"exclude/dir1/dir2/dir3/testfile", "exclude/**", true, false},
		{"exclude/other_file.txt", "exclude/**", true, false},
		{"file.o", "*.[oa]", true, false},
		{"foo/exclude/some_file.txt", "", false, false},
		{"foo/exclude/other/file.txt", "", false, false},
		{"foodir/", "", false, false},
		{"foodir/bar/", "**/foodir/bar", true, false},
		{"foodir/bar/testfile", "", false, false},
		{"git-sample-3/", "", false, false},
		{"git-sample-3/foo/", "!git-sample-3/foo", false, false},
		{"git-sample-3/foo/bar/", "!git-sample-3/foo/bar", false, false},
		{"git-sample-3/foo/test/", "git-sample-3/foo/*", true, false},
		{"git-sample-3/test/", "git-sample-3/*", true, false},
		{"htmldoc/", "", false, false},
		{"htmldoc/docs.html", "!htmldoc/*.html", false, false},
		{"htmldoc/jslib.min.js", "*.min.js", true, false},
		{"lib.a", "*.[oa]", true, false},
		{"log/", "", false, false},
		{"log/foo.log", "!/log/foo.log", false, false},
		{"log/test.log", "log/*.log", true, false},
		{"rootsubdir/", "/rootsubdir/", true, false},
		{"rootsubdir/foo", "", false, false},
		{"src/", "", false, false},
		{"src/findthis.o", "!findthis*", false, false},
		{"src/internal.o", "*.[oa]", true, false},
		{"subdir/", "", false, false},
		{"subdir/hide/", "**/hide/**", true, false},
		{"subdir/hide/foo", "**/hide/**", true, false},
		{"subdir/logdir/", "", false, false},
		{"subdir/logdir/log/", "**/logdir/log", true, false},
		{"subdir/logdir/log/findthis.log", "!findthis*", false, false},
		{"subdir/logdir/log/foo.log", "", false, false},
		{"subdir/logdir/log/test.log", "", false, false},
		{"subdir/rootsubdir/", "", false, false},
		{"subdir/rootsubdir/foo", "", false, false},
		{"subdir/subdir2/", "subdir/subdir2/", true, false},
		{"subdir/subdir2/bar", "", false, false},
		{"README.md", "README.md", true, false},
		{"my-path/README.md", "README.md", true, false},
		{"my-path/also/README.md", "README.md", true, false},
		{"Documentation/git.pdf", "Documentation/*.pdf", true, false},
		{"Documentation/ppc/ppc.pdf", "Documentation/**/p*.pdf", true, false},
		{"tools/perf/Documentation/perf.pdf", "", false, false},
	}

	// define the cache tests
	_CACHETEST = map[string]gitignore.GitIgnore{
		"a":     null(),
		"a/b":   null(),
		"a/b/c": nil,
	}

	// define a set of cache keys known not to be in the cache tests above
	_CACHEUNKNOWN = []string{
		"b",
		"b/c",
	}

	// define the set of .gitignore files for a repository
	_GITREPOSITORY = map[string]string{
		// define the top-level .gitignore file
		"": `
# ignore .bak files
*.bak
`,
		// define subdirectory .gitignore files
		"a": `
# ignore .go files
*.go

# ignore every c directory
#	- this should be the same as c/
**/c/
`,
		"a/b": `
# include .go files in this directory
!*.go

# include everything under e
!**/e/**
`,
		"a/b/d": `
# include c directories
!c/
hidden/
`,
	}

	// define the patterns for $GIT_DIR/info/exclude
	_GITEXCLUDE = `
# exclude every file using 'exclude' in its name
*exclude*
`

	// define repository match tests and their expected results
	_REPOSITORYMATCHES = []match{
		// matching against the nested .gitignore files
		{"include.go", "", false, false},
		{"ignore.go.bak", "*.bak", true, false},
		{"a/ignore.go", "*.go", true, false},
		{"a/ignore.go.bak", "*.bak", true, false},
		{"a/include.sh", "", false, false},
		{"a/c/ignore.go", "**/c/", true, false},
		{"a/c/ignore.go.bak", "**/c/", true, false},
		{"a/c/ignore.sh", "**/c/", true, false},
		{"a/c/", "**/c/", true, false},
		{"a/b/c/d/ignore.go", "**/c/", true, false},
		{"a/b/c/d/ignore.go.bak", "**/c/", true, false},
		{"a/b/c/d/ignore.sh", "**/c/", true, false},
		{"a/b/c/d/", "**/c/", true, false},
		{"a/b/c/", "**/c/", true, false},
		{"a/b/include.go", "!*.go", false, false},
		{"a/b/ignore.go.bak", "*.bak", true, false},
		{"a/b/include.sh", "", false, false},
		{"a/b/d/include.go", "!*.go", false, false},
		{"a/b/d/ignore.go.bak", "*.bak", true, false},
		{"a/b/d/include.sh", "", false, false},
		{"a/b/d/c/", "!c/", false, false},
		{"a/b/d/c/include.go", "!*.go", false, false},
		{"a/b/d/c/ignore.go.bak", "*.bak", true, false},
		{"a/b/d/c/include.sh", "", false, false},
		{"a/b/e/c/", "!**/e/**", false, false},
		{"a/b/e/c/include.go", "!**/e/**", false, false},
		{"a/b/e/c/include.go.bak", "!**/e/**", false, false},
		{"a/b/e/c/include.sh", "!**/e/**", false, false},

		// matching against GIT_DIR/info/exclude
		{"exclude.me", "*exclude*", true, true},
		{"a/exclude.me", "*exclude*", true, true},
		{"a/b/exclude.me", "*exclude*", true, true},
		{"a/b/c/exclude.me", "**/c/", true, false},
		{"a/b/c/d/exclude.me", "**/c/", true, false},
		{"a/c/exclude.me", "**/c/", true, false},
		{"a/b/exclude.me", "*exclude*", true, true},
		{"a/b/d/exclude.me", "*exclude*", true, true},
		{"a/b/d/c/exclude.me", "*exclude*", true, true},
		{"a/b/e/c/exclude.me", "!**/e/**", false, false},
	}

	// define the repository match tests and their expected results when the
	// error handler returns false
	_REPOSITORYMATCHESFALSE = []match{
		{"a/b/c_/d/e_/f/g/h/include.go~", "", false, false},
	}

	// define the position tests
	_POSITIONS = []position{
		{"", 0, 0, 0, "+0"},
		{"", 1, 0, 0, "1"},
		{"", 0, 1, 0, "+0"},
		{"", 0, 0, 1, "+1"},
		{"", 1, 2, 0, "1:2"},
		{"", 1, 0, 3, "1"},
		{"", 1, 2, 3, "1:2"},
		{"file", 0, 0, 0, "file: +0"},
		{"file", 1, 0, 0, "file: 1"},
		{"file", 0, 1, 0, "file: +0"},
		{"file", 0, 0, 1, "file: +1"},
		{"file", 1, 2, 0, "file: 1:2"},
		{"file", 1, 0, 3, "file: 1"},
		{"file", 1, 2, 3, "file: 1:2"},
	}

	// define the token tests
	//		- we us the same position for all tokens, and ignore the
	//		  token string (i.e. the sequence of runes that comprise this
	//		  token), since we test the correctness of rune mappings to toknes
	//	      in the above tests of example .gitignore files
	_TOKENS = []token{
		{gitignore.ILLEGAL, "ILLEGAL", "", 1, 2, 3, 4},
		{gitignore.EOF, "EOF", "", 1, 2, 3, 4},
		{gitignore.EOL, "EOL", "", 1, 2, 3, 4},
		{gitignore.WHITESPACE, "WHITESPACE", "", 1, 2, 3, 4},
		{gitignore.COMMENT, "COMMENT", "", 1, 2, 3, 4},
		{gitignore.SEPARATOR, "SEPARATOR", "", 1, 2, 3, 4},
		{gitignore.NEGATION, "NEGATION", "", 1, 2, 3, 4},
		{gitignore.PATTERN, "PATTERN", "", 1, 2, 3, 4},
		{gitignore.ANY, "ANY", "", 1, 2, 3, 4},
		{gitignore.BAD, "BAD TOKEN", "", 1, 2, 3, 4},

		// invalid tokens
		{-1, "BAD TOKEN", "", 1, 2, 3, 4},
		{12345, "BAD TOKEN", "", 1, 2, 3, 4},
	}

	// define the beginning position for the parser & lexer
	_BEGINNING = gitignore.Position{File: "", Line: 1, Column: 1, Offset: 0}

	// define the tokens from the invalid .gitignore above
	_TOKENSINVALID = []token{
		// 1: # the following two lines will trigger repeated lexer errors
		{gitignore.COMMENT,
			"COMMENT",
			"# the following two lines will trigger repeated lexer errors",
			1, 1, 0, 0},
		{gitignore.EOL, "EOL", "\n", 1, 61, 60, 60},
		// 2: x\rx\rx\rx
		{gitignore.PATTERN, "PATTERN", "x", 2, 1, 61, 62},
		{gitignore.BAD, "BAD TOKEN", "\r", 2, 2, 62, 63},
		{gitignore.PATTERN, "PATTERN", "x", 2, 3, 63, 64},
		{gitignore.BAD, "BAD TOKEN", "\r", 2, 4, 64, 65},
		{gitignore.PATTERN, "PATTERN", "x", 2, 5, 65, 66},
		{gitignore.BAD, "BAD TOKEN", "\r", 2, 6, 66, 67},
		{gitignore.PATTERN, "PATTERN", "x", 2, 7, 67, 68},
		{gitignore.EOL, "EOL", "\n", 2, 8, 68, 69},
		// 3: x\rx\rx\rx
		{gitignore.BAD, "BAD TOKEN", "\r", 3, 1, 69, 71},
		{gitignore.PATTERN, "PATTERN", "x", 3, 2, 70, 72},
		{gitignore.BAD, "BAD TOKEN", "\r", 3, 3, 71, 73},
		{gitignore.PATTERN, "PATTERN", "x", 3, 4, 72, 74},
		{gitignore.BAD, "BAD TOKEN", "\r", 3, 5, 73, 75},
		{gitignore.PATTERN, "PATTERN", "x", 3, 6, 74, 76},
		{gitignore.EOL, "EOL", "\n", 3, 7, 75, 77},
		// 4: !\rx
		{gitignore.NEGATION, "NEGATION", "!", 4, 1, 76, 79},
		{gitignore.BAD, "BAD TOKEN", "\r", 4, 2, 77, 80},
		{gitignore.PATTERN, "PATTERN", "x", 4, 3, 78, 81},
		{gitignore.EOL, "EOL", "\n", 4, 4, 79, 82},
		// 5: /my/valid/pattern
		{gitignore.SEPARATOR, "SEPARATOR", "/", 5, 1, 80, 84},
		{gitignore.PATTERN, "PATTERN", "my", 5, 2, 81, 85},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 5, 4, 83, 87},
		{gitignore.PATTERN, "PATTERN", "valid", 5, 5, 84, 88},
		{gitignore.SEPARATOR, "SEPARATOR", "/", 5, 10, 89, 93},
		{gitignore.PATTERN, "PATTERN", "pattern", 5, 11, 90, 94},
		{gitignore.EOL, "EOL", "\n", 5, 18, 97, 101},
		// 6: !
		{gitignore.NEGATION, "NEGATION", "!", 6, 1, 98, 103},
		{gitignore.EOL, "EOL", "\n", 6, 2, 99, 104},
		// 7: ** *
		{gitignore.ANY, "ANY", "**", 7, 1, 100, 106},
		{gitignore.WHITESPACE, "WHITESPACE", " ", 7, 3, 102, 108},
		{gitignore.PATTERN, "PATTERN", "*", 7, 4, 103, 109},
		{gitignore.EOL, "EOL", "\n", 7, 5, 104, 110},
		// 8: /\r
		{gitignore.SEPARATOR, "SEPARATOR", "/", 8, 1, 105, 112},
		{gitignore.BAD, "BAD TOKEN", "\r", 8, 2, 106, 113},

		{gitignore.EOF, "EOF", "", 8, 3, 107, 114},
	}

	// define the patterns & errors expected during invalid content parsing
	_GITINVALIDPATTERN = []string{"/my/valid/pattern"}
	_GITINVALIDERROR   = []error{
		gitignore.CarriageReturnError,
		gitignore.CarriageReturnError,
		gitignore.CarriageReturnError,
		gitignore.CarriageReturnError,
		gitignore.CarriageReturnError,
		gitignore.CarriageReturnError,
		gitignore.CarriageReturnError,
		gitignore.InvalidPatternError,
		gitignore.InvalidPatternError,
		gitignore.CarriageReturnError,
	}
)

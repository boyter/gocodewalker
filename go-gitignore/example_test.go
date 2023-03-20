package gitignore_test

import (
	"fmt"

	"github.com/denormal/go-gitignore"
)

func ExampleNewFromFile() {
	ignore, err := gitignore.NewFromFile("/my/project/.gitignore")
	if err != nil {
		panic(err)
	}

	// attempt to match an absolute path
	match := ignore.Match("/my/project/src/file.go")
	if match != nil {
		if match.Ignore() {
			fmt.Println("ignore file.go")
		}
	}

	// attempt to match a relative path
	//		- this is equivalent to the call above
	match = ignore.Relative("src/file.go", false)
	if match != nil {
		if match.Include() {
			fmt.Println("include file.go")
		}
	}
} // ExampleNewFromFile()

func ExampleNewRepository() {
	ignore, err := gitignore.NewRepository("/my/project")
	if err != nil {
		panic(err)
	}

	// attempt to match a directory in the repository
	match := ignore.Relative("src/examples", true)
	if match != nil {
		if match.Ignore() {
			fmt.Printf(
				"ignore src/examples because of pattern %q at %s",
				match, match.Position(),
			)
		}
	}

	// if we have an absolute path, or a path relative to the current
	// working directory we can use the short-hand methods
	if ignore.Include("/my/project/etc/service.conf") {
		fmt.Println("include the service configuration")
	}
} // ExampleNewRepository()

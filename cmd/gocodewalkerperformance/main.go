// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"github.com/boyter/gocodewalker"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: gocodewalkerperformance <path>")
		os.Exit(1)
	}

	// useful for improving performance, then go tool pprof -http=localhost:8090 profile.pprof
	//f, _ := os.Create("profile.pprof")
	//_ = pprof.StartCPUProfile(f)
	//defer pprof.StopCPUProfile()

	fileListQueue := make(chan *gocodewalker.File, 100)
	fileWalker := gocodewalker.NewFileWalker(os.Args[1], fileListQueue)

	// handle the error by printing it out and terminating the walker and returning
	// false which should cause continued processing to error
	errorHandler := func(e error) bool {
		fmt.Println("ERR", e.Error())
		return true
	}
	fileWalker.SetErrorHandler(errorHandler)

	go func() {
		err := fileWalker.Start()
		if err != nil {
			fmt.Println("ERROR", err.Error())
		}
	}()

	count := 0
	for f := range fileListQueue {
		fmt.Println(f.Location)
		count++
	}
	fmt.Println(count)
}

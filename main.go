package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/burke/treediff/diff"
)

const parallelism = 8

func main() {
	runtime.GOMAXPROCS(parallelism)

	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <dir> <dir>", os.Args[0])
		os.Exit(1)
	}

	changes, err := diff.Changes(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "treediff encountered an error: %s\n", err)
		os.Exit(1)
	}
	for _, chg := range changes {
		fmt.Printf("%s\n", chg)
	}
}

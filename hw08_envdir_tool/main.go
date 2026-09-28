package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: %s /path/to/env/dir command [args...]\n", filepath.Base(os.Args[0]))
		os.Exit(exitCodeInternal)
	}

	env, err := ReadDir(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "envdir:", err)
		os.Exit(exitCodeInternal)
	}

	os.Exit(RunCmd(os.Args[2:], env))
}

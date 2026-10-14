package main

import (
	"fmt"
	"os"

	"github.com/dogu-dev/dogu/pkg/cli"
)

func main() {
	if len(os.Args) < 2 {
		cli.PrintUsage(os.Stderr)
		os.Exit(1)
	}

	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "dogu error: %v\n", err)
		os.Exit(1)
	}
}

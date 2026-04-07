package main

import (
	"fmt"
	"os"

	"github.com/mishankov/updtr/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.New(version, os.Stdout, os.Stderr).Execute(); err != nil {
		if !cli.IsSilentExit(err) {
			_, _ = fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

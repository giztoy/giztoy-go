package main

import (
	"os"

	"github.com/giztoy/giztoy-go/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

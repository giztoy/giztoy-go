package main

import (
	"os"

	"github.com/GizClaw/gizclaw-go/cmd/internal/commands"
)

func main() {
	if err := commands.New().Execute(); err != nil {
		os.Exit(1)
	}
}

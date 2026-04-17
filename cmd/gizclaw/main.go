package main

import (
	"fmt"
	"io"
	"os"

	"github.com/GizClaw/gizclaw-go/cmd/internal/commands"
	"github.com/GizClaw/gizclaw-go/cmd/internal/service"
	"github.com/GizClaw/gizclaw-go/cmd/internal/servicerun"
)

var runWorkspaceService = servicerun.RunWorkspaceService

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		os.Exit(1)
	}
}

func run(args []string, stderr io.Writer) error {
	if workspace, ok, err := service.RuntimeWorkspaceFromArgs(args); err != nil {
		fmt.Fprintln(stderr, err)
		return err
	} else if ok {
		if err := runWorkspaceService(workspace); err != nil {
			fmt.Fprintln(stderr, err)
			return err
		}
		return nil
	}
	return commands.New().Execute()
}

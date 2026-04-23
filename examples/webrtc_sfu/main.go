package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
)

type config struct {
	addr string
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse args failed: %v\n", err)
		os.Exit(2)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	app, err := newApp(logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create app failed: %v\n", err)
		os.Exit(1)
	}

	logger.Info("starting webrtc sfu example", "addr", cfg.addr)
	if err := http.ListenAndServe(cfg.addr, app); err != nil {
		fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
		os.Exit(1)
	}
}

func parseConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("webrtc_sfu", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := config{
		addr: ":8088",
	}
	fs.StringVar(&cfg.addr, "addr", cfg.addr, "HTTP listen address")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if cfg.addr == "" {
		return config{}, fmt.Errorf("addr is required")
	}
	if len(fs.Args()) != 0 {
		return config{}, fmt.Errorf("unexpected positional args: %v", fs.Args())
	}
	return cfg, nil
}

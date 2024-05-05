package main

import (
	"context"
	"flag"
	"log/slog"
	"os/signal"

	"github.com/YutaroHayakawa/go-radv"
	"github.com/YutaroHayakawa/go-radv/cmd/internal"

	"golang.org/x/sys/unix"
)

func main() {
	configFile := flag.String("f", "", "config file path")

	flag.Parse()

	if *configFile == "" {
		slog.Error("Config file path is required. Aborting.")
		return
	}

	config, err := radv.ParseConfigYAMLFile(*configFile)
	if err != nil {
		slog.Error("Failed to parse config file. Aborting.", "error", err.Error())
		return
	}

	daemon, err := radv.NewDaemon(
		config,
		radv.WithLogger(slog.With("component", "daemon")),
	)
	if err != nil {
		slog.Error("Failed to create daemon. Aborting.", "error", err.Error())
		return
	}

	go func() {
		server := internal.NewServer("localhost:8888", daemon, slog.With("component", "apiServer"))

		slog.Info("Starting HTTP server")

		if err := server.ListenAndServe(); err != nil {
			slog.Error("HTTP server failed with error", "error", err.Error())
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), unix.SIGINT, unix.SIGTERM)
	daemon.Run(ctx)
	cancel()
}

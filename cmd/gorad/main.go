// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of go-ra

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os/signal"

	"github.com/YutaroHayakawa/go-ra"
	"github.com/YutaroHayakawa/go-ra/cmd/internal"

	"golang.org/x/sys/unix"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	configFile := flag.String("f", "", "config file path")
	v := flag.Bool("v", false, "show version information")

	flag.Parse()

	if *v {
		fmt.Printf("Version: %s, Commit: %s, Date: %s\n", version, commit, date)
		return
	}

	if *configFile == "" {
		slog.Error("Config file path is required. Aborting.")
		return
	}

	config, err := ra.ParseConfigYAMLFile(*configFile)
	if err != nil {
		slog.Error("Failed to parse config file. Aborting.", "error", err.Error())
		return
	}

	daemon, err := ra.NewDaemon(
		config,
		ra.WithLogger(slog.With("component", "daemon")),
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

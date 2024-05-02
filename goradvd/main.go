package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"

	"github.com/YutaroHayakawa/go-radv"
	"golang.org/x/sys/unix"
)

func main() {
	configFile := flag.String("f", "", "config file path")

	flag.Parse()

	if *configFile == "" {
		slog.Error("Config file path is required. Aborting.")
		return
	}

	config, err := radv.ParseConfigFile(*configFile)
	if err != nil {
		slog.Error("Failed to parse config file. Aborting.", "error", err.Error())
		return
	}

	slog.Info("Starting radv daemon", slog.Any("config", *config))

	daemon, err := radv.New(config)
	if err != nil {
		slog.Error("Failed to create daemon. Aborting.", "error", err.Error())
		return
	}

	sigCh := make(chan os.Signal, 3)
	signal.Notify(sigCh, unix.SIGINT, unix.SIGTERM, unix.SIGHUP)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for sig := range sigCh {
			switch sig {
			case unix.SIGINT, unix.SIGTERM:
				cancel()
				continue
			case unix.SIGHUP:
				config, err := radv.ParseConfigFile(*configFile)
				if err != nil {
					slog.Error("Failed to parse config file. Skip reloading.", "error", err.Error())
					continue
				}
				if err := daemon.Reload(ctx, config); err != nil {
					slog.Error("Failed to reload configuration", "error", err.Error())
					continue
				}
			default:
				slog.Warn("Received unexpected signal", "signal", sig.String())
			}
		}
	}()

	daemon.Run(ctx)
}

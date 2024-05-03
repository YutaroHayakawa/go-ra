package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os/signal"

	"github.com/YutaroHayakawa/go-radv"
	"github.com/YutaroHayakawa/go-radv/cmd/internal"

	"golang.org/x/sys/unix"
)

func writeError(w http.ResponseWriter, code int, errKind string, msg string) {
	m := internal.RAdvdError{
		Error: errKind,
		Msg:   msg,
	}

	j, err := json.Marshal(m)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Internal Server Error", "msg": "Failed to marshal JSON"}`))
		return
	}

	w.WriteHeader(code)
	w.Write([]byte(j))
}

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

	daemon, err := radv.NewDaemon(config)
	if err != nil {
		slog.Error("Failed to create daemon. Aborting.", "error", err.Error())
		return
	}

	http.HandleFunc("/reload", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		config, err := radv.ParseConfigJSON(r.Body)
		if err != nil {
			if errors.Is(err, &json.SyntaxError{}) {
				writeError(w, http.StatusBadRequest, "JSON Syntax Error", err.Error())
				return
			} else {
				writeError(w, http.StatusInternalServerError, "JSON Parser Error", err.Error())
				return
			}
		}

		if err := daemon.Reload(r.Context(), config); err != nil {
			var verrs radv.ValidationErrors
			if errors.As(err, &verrs) {
				writeError(w, http.StatusBadRequest, "Validation Error", verrs.Error())
				return
			} else {
				writeError(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
				return
			}
		}

		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		status := daemon.Status()

		j, err := json.Marshal(status)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to marshal JSON")
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(j)
	})

	go func() {
		slog.Info("Starting HTTP server on port localhost:8888")
		if err := http.ListenAndServe("localhost:8888", nil); err != nil {
			slog.Error("HTTP server failed with error", "error", err.Error())
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), unix.SIGINT, unix.SIGTERM)
	daemon.Run(ctx)
	cancel()
}

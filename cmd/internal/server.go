package internal

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/YutaroHayakawa/go-ra"
)

type Server struct {
	http.Server
	daemon *ra.Daemon
	logger *slog.Logger
}

func NewServer(host string, daemon *ra.Daemon, logger *slog.Logger) *Server {
	srv := &Server{
		daemon: daemon,
		logger: logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/reload", srv.handleReload)
	mux.HandleFunc("/status", srv.handleStatus)

	srv.Addr = host
	srv.Handler = mux

	return srv
}

func (s *Server) writeError(w http.ResponseWriter, code int, errKind string, msg string) {
	m := Error{
		Kind:    errKind,
		Message: msg,
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

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	config, err := ra.ParseConfigJSON(r.Body)
	if err != nil {
		if errors.Is(err, &json.SyntaxError{}) {
			s.writeError(w, http.StatusBadRequest, "JSONSyntaxError", err.Error())
			return
		} else {
			s.logger.Error("Failed to parse JSON", "error", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if err := s.daemon.Reload(r.Context(), config); err != nil {
		var verrs ra.ValidationErrors
		if errors.As(err, &verrs) {
			s.writeError(w, http.StatusBadRequest, "ValidationError", verrs.Error())
			return
		}

		if err = r.Context().Err(); err != nil {
			s.writeError(w, http.StatusRequestTimeout, "RequestTimeout", err.Error())
			return
		}

		s.logger.Error("Reload failed with unexpected error", "error", err.Error())

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	status := s.daemon.Status()

	j, err := json.Marshal(status)
	if err != nil {
		s.logger.Error("Failed to marshal JSON", "error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(j)
}

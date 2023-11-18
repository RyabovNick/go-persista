package server

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/RyabovNick/go-persista/internal/metrics"
	"github.com/RyabovNick/go-persista/internal/storage"
)

const (
	ExpiresHeader = "Expires"
)

type Server struct {
	Storage *storage.Storage
}

func (s *Server) HandleObject(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/objects/")
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		metrics.GetObjectsCounter.Inc()
		s.getObjectHandler(w, r, key)
	case "PUT":
		metrics.PutObjectsCounter.Inc()
		s.putObjectHandler(w, r, key)
	default:
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
	}
}

func (s *Server) putObjectHandler(w http.ResponseWriter, r *http.Request, key string) {
	// convert expires header to time if exists
	var (
		expires *time.Time
	)
	if r.Header.Get(ExpiresHeader) != "" {
		exp, err := time.Parse(time.RFC3339, r.Header.Get(ExpiresHeader))
		if err != nil {
			http.Error(w, "Invalid expires header", http.StatusBadRequest)
			return
		}

		expires = &exp
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Handle PUT /objects/{Key}
	s.Storage.Put(key, data, expires)

	w.WriteHeader(http.StatusOK)
}

func (s *Server) getObjectHandler(w http.ResponseWriter, _ *http.Request, key string) {
	data, ok := s.Storage.Get(key)
	if !ok {
		http.Error(w, "Object not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Server) HandleLivenessProbe(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) HandleReadinessProbe(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

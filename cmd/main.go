package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RyabovNick/go-persista/internal/metrics"
	"github.com/RyabovNick/go-persista/internal/server"
	"github.com/RyabovNick/go-persista/internal/storage"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	st := storage.New(ctx, storage.WithPersistent(storage.Persistent{
		Format:   storage.JSONFormat,
		Interval: 15 * time.Second,
	}))

	srv := &server.Server{
		Storage: st,
	}

	mux := http.NewServeMux()

	mux.Handle("/objects/", metrics.RequestDurationMiddleware(http.HandlerFunc(srv.HandleObject)))

	// Assume that the service isn't available directly from the internet
	// Otherwise, this methods should be serve on different ports
	mux.HandleFunc("/probes/liveness", srv.HandleLivenessProbe)
	mux.HandleFunc("/probes/readiness", srv.HandleReadinessProbe)
	mux.Handle("/metrics", promhttp.Handler())

	httpSrv := http.Server{
		Addr:        ":8080",
		Handler:     mux,
		ReadTimeout: 30 * time.Second,
	}

	go func() {
		defer stop()

		log.Printf("server started %s", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("listen and server error: %v", err)
		}
	}()

	<-ctx.Done()

	// Give the http server and storage 15 seconds to gracefully shutdown
	ctxt, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctxt); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	log.Print("server stopped")

	st.Shutdown()

	log.Print("storage stopped")
}

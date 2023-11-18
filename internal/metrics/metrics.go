// Package metrics provides prometheus metrics
package metrics

import (
	"net/http"
	"path"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// PutObjectsCounter is a counter of objects put in the storage
	PutObjectsCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "put_objects_total",
		Help: "The total number of objects put in the storage",
	})

	// GetObjectsCounter is a counter of objects get from the storage
	GetObjectsCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "get_objects_total",
		Help: "The total number of objects get from the storage",
	})

	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "request_duration_seconds",
		Help: "The duration of HTTP requests",
	}, []string{"method", "path"})
)

// RequestDurationMiddleware is a middleware that measures the duration
func RequestDurationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timer := prometheus.NewTimer(requestDuration.WithLabelValues(r.Method, path.Dir(r.URL.Path)))
		next.ServeHTTP(w, r)
		timer.ObserveDuration()
	})
}

func init() {
	prometheus.MustRegister(PutObjectsCounter)
	prometheus.MustRegister(GetObjectsCounter)
	prometheus.MustRegister(requestDuration)
}

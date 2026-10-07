package internalhttp

import (
	"net"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

// Unwrap supports optional interfaces through http.ResponseController.
func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func loggingMiddleware(log Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w}
		defer func() {
			status := wrapped.status
			if status == 0 {
				status = http.StatusOK
			}
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			log.Info("http request", "client_ip", ip, "request_time", start.Format(time.RFC3339Nano),
				"method", r.Method, "path", r.URL.RequestURI(), "protocol", r.Proto, "status", status,
				"latency", time.Since(start), "user_agent", r.UserAgent())
		}()
		next.ServeHTTP(wrapped, r)
	})
}

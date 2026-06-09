package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chiMW "github.com/go-chi/chi/v5/middleware"
)

// responseRecorder captura status code e bytes escritos.
type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func newRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Flush propaga Flush se o writer subjacente suportar.
func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Logger loga cada requisição e resposta via slog com campos estruturados.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := chiMW.GetReqID(r.Context())
		rec := newRecorder(w)

		slog.Info("req",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
			"req_id", reqID,
		)

		next.ServeHTTP(rec, r)

		slog.Info("res",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"bytes", rec.bytes,
			"ms", time.Since(start).Milliseconds(),
			"req_id", reqID,
		)
	})
}

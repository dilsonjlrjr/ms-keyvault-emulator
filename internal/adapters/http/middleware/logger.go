package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	chiMW "github.com/go-chi/chi/v5/middleware"
)

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

func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		reqID := chiMW.GetReqID(r.Context())
		rec := newRecorder(w)

		next.ServeHTTP(rec, r)

		dur := time.Since(start)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"bytes", rec.bytes,
			"duration_ms", dur.Milliseconds(),
			"remote", r.RemoteAddr,
			"req_id", reqID,
		}

		if ua := r.UserAgent(); ua != "" {
			attrs = append(attrs, "user_agent", ua)
		}
		if cl := r.ContentLength; cl > 0 {
			attrs = append(attrs, "content_length", cl)
		}

		vault := VaultFromContext(r.Context())
		if vault != nil {
			attrs = append(attrs, "vault", vault.Name)
		} else if vn := vaultNameFromHost(r.Host); vn != "" {
			attrs = append(attrs, "vault", vn)
		}

		lvl := slog.LevelInfo
		if rec.status >= 500 {
			lvl = slog.LevelError
		} else if rec.status >= 400 {
			lvl = slog.LevelWarn
		}

		slog.Log(r.Context(), lvl, "request", attrs...)
	})
}

func vaultNameFromHost(host string) string {
	h, _, _ := strings.Cut(host, ":")
	if dot := strings.IndexByte(h, '.'); dot > 0 {
		return h[:dot]
	}
	return ""
}

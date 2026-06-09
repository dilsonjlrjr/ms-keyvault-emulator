package middleware

import (
	"net/http"

	chiMW "github.com/go-chi/chi/v5/middleware"
)

// Audit grava cada requisição do data-plane no audit_log após a resposta.
func Audit(logFn func(actor, op, resource string, status int, reqID string)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := newRecorder(w)
			next.ServeHTTP(rec, r)

			actor, _ := r.Context().Value(ctxKeyActor).(string)
			if actor == "" {
				actor = "anonymous"
			}
			reqID := chiMW.GetReqID(r.Context())
			logFn(actor, r.Method, r.URL.Path, rec.status, reqID)
		})
	}
}

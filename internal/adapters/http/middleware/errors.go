package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dilsonrabelo/kvemu/internal/domain"
)

type kvErrorBody struct {
	Error kvErrorDetail `json:"error"`
}
type kvErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeKVError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(kvErrorBody{Error: kvErrorDetail{Code: code, Message: msg}})
}

// DomainError converte erros de domínio para HTTP + envelope Azure.
func DomainError(w http.ResponseWriter, err error) {
	switch e := err.(type) {
	case domain.ErrNotFound:
		writeKVError(w, http.StatusNotFound, e.Kind+"NotFound",
			"A "+strings.ToLower(e.Kind)+" with (name/id) "+e.Name+" was not found.")
	case domain.ErrConflict:
		writeKVError(w, http.StatusConflict, "Conflict", e.Error())
	case domain.ErrBadParam:
		writeKVError(w, http.StatusBadRequest, "BadParameter", e.Msg)
	case domain.ErrForbidden:
		writeKVError(w, http.StatusForbidden, "Forbidden", e.Msg)
	case domain.ErrDeleted:
		writeKVError(w, http.StatusConflict, "Conflict",
			e.Kind+" "+e.Name+" is currently in a deleted but recoverable state.")
	default:
		writeKVError(w, http.StatusInternalServerError, "InternalError", "An internal error occurred.")
	}
}

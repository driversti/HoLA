package respond

import (
	"encoding/json"
	"net/http"
)

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// ErrorResponse is the standard error format per SPEC.md.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// Error writes a structured JSON error response.
func Error(w http.ResponseWriter, status int, message, code string) {
	JSON(w, status, ErrorResponse{Error: message, Code: code})
}

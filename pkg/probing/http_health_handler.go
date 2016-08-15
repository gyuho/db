package probing

import (
	"encoding/json"
	"net/http"
	"time"
)

// NewHTTPHealthHandler returns http.Handler wrapper.
//
// (probing.NewHandler)
func NewHTTPHealthHandler() http.Handler {
	return &httpHealth{}
}

// httpHealth implements http.Handler interface.
type httpHealth struct{}

// Health represents health status.
type Health struct {
	OK            bool      `json:"ok"`
	RequestedTime time.Time `json:"requested_time"`
}

// ServeHTTP writes health-check messages in JSON format.
func (h *httpHealth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	health := Health{OK: true, RequestedTime: time.Now()}
	json.NewEncoder(w).Encode(health)
}

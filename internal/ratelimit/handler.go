package ratelimit

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ratelimit/api/internal/httpx"
)

// Handler exposes Part 1 HTTP endpoints.
type Handler struct {
	limiter *Limiter
}

// NewHandler wires the limiter into HTTP handlers.
func NewHandler(limiter *Limiter) *Handler {
	return &Handler{limiter: limiter}
}

// PostRequest handles POST /request.
func (h *Handler) PostRequest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "could not read request body")
		return
	}
	var req RequestBody
	if err := json.Unmarshal(body, &req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	if strings.TrimSpace(req.UserID) == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "user_id must not be blank")
		return
	}
	if len(req.Payload) == 0 || string(req.Payload) == "null" {
		httpx.WriteError(w, r, http.StatusBadRequest, "payload is required")
		return
	}
	if !json.Valid(req.Payload) {
		httpx.WriteError(w, r, http.StatusBadRequest, "payload must be valid JSON")
		return
	}

	userID := strings.TrimSpace(req.UserID)
	inWindow, ok := h.limiter.TryAccept(userID)
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(h.limiter.WindowDuration().Seconds())))
		httpx.WriteError(w, r, http.StatusTooManyRequests, "Rate limit exceeded for user: "+userID)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, AcceptedResponse{
		Status:           "accepted",
		UserID:           userID,
		RequestsInWindow: inWindow,
	})
}

// GetStats handles GET /stats.
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, StatsResponse{Users: h.limiter.Snapshot()})
}

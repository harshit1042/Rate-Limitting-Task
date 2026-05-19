package ratelimit

import "encoding/json"

// RequestBody is the JSON body for POST /request.
type RequestBody struct {
	UserID  string          `json:"user_id"`
	Payload json.RawMessage `json:"payload"`
}

// AcceptedResponse confirms an accepted request.
type AcceptedResponse struct {
	Status           string `json:"status"`
	UserID           string `json:"user_id"`
	RequestsInWindow int    `json:"requests_in_window"`
}

// StatsResponse is the JSON body for GET /stats.
type StatsResponse struct {
	Users []UserStats `json:"users"`
}

// UserStats is per-user rate limit counters.
type UserStats struct {
	UserID                  string `json:"user_id"`
	TotalAccepted           int64  `json:"total_accepted"`
	TotalRejected           int64  `json:"total_rejected"`
	RequestsInCurrentWindow int    `json:"requests_in_current_window"`
}

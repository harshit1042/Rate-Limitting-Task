package ratelimit

import "encoding/json"

type RequestBody struct {
	UserID  string          `json:"user_id"`
	Payload json.RawMessage `json:"payload"`
}

type AcceptedResponse struct {
	Status           string `json:"status"`
	UserID           string `json:"user_id"`
	RequestsInWindow int    `json:"requests_in_window"`
}

type StatsResponse struct {
	Users []UserStats `json:"users"`
}

type UserStats struct {
	UserID                  string `json:"user_id"`
	TotalAccepted           int64  `json:"total_accepted"`
	TotalRejected           int64  `json:"total_rejected"`
	RequestsInCurrentWindow int    `json:"requests_in_current_window"`
}

package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func DecodeJSON[T any](r *http.Request) (T, error) {
	var zero T
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return zero, errors.New("could not read request body")
	}
	if len(body) == 0 {
		return zero, errors.New("request body is required")
	}
	if err := json.Unmarshal(body, &zero); err != nil {
		return zero, errors.New("invalid JSON request body")
	}
	return zero, nil
}

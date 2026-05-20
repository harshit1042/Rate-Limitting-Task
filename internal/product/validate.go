package product

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ratelimit/api/internal/config"
)

func ValidateURLBatch(cfg config.Config, field string, urls []string) error {
	if len(urls) == 0 {
		return nil
	}
	if len(urls) > cfg.MaxURLsPerRequest {
		return fmt.Errorf("%s must contain at most %d URLs per request", field, cfg.MaxURLsPerRequest)
	}
	for _, raw := range urls {
		if err := validateOneURL(cfg, field, raw); err != nil {
			return err
		}
	}
	return nil
}

func validateOneURL(cfg config.Config, field, raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("%s must not contain blank URLs", field)
	}
	if len(trimmed) > cfg.MaxURLLength {
		return fmt.Errorf("%s URL exceeds maximum length of %d characters", field, cfg.MaxURLLength)
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("%s contains an invalid URL", field)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%s URLs must use http:// or https://", field)
	}
	if u.Host == "" {
		return fmt.Errorf("%s URLs must include a host", field)
	}
	return nil
}

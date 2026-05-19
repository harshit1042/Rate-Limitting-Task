package product

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ratelimit/api/internal/config"
	"github.com/ratelimit/api/internal/id"
)

// Service implements catalog business rules (validation + store orchestration).
type Service struct {
	cfg   config.Config
	store *Store
	clock func() time.Time
}

// NewService creates a catalog service backed by an in-memory store.
func NewService(cfg config.Config, store *Store) *Service {
	return &Service{
		cfg:   cfg,
		store: store,
		clock: func() time.Time { return time.Now().UTC() },
	}
}

// Create validates input and persists a new product.
func (s *Service) Create(name, sku string, imageURLs, videoURLs []string) (*Product, error) {
	name = strings.TrimSpace(name)
	sku = strings.TrimSpace(sku)
	if name == "" {
		return nil, fmt.Errorf("name must not be blank")
	}
	if sku == "" {
		return nil, fmt.Errorf("sku must not be blank")
	}
	images, videos, err := s.normalizeAndValidateURLs(imageURLs, videoURLs)
	if err != nil {
		return nil, err
	}
	p, err := s.store.Create(name, sku, images, videos, s.clock())
	if errors.Is(err, ErrDuplicateSKU) {
		return nil, fmt.Errorf("product with sku already exists: %s: %w", sku, ErrDuplicateSKU)
	}
	return p, err
}

// Get returns one product by id.
func (s *Service) Get(productID id.UUID) (*Product, error) {
	return s.store.GetByID(productID)
}

// List returns a paginated summary page. Empty limit/offset strings use defaults.
func (s *Service) List(limitStr, offsetStr string) ([]ListItem, int, int, int, error) {
	limit := s.cfg.DefaultLimit
	offset := 0

	if limitStr != "" {
		n, err := strconv.Atoi(limitStr)
		if err != nil {
			return nil, 0, 0, 0, fmt.Errorf("limit must be a number")
		}
		limit = n
	}
	if offsetStr != "" {
		n, err := strconv.Atoi(offsetStr)
		if err != nil {
			return nil, 0, 0, 0, fmt.Errorf("offset must be a number")
		}
		offset = n
	}
	if limit < 1 {
		return nil, 0, 0, 0, fmt.Errorf("limit must be at least 1")
	}
	if limit > s.cfg.MaxLimit {
		return nil, 0, 0, 0, fmt.Errorf("limit must not exceed %d", s.cfg.MaxLimit)
	}
	if offset < 0 {
		return nil, 0, 0, 0, fmt.Errorf("offset must be non-negative")
	}

	items, total := s.store.ListPage(offset, limit)
	return items, limit, offset, total, nil
}

// AppendMedia validates and appends URL batches to a product.
func (s *Service) AppendMedia(productID id.UUID, imageURLs, videoURLs []string) (*Product, error) {
	images, videos, err := s.normalizeAndValidateURLs(imageURLs, videoURLs)
	if err != nil {
		return nil, err
	}
	if len(images) == 0 && len(videos) == 0 {
		return nil, fmt.Errorf("at least one of image_urls or video_urls must be provided and non-empty")
	}
	return s.store.AppendMedia(productID, images, videos)
}

func (s *Service) normalizeAndValidateURLs(imageURLs, videoURLs []string) ([]string, []string, error) {
	images := coalesceSlice(imageURLs)
	videos := coalesceSlice(videoURLs)
	if err := ValidateURLBatch(s.cfg, "image_urls", images); err != nil {
		return nil, nil, err
	}
	if err := ValidateURLBatch(s.cfg, "video_urls", videos); err != nil {
		return nil, nil, err
	}
	return images, videos, nil
}

func coalesceSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

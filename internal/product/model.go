package product

import (
	"time"

	"github.com/ratelimit/api/internal/id"
)

// Product is a catalog item with URL-only media (no binary uploads).
type Product struct {
	ID        id.UUID   `json:"id"`
	Name      string    `json:"name"`
	SKU       string    `json:"sku"`
	ImageURLs []string  `json:"image_urls"`
	VideoURLs []string  `json:"video_urls"`
	CreatedAt time.Time `json:"created_at"`
}

// ListItem is a paginated grid row without full media arrays.
type ListItem struct {
	ID           id.UUID   `json:"id"`
	Name         string    `json:"name"`
	SKU          string    `json:"sku"`
	ImageCount   int       `json:"image_count"`
	VideoCount   int       `json:"video_count"`
	ThumbnailURL *string   `json:"thumbnail_url"`
	CreatedAt    time.Time `json:"created_at"`
}

// listPage is the JSON body for GET /products.
type listPage struct {
	Items      []ListItem `json:"items"`
	Limit      int        `json:"limit"`
	Offset     int        `json:"offset"`
	TotalCount int        `json:"total_count"`
}

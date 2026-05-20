package product

import (
	"sort"
	"sync"
	"time"

	"github.com/ratelimit/api/internal/id"
)

type Store struct {
	mu    sync.RWMutex
	byID  map[id.UUID]*Product
	bySKU map[string]id.UUID
}

func NewStore() *Store {
	return &Store{
		byID:  make(map[id.UUID]*Product),
		bySKU: make(map[string]id.UUID),
	}
}

func (s *Store) Create(name, sku string, imageURLs, videoURLs []string, now time.Time) (*Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.bySKU[sku]; exists {
		return nil, ErrDuplicateSKU
	}

	p := &Product{
		ID:        id.New(),
		Name:      name,
		SKU:       sku,
		ImageURLs: cloneStrings(imageURLs),
		VideoURLs: cloneStrings(videoURLs),
		CreatedAt: now,
	}
	s.byID[p.ID] = p
	s.bySKU[sku] = p.ID
	return cloneProduct(p), nil
}

func (s *Store) GetByID(productID id.UUID) (*Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.byID[productID]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneProduct(p), nil
}

func (s *Store) ListPage(offset, limit int) ([]ListItem, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]*Product, 0, len(s.byID))
	for _, p := range s.byID {
		all = append(all, p)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].ID.String() < all[j].ID.String()
		}
		return all[i].CreatedAt.Before(all[j].CreatedAt)
	})

	total := len(all)
	if offset >= total {
		return []ListItem{}, total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := all[offset:end]
	out := make([]ListItem, len(page))
	for i, p := range page {
		out[i] = toListItem(p)
	}
	return out, total
}

func (s *Store) AppendMedia(productID id.UUID, imageURLs, videoURLs []string) (*Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.byID[productID]
	if !ok {
		return nil, ErrNotFound
	}
	if len(imageURLs) > 0 {
		p.ImageURLs = append(p.ImageURLs, cloneStrings(imageURLs)...)
	}
	if len(videoURLs) > 0 {
		p.VideoURLs = append(p.VideoURLs, cloneStrings(videoURLs)...)
	}
	return cloneProduct(p), nil
}

func toListItem(p *Product) ListItem {
	item := ListItem{
		ID:         p.ID,
		Name:       p.Name,
		SKU:        p.SKU,
		ImageCount: len(p.ImageURLs),
		VideoCount: len(p.VideoURLs),
		CreatedAt:  p.CreatedAt,
	}
	if len(p.ImageURLs) > 0 {
		thumb := p.ImageURLs[0]
		item.ThumbnailURL = &thumb
	}
	return item
}

func cloneProduct(p *Product) *Product {
	return &Product{
		ID:        p.ID,
		Name:      p.Name,
		SKU:       p.SKU,
		ImageURLs: cloneStrings(p.ImageURLs),
		VideoURLs: cloneStrings(p.VideoURLs),
		CreatedAt: p.CreatedAt,
	}
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

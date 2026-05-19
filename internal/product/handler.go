package product

import (
	"errors"
	"net/http"

	"github.com/ratelimit/api/internal/httpx"
	"github.com/ratelimit/api/internal/id"
)

// Handler exposes Part 2 HTTP endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates the product HTTP handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type createRequest struct {
	Name      string   `json:"name"`
	SKU       string   `json:"sku"`
	ImageURLs []string `json:"image_urls"`
	VideoURLs []string `json:"video_urls"`
}

type appendRequest struct {
	ImageURLs []string `json:"image_urls"`
	VideoURLs []string `json:"video_urls"`
}

// Create handles POST /products.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	req, err := httpx.DecodeJSON[createRequest](r)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	p, err := h.service.Create(req.Name, req.SKU, req.ImageURLs, req.VideoURLs)
	writeResult(w, r, http.StatusCreated, p, err)
}

// List handles GET /products.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, limit, offset, total, err := h.service.List(q.Get("limit"), q.Get("offset"))
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, listPage{
		Items:      items,
		Limit:      limit,
		Offset:     offset,
		TotalCount: total,
	})
}

// Get handles GET /products/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request, productID id.UUID) {
	p, err := h.service.Get(productID)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, r, http.StatusNotFound, "Product not found: "+productID.String())
		return
	}
	writeResult(w, r, http.StatusOK, p, err)
}

// AppendMedia handles POST /products/{id}/media.
func (h *Handler) AppendMedia(w http.ResponseWriter, r *http.Request, productID id.UUID) {
	req, err := httpx.DecodeJSON[appendRequest](r)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	p, err := h.service.AppendMedia(productID, req.ImageURLs, req.VideoURLs)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, r, http.StatusNotFound, "Product not found: "+productID.String())
		return
	}
	writeResult(w, r, http.StatusOK, p, err)
}

func writeResult(w http.ResponseWriter, r *http.Request, okStatus int, p *Product, err error) {
	if err == nil {
		httpx.WriteJSON(w, okStatus, p)
		return
	}
	if errors.Is(err, ErrDuplicateSKU) {
		httpx.WriteError(w, r, http.StatusConflict, err.Error())
		return
	}
	httpx.WriteError(w, r, http.StatusBadRequest, err.Error())
}

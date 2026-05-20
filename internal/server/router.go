package server

import (
	"net/http"

	"github.com/ratelimit/api/internal/config"
	"github.com/ratelimit/api/internal/httpx"
	"github.com/ratelimit/api/internal/id"
	"github.com/ratelimit/api/internal/product"
	"github.com/ratelimit/api/internal/ratelimit"
)

func NewRouter(cfg config.Config) http.Handler {
	limiter := ratelimit.NewLimiter(cfg.MaxRequestsPerMinute, cfg.Window)
	rateHandler := ratelimit.NewHandler(limiter)

	store := product.NewStore()
	productHandler := product.NewHandler(product.NewService(cfg, store))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /request", rateHandler.PostRequest)
	mux.HandleFunc("GET /stats", rateHandler.GetStats)
	mux.HandleFunc("POST /products", productHandler.Create)
	mux.HandleFunc("GET /products", productHandler.List)
	mux.HandleFunc("GET /products/{id}", withProductID(productHandler.Get))
	mux.HandleFunc("POST /products/{id}/media", withProductID(productHandler.AppendMedia))
	return mux
}

type productRoute func(http.ResponseWriter, *http.Request, id.UUID)

func withProductID(handle productRoute) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		productID, err := id.Parse(r.PathValue("id"))
		if err != nil {
			httpx.WriteError(w, r, http.StatusBadRequest, "invalid product id")
			return
		}
		handle(w, r, productID)
	}
}

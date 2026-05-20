package product

import "errors"

var (
	ErrNotFound     = errors.New("product not found")
	ErrDuplicateSKU = errors.New("duplicate sku")
)

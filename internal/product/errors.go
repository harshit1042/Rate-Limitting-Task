package product

import "errors"

var (
	// ErrNotFound means the product id does not exist.
	ErrNotFound = errors.New("product not found")
	// ErrDuplicateSKU means the sku is already in use.
	ErrDuplicateSKU = errors.New("duplicate sku")
)

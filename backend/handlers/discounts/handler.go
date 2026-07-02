package discounts

import "log"

// DiscountsHandler serves public discount validation endpoints.
type DiscountsHandler struct {
	log *log.Logger
}

func New(logger *log.Logger) *DiscountsHandler {
	return &DiscountsHandler{log: logger}
}

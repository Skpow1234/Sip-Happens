package admin

import (
	"strings"

	"sip-happens/pkg/domain"
)

// validateProductPublish checks INVIMA registration before a product may be active in catalog.
func validateProductPublish(invimaReg string, isActive bool) error {
	if !isActive {
		return nil
	}
	if strings.TrimSpace(invimaReg) == "" {
		return domain.ErrInvalidInput
	}
	return nil
}

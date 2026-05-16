package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// recordAdminPIIAccess logs Ley 1581 access to customer personal data via admin APIs.
func (s *Server) recordAdminPIIAccess(r *http.Request, view, aggregateType, aggregateID string, extra map[string]string) {
	if s.Admin == nil {
		return
	}
	view = strings.TrimSpace(view)
	if view == "" {
		return
	}
	payload := map[string]string{"view": view}
	for k, v := range extra {
		if k != "" && v != "" {
			payload[k] = v
		}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	role, ok := s.callerAdminRole(r)
	actor := adminActorFromRequest(r)
	if ok {
		actor = fmt.Sprintf("%s:%s", actor, adminRoleLabel(role))
	}
	aggID := aggregateID
	if aggID == "" {
		aggID = "n/a"
	}
	_ = s.Admin.RecordAudit(r.Context(), actor, aggregateType, aggID, "customer.pii_viewed", string(b))
}

func adminRoleLabel(r AdminRole) string {
	switch r {
	case AdminRoleOps:
		return "ops"
	case AdminRoleSupport:
		return "support"
	case AdminRoleFinance:
		return "finance"
	default:
		return "unknown"
	}
}

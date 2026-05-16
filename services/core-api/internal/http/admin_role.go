package http

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"sip-happens/pkg/config"
)

// AdminRole is the coarse RBAC tier for bearer admin tokens (roadmap-040).
// finance < support < ops for privilege checks via AtLeast.
type AdminRole int

const (
	AdminRoleNone AdminRole = iota
	AdminRoleFinance
	AdminRoleSupport
	AdminRoleOps
)

func adminRoleRank(r AdminRole) int {
	switch r {
	case AdminRoleOps:
		return 3
	case AdminRoleSupport:
		return 2
	case AdminRoleFinance:
		return 1
	default:
		return 0
	}
}

// AtLeast returns true if r has at least the privileges of min.
func (r AdminRole) AtLeast(min AdminRole) bool {
	return adminRoleRank(r) >= adminRoleRank(min)
}

func parseAdminRoleLabel(s string) AdminRole {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ops", "admin":
		return AdminRoleOps
	case "support":
		return AdminRoleSupport
	case "finance":
		return AdminRoleFinance
	default:
		return AdminRoleNone
	}
}

func adminBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, prefix))
}

// callerAdminRole resolves the caller's AdminRole from AdminTokenRoles, or legacy AdminToken(s) as ops-only.
func (s *Server) callerAdminRole(r *http.Request) (AdminRole, bool) {
	tok := adminBearerToken(r)
	if tok == "" {
		return AdminRoleNone, false
	}
	if len(s.AdminTokenRoles) > 0 {
		role, ok := s.AdminTokenRoles[tok]
		if !ok {
			return AdminRoleNone, false
		}
		return role, true
	}
	// Legacy: ADMIN_TOKENS / ADMIN_TOKEN — full ops.
	if len(s.AdminTokens) > 0 {
		for _, t := range s.AdminTokens {
			if t == "" {
				continue
			}
			if subtle.ConstantTimeCompare([]byte(tok), []byte(t)) == 1 {
				return AdminRoleOps, true
			}
		}
		return AdminRoleNone, false
	}
	if s.AdminToken != "" && subtle.ConstantTimeCompare([]byte(tok), []byte(s.AdminToken)) == 1 {
		return AdminRoleOps, true
	}
	return AdminRoleNone, false
}

// requireAdminRole validates bearer auth and enforces a minimum role.
func (s *Server) requireAdminRole(w http.ResponseWriter, r *http.Request, min AdminRole) bool {
	if s.AdminToken == "" && len(s.AdminTokens) == 0 && len(s.AdminTokenRoles) == 0 {
		writeAPIError(w, r, http.StatusNotFound, "not_found", "admin not configured")
		return false
	}
	role, ok := s.callerAdminRole(r)
	if !ok {
		writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return false
	}
	if !role.AtLeast(min) {
		writeAPIError(w, r, http.StatusForbidden, "forbidden", "insufficient admin role")
		return false
	}
	return true
}

func adminActorFromRequest(r *http.Request) string {
	a := strings.TrimSpace(r.Header.Get("X-Admin-Actor"))
	if a == "" {
		return "admin"
	}
	return a
}

// AdminTokenRolesFromConfig builds the token→role map from CoreAPI env (ADMIN_* token lists).
func AdminTokenRolesFromConfig(c config.CoreAPI) map[string]AdminRole {
	m := c.AdminTokenRoleMap()
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]AdminRole, len(m))
	for tok, lab := range m {
		out[tok] = parseAdminRoleLabel(lab)
	}
	return out
}

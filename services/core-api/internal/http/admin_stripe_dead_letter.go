package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"sip-happens/pkg/domain"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
)

func (s *Server) handleAdminStripeDeadLetters(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleSupport) {
		return
	}
	if s.StripeWebhook == nil || s.StripeWebhook.EventLog == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "stripe webhook not configured")
		return
	}
	log, ok := s.StripeWebhook.EventLog.(*stripepayments.SQLEventLogStore)
	if !ok || log.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "stripe event log not configured")
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	limit := parseLimitQuery(r, 100, 500)
	offset := parseOffsetQuery(r)
	rows, err := log.ListDeadLetters(r.Context(), limit, offset)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": rows, "limit": limit, "offset": offset})
}

func (s *Server) handleAdminStripeDeadLetterReplay(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleOps) {
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.StripeWebhook == nil || s.StripeWebhook.EventLog == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "stripe webhook not configured")
		return
	}
	log, ok := s.StripeWebhook.EventLog.(*stripepayments.SQLEventLogStore)
	if !ok || log.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "stripe event log not configured")
		return
	}

	limit := parseIntDefault(r.URL.Query().Get("limit"), 0)
	if idStr := strings.TrimSpace(r.URL.Query().Get("id")); idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid id")
			return
		}
		res, err := s.StripeWebhook.ReplayDeadLetterByID(r.Context(), log, id)
		if err != nil {
			status, code := http.StatusInternalServerError, "internal"
			if err == domain.ErrNotFound {
				status, code = http.StatusNotFound, "not_found"
			} else if res.Status == "failed" {
				status, code = http.StatusUnprocessableEntity, "invalid_input"
			}
			writeAPIError(w, r, status, code, "replay failed")
			return
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"result": res})
		return
	}

	if limit <= 0 {
		limit = parseLimitQuery(r, 10, 100)
	}
	results, err := s.StripeWebhook.ReplayDeadLetters(r.Context(), log, limit)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"results": results, "limit": limit})
}

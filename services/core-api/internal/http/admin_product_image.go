package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/objectstore"
)

func (s *Server) handleAdminProductImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.requireAdminRole(w, r, AdminRoleOps) {
		return
	}
	if s.Admin == nil || s.Catalog == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	if s.ProductImages == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "unavailable", "object storage not configured")
		return
	}

	productID := domain.ProductID(strings.TrimSpace(r.PathValue("id")))
	if productID == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing product id")
		return
	}

	p, err := s.Catalog.GetProduct(r.Context(), s.DB, productID)
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal"
		if err == domain.ErrNotFound {
			status = http.StatusNotFound
			code = "not_found"
		}
		writeAPIError(w, r, status, code, "product not found")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid multipart form")
		return
	}
	file, hdr, err := r.FormFile("image")
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing image file")
		return
	}
	defer file.Close()

	ct, body, err := objectstore.DetectImageContentType(file)
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "unsupported image type")
		return
	}
	size := hdr.Size
	if size <= 0 {
		size = -1
	}

	publicURL, err := s.ProductImages.UploadProductImage(r.Context(), productID, p.SKU, ct, body, size)
	if err != nil {
		writeAPIError(w, r, http.StatusBadGateway, "bad_gateway", "upload failed")
		return
	}
	if err := s.Admin.SetProductImageURL(r.Context(), "admin", productID, publicURL); err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "product not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"product_id": productID,
		"image_url":  publicURL,
	})
}

package tui

import (
	"fmt"
	"strconv"
	"strings"

	"sip-happens/services/ssh-gateway/internal/api"
)

func parseOptionalABV(s string) (*float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, fmt.Errorf("%s", L.ErrInvalidABV(s))
	}
	return &f, nil
}

func (s *AppState) applyCatalogFiltersFromForm() error {
	f := &s.catalogFilter
	s.catalogQuery.Style = strings.TrimSpace(f.style)
	s.catalogQuery.Brewery = strings.TrimSpace(f.brewery)
	abvMin, err := parseOptionalABV(f.abvMin)
	if err != nil {
		return err
	}
	abvMax, err := parseOptionalABV(f.abvMax)
	if err != nil {
		return err
	}
	s.catalogQuery.ABVMin = abvMin
	s.catalogQuery.ABVMax = abvMax
	s.catalogQuery.Offset = 0
	return nil
}

func (s *AppState) syncCatalogFilterForm() {
	s.catalogFilter.style = s.catalogQuery.Style
	s.catalogFilter.brewery = s.catalogQuery.Brewery
	s.catalogFilter.abvMin = ""
	s.catalogFilter.abvMax = ""
	if s.catalogQuery.ABVMin != nil {
		s.catalogFilter.abvMin = strconv.FormatFloat(*s.catalogQuery.ABVMin, 'f', -1, 64)
	}
	if s.catalogQuery.ABVMax != nil {
		s.catalogFilter.abvMax = strconv.FormatFloat(*s.catalogQuery.ABVMax, 'f', -1, 64)
	}
}

func catalogQueryForLoad(s AppState) api.CatalogQuery {
	q := s.catalogQuery
	// Page size and featured flag follow the active group; user filters
	// (style, brewery, abv) and free-text query ride along unchanged so
	// the destacados tab can also be filtered.
	switch s.catalogGroup {
	case catalogGroupFeatured:
		q.Limit = catalogFeaturedSize
		q.Featured = true
	default:
		q.Limit = catalogPageSize
		q.Featured = false
	}
	return q
}

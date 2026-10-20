package http

import (
	"testing"

	"sip-happens/pkg/domain"
)

func TestScopeAllows(t *testing.T) {
	cases := []struct {
		name    string
		granted []domain.PersonalAccessTokenScope
		need    domain.PersonalAccessTokenScope
		want    bool
	}{
		{"exact read", []domain.PersonalAccessTokenScope{domain.PATScopeOrdersRead}, domain.PATScopeOrdersRead, true},
		{"write implies read", []domain.PersonalAccessTokenScope{domain.PATScopeOrdersWrite}, domain.PATScopeOrdersRead, true},
		{"read does not imply write", []domain.PersonalAccessTokenScope{domain.PATScopeOrdersRead}, domain.PATScopeOrdersWrite, false},
		{"unrelated", []domain.PersonalAccessTokenScope{domain.PATScopeCartWrite}, domain.PATScopeOrdersRead, false},
		{"profile write implies read", []domain.PersonalAccessTokenScope{domain.PATScopeProfileWrite}, domain.PATScopeProfileRead, true},
		{"empty", nil, domain.PATScopeCartWrite, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := scopeAllows(tc.granted, tc.need); got != tc.want {
				t.Fatalf("scopeAllows(%v, %q) = %v, want %v", tc.granted, tc.need, got, tc.want)
			}
		})
	}
}

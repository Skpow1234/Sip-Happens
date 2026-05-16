package healthcheck

import "testing"

func TestLocalHTTPAddr(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{":8080", "http://127.0.0.1:8080"},
		{"0.0.0.0:8080", "http://127.0.0.1:8080"},
		{"", "http://127.0.0.1:8080"},
	}
	for _, tc := range tests {
		if got := LocalHTTPAddr(tc.in); got != tc.want {
			t.Fatalf("LocalHTTPAddr(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

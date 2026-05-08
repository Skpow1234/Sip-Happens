package featureflags

import (
	"strings"
)

// Flags is a simple string-set driven by env config.
// It is intentionally minimal; for remote-config, add a new adapter later.
type Flags struct {
	Enabled map[string]struct{}
}

func Parse(csv string) Flags {
	out := Flags{Enabled: map[string]struct{}{}}
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return out
	}
	for _, p := range strings.Split(csv, ",") {
		k := strings.ToLower(strings.TrimSpace(p))
		if k == "" {
			continue
		}
		out.Enabled[k] = struct{}{}
	}
	return out
}

func (f Flags) Has(name string) bool {
	if f.Enabled == nil {
		return false
	}
	_, ok := f.Enabled[strings.ToLower(strings.TrimSpace(name))]
	return ok
}


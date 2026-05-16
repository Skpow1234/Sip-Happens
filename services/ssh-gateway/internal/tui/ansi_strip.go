package tui

// stripANSI removes ECMA-48 escape sequences for tests and plain-text checks.
func stripANSI(s string) string {
	var b []byte
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' || (s[i] == '\x9b' && i+1 < len(s)) {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		b = append(b, s[i])
		i++
	}
	return string(b)
}

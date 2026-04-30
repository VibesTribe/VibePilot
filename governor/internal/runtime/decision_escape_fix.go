package runtime

import "strings"

// unescapePlanContent converts literal JSON escape sequences to actual characters.
func unescapePlanContent(s string) string {
	// Replace \\ with a placeholder first to avoid double-unescaping
	const placeholder = "\x00ESC_BACKSLASH\x00"
	s = strings.ReplaceAll(s, `\\`, placeholder)
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, "\t")
	s = strings.ReplaceAll(s, `\"`, `"`)
	// Restore escaped backslashes as single backslash
	s = strings.ReplaceAll(s, placeholder, `\`)
	return s
}

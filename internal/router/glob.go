package router

import "strings"

// matchGlob supports:
//
//	"*"              → match all
//	"payment.*"      → match payment.succeeded, payment.failed (single level)
//	"payment.**"     → match payment.intent.succeeded (multi-level)
//	"payment.succeeded" → exact match
func matchGlob(pattern, value string) bool {
	if pattern == "*" || pattern == "**" {
		return true
	}

	patternParts := strings.Split(pattern, ".")
	valueParts := strings.Split(value, ".")

	pi, vi := 0, 0
	for pi < len(patternParts) && vi < len(valueParts) {
		if patternParts[pi] == "**" {
			return true // ** matches all remaining
		}
		if patternParts[pi] == "*" {
			pi++
			vi++
			continue
		}
		if patternParts[pi] != valueParts[vi] {
			return false
		}
		pi++
		vi++
	}

	return pi == len(patternParts) && vi == len(valueParts)
}

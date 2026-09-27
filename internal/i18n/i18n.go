// Package i18n selects the language of human-readable messages. Machine
// facing strings (states, ops, JSON keys) are always English.
package i18n

import (
	"fmt"
	"strings"
)

// Supported languages.
const (
	ZH = "zh"
	EN = "en"
)

var current = EN

// Set selects the message language. Unknown values fall back to English so
// output stays predictable.
func Set(l string) {
	switch strings.ToLower(strings.TrimSpace(l)) {
	case ZH, "cn", "chinese":
		current = ZH
	case EN, "english":
		current = EN
	default:
		current = EN
	}
}

// SetAuto picks zh when the locale mentions Chinese, otherwise English.
func SetAuto(locale string) {
	loc := strings.ToLower(locale)
	if strings.Contains(loc, "zh") || strings.Contains(loc, "chinese") || strings.Contains(loc, "cn") {
		current = ZH
		return
	}
	current = EN
}

// Current returns the active language code.
func Current() string { return current }

// Tr formats a message in the active language; zh/en are format strings.
func Tr(zh, en string, args ...any) string {
	s := en
	if current == ZH {
		s = zh
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}

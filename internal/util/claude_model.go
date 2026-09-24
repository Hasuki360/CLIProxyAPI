package util

import "strings"

// context1MSuffix is the Claude Code model-name marker that selects the
// 1M-token context window (e.g. "claude-opus-5-5[1m]").
const context1MSuffix = "[1m]"

// IsClaudeThinkingModel checks if the model is a Claude thinking model
// that requires the interleaved-thinking beta header.
func IsClaudeThinkingModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "claude") && strings.Contains(lower, "thinking")
}

// StripContext1MSuffix removes the "[1m]" context-window marker from a model
// name and reports whether it was present. The marker may sit before or after
// a thinking suffix, so both "m[1m](high)" and "m(high)[1m]" become "m(high)".
// Matching is case-insensitive. A name that is only the marker is left as is.
func StripContext1MSuffix(model string) (string, bool) {
	trimmed := strings.TrimSpace(model)
	if hasContext1MSuffix(trimmed) {
		if base := strings.TrimSpace(trimmed[:len(trimmed)-len(context1MSuffix)]); base != "" {
			return base, true
		}
		return model, false
	}
	if !strings.HasSuffix(trimmed, ")") {
		return model, false
	}
	open := strings.LastIndex(trimmed, "(")
	if open <= 0 || !hasContext1MSuffix(trimmed[:open]) {
		return model, false
	}
	base := strings.TrimSpace(trimmed[:open-len(context1MSuffix)])
	if base == "" {
		return model, false
	}
	return base + trimmed[open:], true
}

func hasContext1MSuffix(model string) bool {
	return len(model) >= len(context1MSuffix) && strings.EqualFold(model[len(model)-len(context1MSuffix):], context1MSuffix)
}

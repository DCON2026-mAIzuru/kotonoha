package llm

import "strings"

func normalizeGeminiModelName(model string) string {
	return strings.TrimPrefix(strings.TrimSpace(model), "models/")
}

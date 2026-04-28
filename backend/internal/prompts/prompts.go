package prompts

import (
	_ "embed"
	"strings"
)

//go:embed parenting_chat.md
var parentingChat string

func ParentingChat() string {
	return strings.TrimSpace(parentingChat)
}

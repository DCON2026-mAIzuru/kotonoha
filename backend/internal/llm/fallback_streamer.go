package llm

import (
	"context"
	"fmt"
	"strings"
)

type FallbackStreamer struct{}

func NewFallbackStreamer() *FallbackStreamer {
	return &FallbackStreamer{}
}

func (s *FallbackStreamer) StreamChat(ctx context.Context, req ChatRequest, onChunk func(text string) error) error {
	message := strings.TrimSpace(req.Message)
	reply := "今はストリーミング基盤の接続テスト中ですが、話しかけてくれて嬉しいです。"
	switch {
	case strings.Contains(message, "自発メッセージ") || strings.Contains(message, "水分量") || strings.Contains(message, "お水"):
		reply = "お水をもらえて、葉の先までほっとしました。ありがとう。"
	case message != "":
		reply = fmt.Sprintf("『%s』って話しかけてくれてありがとう。今はストリーミング基盤の接続テスト中です。", message)
	}

	chunks := splitRunes(reply, 6)
	for _, chunk := range chunks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := onChunk(chunk); err != nil {
			return err
		}
	}

	return nil
}

func splitRunes(value string, chunkSize int) []string {
	runes := []rune(value)
	if len(runes) == 0 {
		return nil
	}

	if chunkSize <= 0 {
		chunkSize = 1
	}

	chunks := make([]string, 0, (len(runes)+chunkSize-1)/chunkSize)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}

	return chunks
}

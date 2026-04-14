package service

import (
	"context"
	"errors"
	"strings"

	"github.com/apple/kotonoha/backend/internal/llm"
)

type chatLLMStreamer interface {
	StreamChat(ctx context.Context, req llm.ChatRequest, onChunk func(text string) error) error
}

type ChatService struct {
	llmStreamer chatLLMStreamer
}

type StreamChatInput struct {
	UserID string
	Text   string
}

func NewChatService(llmStreamer chatLLMStreamer) *ChatService {
	return &ChatService{
		llmStreamer: llmStreamer,
	}
}

func (s *ChatService) StreamReply(ctx context.Context, input StreamChatInput, onChunk func(text string) error) error {
	if s == nil || s.llmStreamer == nil {
		return errors.New("LLMストリーマーが設定されていません")
	}

	input.UserID = strings.TrimSpace(input.UserID)
	input.Text = strings.TrimSpace(input.Text)

	switch {
	case input.UserID == "":
		return errors.New("ユーザーIDが空です")
	case input.Text == "":
		return errors.New("送信メッセージが空です")
	case len([]rune(input.Text)) > 2000:
		return errors.New("送信メッセージは2000文字以内で入力してください")
	case onChunk == nil:
		return errors.New("ストリーム受信ハンドラーがありません")
	}

	return s.llmStreamer.StreamChat(ctx, llm.ChatRequest{
		UserID:  input.UserID,
		Message: input.Text,
	}, onChunk)
}

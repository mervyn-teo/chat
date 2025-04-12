package openai

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

type ChatCompletionMessage = openai.ChatCompletionMessage

const (
	ChatMessageRoleSystem    = openai.ChatMessageRoleSystem
	ChatMessageRoleUser      = openai.ChatMessageRoleUser
	ChatMessageRoleAssistant = openai.ChatMessageRoleAssistant
)

func SendMessage(client *openai.Client, messages []ChatCompletionMessage) (string, error) {
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    OpenRouterModel,
			Messages: messages,
		},
	)

	if err != nil {
		return "", fmt.Errorf("chat completion failed: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("received an empty or invalid response from API")
	}

	return resp.Choices[0].Message.Content, nil
}

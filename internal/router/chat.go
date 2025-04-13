package router

import (
	"context"
	"fmt"
	"log"
	"strings"
	"untitled/internal/bot"

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

func MessageLoop(Mybot *bot.Bot, client *openai.Client, messageChannel chan *bot.MessageWithWait, instructions string) {
	msg := initRouter(instructions)

	messages := msg
	for {
		userInput := <-messageChannel
		parsed, isSkip := parseUserInput(userInput.Message.Content)

		if isSkip {
			continue
		}

		messages = append(messages, ChatCompletionMessage{
			Role:    ChatMessageRoleUser,
			Content: parsed,
		})

		aiResponseContent, err := SendMessage(client, messages)
		if err != nil {
			log.Printf("Error getting response from OpenRouter: %v", err)
			if len(messages) > 0 {
				messages = messages[:len(messages)-1]
			}
			continue
		}

		messages = append(messages, ChatCompletionMessage{
			Role:    ChatMessageRoleAssistant,
			Content: aiResponseContent,
		})

		go Mybot.RespondToMessage(userInput.Message.ChannelID, aiResponseContent, userInput.Message.Reference(), userInput.WaitMessage)
	}
}

func parseUserInput(userInput string) (parsed string, skip bool) {

	userInput = strings.TrimSpace(userInput)

	if userInput == "" {
		return "", true
	}
	return userInput, false
}

func initRouter(instructions string) []ChatCompletionMessage {
	var messages []ChatCompletionMessage
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: instructions,
	})
	return messages
}

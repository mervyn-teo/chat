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

func MessageLoop(ctx context.Context, Mybot *bot.Bot, client *openai.Client, messageChannel chan *bot.MessageWithWait, instructions string) {
	messages := initRouter(instructions)

	for {
		select {
		case <-ctx.Done(): // Check if context has been cancelled
			log.Println("Router loop: shutdown signal received.")
			// Perform any necessary cleanup within the router
			return // Exit the loop

		case userInput := <-messageChannel:
			userID := userInput.Message.Author.ID
			parsed, isSkip := parseUserInput(userInput.Message.Content)

			if isSkip {
				continue
			}

			currentMessages, userExists := messages[userID]
			if !userExists {
				// First message from this user, initialize with system prompt
				log.Printf("Initializing conversation for user: %s", userID)
				currentMessages = setInitialMessages(instructions)
			}

			messages[userID] = append(currentMessages, ChatCompletionMessage{
				Role:    ChatMessageRoleUser,
				Content: parsed,
			})

			aiResponseContent, err := SendMessage(client, messages[userInput.Message.Author.ID])
			if err != nil {
				log.Printf("Error getting response from OpenRouter: %v", err)
				if len(messages) > 0 {
					// Remove the last message if there's an error
					messages[userID] = currentMessages[:len(messages)-1]
					aiResponseContent = "There was an error processing your request. Please try again."
				}
			}

			messages[userID] = append(currentMessages, ChatCompletionMessage{
				Role:    ChatMessageRoleAssistant,
				Content: aiResponseContent,
			})

			go Mybot.RespondToMessage(userInput.Message.ChannelID, aiResponseContent, userInput.Message.Reference(), userInput.WaitMessage)
		}
	}
}

func parseUserInput(userInput string) (parsed string, skip bool) {

	userInput = strings.TrimSpace(userInput)

	if userInput == "" {
		return "", true
	}
	return userInput, false
}

func initRouter(instructions string) map[string][]ChatCompletionMessage {
	messages := make(map[string][]ChatCompletionMessage)
	return messages
}

func setInitialMessages(instructions string) []ChatCompletionMessage {
	return []ChatCompletionMessage{
		{
			Role:    ChatMessageRoleSystem,
			Content: instructions,
		},
	}
}

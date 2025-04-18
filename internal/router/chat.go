package router

import (
	"context"
	"fmt"
	"log"
	"strings"
	"untitled/internal/bot"
	"untitled/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

type ChatCompletionMessage = openai.ChatCompletionMessage

const (
	ChatMessageRoleSystem    = openai.ChatMessageRoleSystem
	ChatMessageRoleUser      = openai.ChatMessageRoleUser
	ChatMessageRoleAssistant = openai.ChatMessageRoleAssistant
)

func SendMessage(client *openai.Client, messages []ChatCompletionMessage) (string, error) {
	availableTools := tools.GetAvailableTools()
	if len(availableTools) < 1 {
		log.Println("Warning: No tools available for the model to use.")
	}

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    OpenRouterModel,
			Messages: messages,
			Tools:    availableTools,
		},
	)

	if err != nil {
		return "", fmt.Errorf("chat completion failed: %w", err)
	}

	choice := resp.Choices[0]

	if choice.FinishReason == openai.FinishReasonToolCalls && len(choice.Message.ToolCalls) > 0 {
		log.Printf("Model wants to use tools. Number of tool calls: %d\n", len(choice.Message.ToolCalls))

		followUpMessages := append(messages, choice.Message)
		var toolResponses []openai.ChatCompletionMessage
		for _, toolCall := range choice.Message.ToolCalls {
			resultString, execErr := tools.ExecuteToolCall(toolCall)

			if execErr != nil {
				log.Printf("Failed to execute tool call %s (%s): %v", toolCall.ID, toolCall.Function.Name, execErr)
				// The resultString should contain an AI-friendly error message
			} else {
				log.Printf("Successfully executed tool call %s (%s)", toolCall.ID, toolCall.Function.Name)
			}

			// Prepare the message with the function result for the next API call
			toolResponses = append(toolResponses, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    resultString,
				ToolCallID: toolCall.ID,
			})
		}

		followUpMessages = append(followUpMessages, toolResponses...)

		fmt.Println("\n--- Sending follow-up request with tool results ---")

		// Create the follow-up request with the updated message history
		followUpReq := openai.ChatCompletionRequest{
			Model:    OpenRouterModel, // Use the same model
			Messages: followUpMessages,
		}

		log.Println("Sending follow-up request...")
		finalResp, finalErr := client.CreateChatCompletion(context.Background(), followUpReq)
		if finalErr != nil {
			log.Fatalf("Error creating follow-up chat completion: %v", finalErr)
		}

		// The final response from the model, incorporating the tool results
		fmt.Println("\n--- Final response from model ---")
		return finalResp.Choices[0].Message.Content, nil
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
			parsedUserMsg, isSkip := parseUserInput(userInput.Message.Content)

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
				Content: parsedUserMsg,
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

			log.Println("Response to user: " + aiResponseContent)
			go Mybot.RespondToMessage(userInput.Message.ChannelID, aiResponseContent, userInput.Message.Reference(), userInput.WaitMessage)
		}
	}
}

func parseUserInput(userInput string) (parsed string, skip bool) {

	userInput = strings.TrimSpace(userInput)
	userInput = strings.TrimPrefix(userInput, "!ask")

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

package router

import (
	"context"
	"fmt"
	"log"
	"strings"
	"untitled/internal/bot"
	"untitled/internal/reminder"
	"untitled/internal/storage"
	"untitled/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

type ChatCompletionMessage = openai.ChatCompletionMessage

const (
	ChatMessageRoleSystem    = openai.ChatMessageRoleSystem
	ChatMessageRoleUser      = openai.ChatMessageRoleUser
	ChatMessageRoleAssistant = openai.ChatMessageRoleAssistant
)

var reminders reminder.ReminderList

func SendMessage(client *openai.Client, messages []ChatCompletionMessage, myBot *bot.Bot) (string, error) {
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

	if len(resp.Choices) == 0 {
		log.Println("received an empty response from API, trying again")
		return "", fmt.Errorf("received an empty response from API")
	}

	choice := resp.Choices[0]

	// Check if the model wants to use tools
	for choice.FinishReason == openai.FinishReasonToolCalls && len(choice.Message.ToolCalls) > 0 {
		log.Printf("Model wants to use tools. Number of tool calls: %d\n", len(choice.Message.ToolCalls))

		followUpMessages := append(messages, choice.Message)
		var toolResponses []openai.ChatCompletionMessage

		var resultString string
		var err error

		for _, toolCall := range choice.Message.ToolCalls {

			// intersect reminder call
			if strings.Contains(toolCall.Function.Name, "reminder") {
				log.Printf("Reminder call detected. ID: %s", toolCall.ID)
				resultString, err = tools.HandleReminderCall(toolCall, &reminders, myBot)
			} else {
				resultString, err = tools.ExecuteToolCall(toolCall)
			}

			if err != nil {
				log.Printf("Failed to execute tool call %s (%s): %v", toolCall.ID, toolCall.Function.Name, err)
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
			Tools:    availableTools, // Include the tool calls in the follow-up request
		}

		log.Println("Sending follow-up request...")
		finalResp, finalErr := client.CreateChatCompletion(context.Background(), followUpReq)

		if finalErr != nil {
			log.Fatalf("Error creating follow-up chat completion: %v", finalErr)
		}

		if len(finalResp.Choices) == 0 {
			log.Println("received an empty response from API, trying again")
			return "", fmt.Errorf("received an empty response from API")
		}

		choice = finalResp.Choices[0]

		if choice.FinishReason == openai.FinishReasonToolCalls {
			continue
		}

		// The final response from the model, incorporating the tool results
		fmt.Println("\n--- Final response from model ---")
		return finalResp.Choices[0].Message.Content, nil
	}

	return resp.Choices[0].Message.Content, nil
}

func SplitString(s string, chunkSize int) []string {
	log.Printf("Splitting string into chunks of size %d", chunkSize)
	runes := []rune(s)
	ret := make([]string, 0, len(runes)/chunkSize)
	res := ""

	for i, r := range runes {
		res = res + string(r)
		if i > 0 && ((i+1)%chunkSize == 0 || i == len(runes)-1) {
			ret = append(ret, res)
			res = ""
		}
	}

	return ret
}

func MessageLoop(ctx context.Context, Mybot *bot.Bot, client *openai.Client, messageChannel chan *bot.MessageWithWait, instructions string, messages map[string][]ChatCompletionMessage, chatFilepath string) {
	err := reminder.LoadRemindersFromFile(&reminders)
	if err != nil {
		log.Println("Error loading reminders from file:", err)
		return
	}

	if messages == nil {
		log.Println("Router loop: messages map is nil, initializing.")
		messages = initRouter()
		storage.SaveChatHistory(messages, chatFilepath)
	}

	for {
		select {
		case <-ctx.Done(): // Check if context has been cancelled
			log.Println("Router loop: shutdown signal received.")
			// Perform any necessary cleanup within the router
			return // Exit the loop

		case userInput := <-messageChannel:

			if *userInput.IsForget {
				// Handle the forget command
				messages[userInput.Message.Author.ID] = setInitialMessages(instructions, userInput.Message.Author.ID)
				storage.SaveChatHistory(messages, chatFilepath)

				log.Printf("Forget command executed for user %s in channel %s", userInput.Message.Author.ID, userInput.Message.ChannelID)
				go Mybot.RespondToMessage(userInput.Message.ChannelID, "Your message history has been cleared", userInput.Message.Reference(), userInput.WaitMessage)
				continue
			}

			userID := userInput.Message.Author.ID
			parsedUserMsg, isSkip := parseUserInput(userInput.Message.Content)
			parsedUserMsg = fmt.Sprintf("userID: %s, userName: %s said in channelID %s: %s", userID, userInput.Message.Author.Username, userInput.Message.ChannelID, parsedUserMsg)

			if isSkip {
				continue
			}

			currentMessages, userExists := messages[userID]
			if !userExists {
				// First message from this user, initialize with system prompt
				log.Printf("Initializing conversation for user: %s", userID)
				currentMessages = setInitialMessages(instructions, userID)
			}

			messages[userID] = append(currentMessages, ChatCompletionMessage{
				Role:    ChatMessageRoleUser,
				Content: parsedUserMsg,
			})
			storage.SaveChatHistory(messages, chatFilepath)

			aiResponseContent, err := SendMessage(client, messages[userID], Mybot)

			if err != nil {
				log.Printf("Error getting response from OpenRouter: %v", err)
				if len(messages) > 0 {
					// Remove the last message if there's an error
					messages[userID] = currentMessages[:len(messages)-1]
					aiResponseContent = "There was an error processing your request. Please try again."
				}
			}

			messages[userID] = append(messages[userID], ChatCompletionMessage{
				Role:    ChatMessageRoleAssistant,
				Content: aiResponseContent,
			})
			storage.SaveChatHistory(messages, chatFilepath)

			log.Println("Response to user: " + aiResponseContent)

			if len(aiResponseContent) > 1900 {
				// Split the response into chunks of 1900 characters
				chunks := SplitString(aiResponseContent, 1900)
				go Mybot.RespondToLongMessage(userInput.Message.ChannelID, chunks, userInput.Message.Reference(), userInput.WaitMessage)
				continue
			}

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

func initRouter() map[string][]ChatCompletionMessage {
	messages := make(map[string][]ChatCompletionMessage)
	return messages
}

func setInitialMessages(instructions string, userID string) []ChatCompletionMessage {
	return []ChatCompletionMessage{
		{
			Role:    ChatMessageRoleSystem,
			Content: "You are talking to: " + userID + "\n" + instructions,
		},
	}
}

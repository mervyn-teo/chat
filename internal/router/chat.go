package router

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"untitled/internal/bot"
	"untitled/internal/music"
	"untitled/internal/reminder"
	"untitled/internal/storage"
	"untitled/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

type ChatCompletionMessage = openai.ChatCompletionMessage

const (
	ChatMessageRoleSystem = openai.ChatMessageRoleSystem
	ChatMessageRoleUser   = openai.ChatMessageRoleUser

	MaxMessageLength      = 1900
	MaxMessagesToKeep     = 20
	MaxToolCallIterations = 10
	DefaultChunkSize      = 1900
)

var (
	reminders      reminder.ReminderList
	remindersMutex sync.RWMutex

	songMap      map[string]map[string]*music.SongList = make(map[string]map[string]*music.SongList)
	songMapMutex sync.RWMutex
)

// SendMessage sends a message to the OpenRouter API and handles tool calls
func SendMessage(client *openai.Client, messages *[]ChatCompletionMessage, myBot *bot.Bot) (string, error) {
	if client == nil || messages == nil || myBot == nil {
		return "", fmt.Errorf("invalid parameters: client, messages, or bot is nil")
	}

	availableTools := tools.GetAvailableTools()
	if len(availableTools) < 1 {
		log.Println("Warning: No tools available for the model to use.")
	}

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    OpenRouterModel,
			Messages: *messages,
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

	interations := 0
	// Check if the model wants to use tools
	for choice.FinishReason == openai.FinishReasonToolCalls && len(choice.Message.ToolCalls) > 0 {
		interations++
		if interations > MaxToolCallIterations {
			log.Printf("Maximum tool call iterations (%d) exceeded", MaxToolCallIterations)
			return "", fmt.Errorf("maximum tool call iterations exceeded")
		}

		log.Printf("Model wants to use tools. Number of tool calls: %d\n", len(choice.Message.ToolCalls))

		*messages = append(*messages, choice.Message)
		var toolResponses []openai.ChatCompletionMessage

		for _, toolCall := range choice.Message.ToolCalls {

			resultString, err := runFunctionCall(toolCall, myBot)

			if err != nil {
				log.Printf("Failed to execute tool call %s (%s): %v", toolCall.ID, toolCall.Function.Name, err)
				fmt.Println("result Str: " + resultString)
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

		*messages = append(*messages, toolResponses...)

		fmt.Println("\n--- Sending follow-up request with tool results ---")

		// Create the follow-up request with the updated message history
		followUpReq := openai.ChatCompletionRequest{
			Model:    OpenRouterModel, // Use the same model
			Messages: *messages,
			Tools:    availableTools, // Include the tool calls in the follow-up request
		}

		log.Println("Sending follow-up request...")
		finalResp, finalErr := client.CreateChatCompletion(context.Background(), followUpReq)

		if finalErr != nil {
			log.Fatalf("Error creating follow-up chat completion: %v", finalErr)
		}

		if len(finalResp.Choices) == 0 {
			log.Println("received an empty response from openRouter, trying again")
			return "", fmt.Errorf("received an empty response from openRouter")
		}

		*messages = append(*messages, finalResp.Choices[0].Message)

		choice = finalResp.Choices[0]

		if choice.FinishReason == openai.FinishReasonToolCalls {
			continue
		}

		// The final response from the model, incorporating the tool results
		fmt.Println("\n--- Final response from model ---")
		return finalResp.Choices[0].Message.Content, nil
	}

	if choice.FinishReason == openai.FinishReasonStop {
		*messages = append(*messages, resp.Choices[0].Message)
		return choice.Message.Content, nil
	}

	return resp.Choices[0].Message.Content, nil
}

func runFunctionCall(toolCall openai.ToolCall, myBot *bot.Bot) (string, error) {
	var resultString string
	var err error

	// Check if the tool call is a function call

	if strings.Contains(toolCall.Function.Name, "reminder") {
		log.Printf("Reminder call detected. ID: %s", toolCall.ID)
		remindersMutex.Lock()
		resultString, err = tools.HandleReminderCall(toolCall, &reminders, myBot)
		remindersMutex.Unlock()

	} else if strings.Contains(toolCall.Function.Name, "song") {
		log.Printf("Music call detected. ID: %s", toolCall.ID)
		songMapMutex.Lock()
		resultString, err = tools.HandleMusicCall(toolCall, &songMap, myBot)
		songMapMutex.Unlock()

	} else if strings.Contains(toolCall.Function.Name, "voice") {
		log.Printf("Voice channel call detected. ID: %s", toolCall.ID)
		resultString, err = tools.HandleVoiceChannel(toolCall, myBot)
	} else {
		log.Printf("Normal function call detected. ID: %s\n", toolCall.ID)
		resultString, err = tools.ExecuteToolCall(toolCall)
	}

	if err != nil {
		log.Printf("Error executing function call: %v", err)
		return resultString, err
	}

	return resultString, nil
}

func SplitString(s string, chunkSize int) []string {
	if chunkSize <= 0 {
		log.Printf("Invalid chunk size: %d, using default", chunkSize)
		chunkSize = DefaultChunkSize
	}

	runes := []rune(s)
	if len(runes) == 0 {
		return []string{""}
	}

	ret := make([]string, 0, (len(runes)/chunkSize)+1)
	var res strings.Builder // Efficient string building

	for i, r := range runes {
		res.WriteRune(r)
		if (i+1)%chunkSize == 0 || i == len(runes)-1 {
			ret = append(ret, res.String())
			res.Reset()
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

	err = music.LoadSongMapFromFile(&songMap)
	if err != nil {
		log.Println("Error loading song map from file:", err)
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

			/*
				Mesaage format:
				{
					"userID"		: "1234567890",
					"userName"		: "exampleUser",
					"guildID"		: "0987654321",
					"textchannelID"	: "1122334455",
					"message" 		: "Hello, how are you?"
				}

			*/

			userID := userInput.Message.Author.ID
			parsedUserMsg, isSkip := parseUserInput(userInput.Message.Content)
			parsedUserMsg = fmt.Sprintf("{\n"+
				"\"userID\": \"%s\", \n"+
				"\"userName\": \"%s\", \n"+
				"\"guildID\": %s, \n"+
				"\"textchannelID\" \"%s\", \n"+
				"\"content\": \"%s\"\n"+
				"}", userID, userInput.Message.Author.Username, userInput.Message.GuildID, userInput.Message.ChannelID, parsedUserMsg)

			if isSkip {
				continue
			}

			currentMessages, userExists := messages[userID]
			if !userExists {
				// First message from this user, initialize with system prompt
				log.Printf("Initializing conversation for user: %s", userID)
				currentMessages = setInitialMessages(instructions, userID)
			}

			updatedMessages := make([]ChatCompletionMessage, len(currentMessages))
			copy(updatedMessages, currentMessages)

			updatedMessages = append(updatedMessages, ChatCompletionMessage{
				Role:    ChatMessageRoleUser,
				Content: parsedUserMsg,
			})

			storage.SaveChatHistory(messages, chatFilepath)

			aiResponseContent, err := SendMessage(client, &updatedMessages, Mybot)

			if err != nil {
				log.Printf("Error getting response from OpenRouter: %v", err)
				if len(messages[userID]) > 0 {
					messages[userID] = messages[userID][:len(messages[userID])-1]
				}
			}

			messages[userID] = updatedMessages

			// Trim messages
			trimmed := trimMsg(messages[userID], MaxMessagesToKeep)
			messages[userID] = trimmed

			storage.SaveChatHistory(messages, chatFilepath)

			log.Println("Response to user: " + aiResponseContent)

			if len(aiResponseContent) > MaxMessageLength {
				// Split the response into chunks of 1900 characters
				chunks := SplitString(aiResponseContent, MaxMessageLength)
				go Mybot.RespondToLongMessage(userInput.Message.ChannelID, chunks, userInput.Message.Reference(), userInput.WaitMessage)
				continue
			}

			go Mybot.RespondToMessage(userInput.Message.ChannelID, aiResponseContent, userInput.Message.Reference(), userInput.WaitMessage)
		}
	}
}

// Trim messages to a maximum length, only keeping the last maxMsg number of user messages
func trimMsg(messages []ChatCompletionMessage, maxMsg int) []ChatCompletionMessage {
	log.Printf("Trimming messages to a maximum of %d\n", maxMsg)

	if len(messages) == 0 {
		return messages
	}

	fmt.Println("messages length: ", len(messages))

	// Always preserve the first message (usually system message)
	firstMsg := messages[0]
	var temp []ChatCompletionMessage
	userMsgCount := 0

	// Iterate from the end backwards, skipping the first message
	for i := len(messages) - 1; i >= 1; i-- {
		// Check if we've reached the maximum user messages before adding
		if messages[i].Role == ChatMessageRoleUser {
			if userMsgCount >= maxMsg {
				log.Println("Reached maximum number of user messages to keep.")
				break
			}
			userMsgCount++
		}

		temp = append(temp, messages[i])
	}

	// If no user messages were found, return original messages
	if userMsgCount == 0 {
		return messages
	}

	// Reverse temp to restore chronological order
	for j := 0; j < len(temp)/2; j++ {
		temp[j], temp[len(temp)-1-j] = temp[len(temp)-1-j], temp[j]
	}

	// Create result with first message at the beginning
	result := make([]ChatCompletionMessage, 0, len(temp)+1)
	result = append(result, firstMsg)
	result = append(result, temp...)

	return result
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

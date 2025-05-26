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
		log.Printf("WARN: No tools available for the model to use.")
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
		log.Printf("ERROR: Received an empty response from API on initial request.")
		return "", fmt.Errorf("received an empty response from API")
	}

	choice := resp.Choices[0]

	interations := 0
	// Check if the model wants to use tools
	for choice.FinishReason == openai.FinishReasonToolCalls && len(choice.Message.ToolCalls) > 0 {
		interations++
		if interations > MaxToolCallIterations {
			log.Printf("ERROR: Maximum tool call iterations (%d) exceeded.", MaxToolCallIterations)
			return "", fmt.Errorf("maximum tool call iterations exceeded")
		}

		log.Printf("INFO: Model wants to use tools. Iteration: %d, Number of tool calls: %d.", interations, len(choice.Message.ToolCalls))

		*messages = append(*messages, choice.Message) // Add assistant's message with tool calls
		var toolResponses []openai.ChatCompletionMessage

		for _, toolCall := range choice.Message.ToolCalls {
			log.Printf("DEBUG: Processing tool call ID: %s, Function: %s, Arguments: %s", toolCall.ID, toolCall.Function.Name, toolCall.Function.Arguments)
			resultString, err := runFunctionCall(toolCall, myBot)

			if err != nil {
				// Log the error from runFunctionCall itself (e.g., failed to execute, not necessarily error from the tool's logic)
				log.Printf("ERROR: Error running function call %s (%s): %v", toolCall.ID, toolCall.Function.Name, err)
				// It's important that resultString still contains an error message for the LLM in this case.
				// The runFunctionCall is expected to produce a string indicating the error to the LLM.
			} else {
				log.Printf("INFO: Successfully executed tool call %s (%s).", toolCall.ID, toolCall.Function.Name)
			}
			log.Printf("DEBUG: Result for tool call %s: %s", toolCall.ID, resultString)

			toolResponses = append(toolResponses, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    resultString, // This content should be the actual result or an error message from the tool's execution.
				ToolCallID: toolCall.ID,
			})
		}

		*messages = append(*messages, toolResponses...) // Add tool responses to history

		log.Printf("INFO: Sending follow-up request to OpenRouter with %d tool responses.", len(toolResponses))
		followUpReq := openai.ChatCompletionRequest{
			Model:    OpenRouterModel,
			Messages: *messages,
			Tools:    availableTools,
		}

		finalResp, finalErr := client.CreateChatCompletion(context.Background(), followUpReq)

		if finalErr != nil {
			log.Printf("ERROR: Error creating follow-up chat completion: %v", finalErr)
			return "", fmt.Errorf("error creating follow-up chat completion: %w", finalErr)
		}

		if len(finalResp.Choices) == 0 {
			log.Printf("ERROR: Received an empty response from OpenRouter on follow-up request.")
			return "", fmt.Errorf("received an empty response from openRouter on follow-up")
		}

		*messages = append(*messages, finalResp.Choices[0].Message) // Add assistant's final response
		choice = finalResp.Choices[0]

		if choice.FinishReason == openai.FinishReasonToolCalls {
			log.Printf("INFO: Model wants to use tools again. Continuing iteration.")
			continue
		}

		log.Printf("INFO: Model finished generating response after tool calls. Finish reason: %s.", choice.FinishReason)
		return finalResp.Choices[0].Message.Content, nil
	}

	// This block is reached if the first response did not involve tool calls
	if choice.FinishReason == openai.FinishReasonStop {
		*messages = append(*messages, resp.Choices[0].Message) // Add assistant's message
		log.Printf("INFO: Model finished generating response (no tool calls). Finish reason: %s.", choice.FinishReason)
		return choice.Message.Content, nil
	}

	log.Printf("WARN: Model stopped for an unexpected reason: %s. Content: %s", choice.FinishReason, choice.Message.Content)
	*messages = append(*messages, resp.Choices[0].Message) // Still add the message
	return resp.Choices[0].Message.Content, nil
}

func runFunctionCall(toolCall openai.ToolCall, myBot *bot.Bot) (string, error) {
	var resultString string
	var err error

	// Check if the tool call is a function call

	if strings.Contains(toolCall.Function.Name, "reminder") {
		log.Printf("INFO: Reminder call detected. ToolCall ID: %s, Function: %s", toolCall.ID, toolCall.Function.Name)
		remindersMutex.Lock()
		resultString, err = tools.HandleReminderCall(toolCall, &reminders, myBot)
		remindersMutex.Unlock()

	} else if strings.Contains(toolCall.Function.Name, "song") {
		log.Printf("INFO: Music call detected. ToolCall ID: %s, Function: %s", toolCall.ID, toolCall.Function.Name)
		songMapMutex.Lock()
		resultString, err = tools.HandleMusicCall(toolCall, &songMap, myBot)
		songMapMutex.Unlock()

	} else if strings.Contains(toolCall.Function.Name, "voice") {
		log.Printf("INFO: Voice channel call detected. ToolCall ID: %s, Function: %s", toolCall.ID, toolCall.Function.Name)
		resultString, err = tools.HandleVoiceChannel(toolCall, myBot)
	} else {
		log.Printf("INFO: Normal function call detected. ToolCall ID: %s, Function: %s", toolCall.ID, toolCall.Function.Name)
		resultString, err = tools.ExecuteToolCall(toolCall)
	}

	// The error 'err' from tool execution (e.g., tool's internal logic error) is distinct from
	// an error in runFunctionCall itself (e.g., unknown tool).
	// The SendMessage function already logs if 'err' is not nil, along with toolCall.ID and Function.Name.
	// So, no redundant logging of 'err' here.
	return resultString, err
}

func SplitString(s string, chunkSize int) []string {
	if chunkSize <= 0 {
		log.Printf("WARN: Invalid chunk size %d, using default %d.", chunkSize, DefaultChunkSize)
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
		log.Printf("ERROR: Error loading reminders from file '%s': %v", "reminders.json", err)
		log.Printf("ERROR: MessageLoop cannot start without reminders. Returning.")
		return
	}
	log.Printf("INFO: Reminders loaded successfully.")

	err = music.LoadSongMapFromFile(&songMap) // Assuming this function exists and is similar to LoadReminders
	if err != nil {
		log.Printf("ERROR: Error loading song map from file: %v", err) // Add filename if available
		log.Printf("ERROR: MessageLoop cannot start without song map. Returning.")
		return
	}
	log.Printf("INFO: Song map loaded successfully.")

	if messages == nil {
		log.Printf("INFO: MessageLoop: messages map is nil, initializing.")
		messages = initRouter()
		// It's better to save history only when it changes.
		// If initRouter() creates an empty map, saving it now is not essential.
	}

	log.Printf("INFO: MessageLoop started. Waiting for messages on channel.")
	for {
		select {
		case <-ctx.Done():
			log.Printf("INFO: MessageLoop: Shutdown signal received via context.Done().")
			return

		case userInput, ok := <-messageChannel:
			if !ok {
				log.Printf("INFO: MessageLoop: messageChannel was closed. Exiting loop.")
				return
			}

			userID := userInput.Message.Author.ID
			channelID := userInput.Message.ChannelID
			guildID := userInput.Message.GuildID // Assuming GuildID is part of userInput.Message

			log.Printf("DEBUG: Received message from UserID: %s, ChannelID: %s, GuildID: %s", userID, channelID, guildID)

			if *userInput.IsForget {
				log.Printf("INFO: Forget command received for UserID: %s in ChannelID: %s.", userID, channelID)
				messages[userID] = setInitialMessages(instructions, userID)
				if err := storage.SaveChatHistory(messages, chatFilepath); err != nil {
					log.Printf("ERROR: Failed to save chat history after forget command for UserID %s: %v", userID, err)
				} else {
					log.Printf("INFO: Chat history saved after forget command for UserID %s.", userID)
				}
				go Mybot.RespondToMessage(channelID, "Your message history with me has been cleared.", userInput.Message.Reference(), userInput.WaitMessage)
				continue
			}

			// Construct user message with all relevant context for the LLM
			parsedUserMsgContent, isSkip := parseUserInput(userInput.Message.Content)
			if isSkip {
				log.Printf("DEBUG: Skipping empty or whitespace message from UserID %s in ChannelID %s.", userID, channelID)
				// Important: If we skip, we must handle the WaitMessage, otherwise it will hang around.
				if userInput.WaitMessage != nil {
					// Assuming Mybot has a method to just delete the wait message without sending a new one.
					// If not, this needs to be implemented in bot.go
					// For now, let's assume Mybot.RespondToMessage handles waitMessage deletion even with empty response.
					// A dedicated Mybot.DeleteWaitMessage(channelID, messageID) would be cleaner.
					go Mybot.RespondToMessage(channelID, "", userInput.Message.Reference(), userInput.WaitMessage)
				}
				continue
			}
			// Full context for the LLM
			parsedUserMsg := fmt.Sprintf("UserID: %s, UserName: %s, GuildID: %s, ChannelID: %s, says: %s", userID, userInput.Message.Author.Username, guildID, channelID, parsedUserMsgContent)
			log.Printf("DEBUG: Parsed user message for LLM: %s", parsedUserMsg)


			currentMessages, userExists := messages[userID]
			if !userExists {
				log.Printf("INFO: Initializing new conversation history for UserID: %s.", userID)
				currentMessages = setInitialMessages(instructions, userID)
			}

			// Create a new slice for updated messages to avoid modifying the original slice in map directly during SendMessage
			updatedMessages := make([]ChatCompletionMessage, len(currentMessages))
			copy(updatedMessages, currentMessages)

			updatedMessages = append(updatedMessages, ChatCompletionMessage{
				Role:    ChatMessageRoleUser,
				Content: parsedUserMsg, // This is the full contextualized message
			})

			// aiResponseContent contains the actual text reply from the LLM
			aiResponseContent, err := SendMessage(client, &updatedMessages, Mybot) // updatedMessages is modified by SendMessage

			if err != nil {
				log.Printf("ERROR: Error from SendMessage for UserID %s: %v", userID, err)
				// Try to roll back the user's last message from history if an error occurs.
				// messages[userID] at this point still holds the history *before* the problematic user message + AI response.
				// So, if currentMessages was correctly snapshotting that, we can revert.
				messages[userID] = currentMessages
				go Mybot.RespondToMessage(channelID, "Sorry, I encountered an error processing your message. Please try again.", userInput.Message.Reference(), userInput.WaitMessage)
				continue // Prevents saving potentially corrupted history (though SendMessage might have already added to updatedMessages)
			}

			// SendMessage has modified updatedMessages to include the user's prompt, tool interactions, and AI's final response.
			messages[userID] = updatedMessages

			trimmedMessages := trimMsg(messages[userID], MaxMessagesToKeep)
			messages[userID] = trimmedMessages

			if err := storage.SaveChatHistory(messages, chatFilepath); err != nil {
				log.Printf("ERROR: Failed to save chat history for UserID %s: %v", userID, err)
			} else {
				log.Printf("INFO: Chat history saved for UserID %s.", userID)
			}

			log.Printf("INFO: Response to UserID %s in ChannelID %s: \"%s\"", userID, channelID, aiResponseContent)

			if len(aiResponseContent) > MaxMessageLength {
				log.Printf("INFO: AI response for UserID %s is long (%d chars), splitting.", userID, len(aiResponseContent))
				chunks := SplitString(aiResponseContent, MaxMessageLength)
				go Mybot.RespondToLongMessage(userInput.Message.ChannelID, chunks, userInput.Message.Reference(), userInput.WaitMessage)
				continue
			}

			go Mybot.RespondToMessage(userInput.Message.ChannelID, aiResponseContent, userInput.Message.Reference(), userInput.WaitMessage)
		}
	}
}

// Trim messages to a maximum length, only keeping the last maxMsg number of *user* messages
// and their associated assistant/tool messages.
func trimMsg(messages []ChatCompletionMessage, maxUserMessagesToKeep int) []ChatCompletionMessage {
	if len(messages) <= 1 { // Always keep the system message if it's the only one
		return messages
	}

	log.Printf("DEBUG: Trimming messages. Original length: %d. Max user messages to keep: %d.", len(messages), maxUserMessagesToKeep)

	firstMsg := messages[0] // Assume this is the system prompt
	if firstMsg.Role != ChatMessageRoleSystem {
		log.Printf("WARN: trimMsg: First message is not System role. Actual: %s. Trimming might behave unexpectedly.", firstMsg.Role)
	}

	// We want to keep the system prompt + the interactions related to the last 'maxUserMessagesToKeep' user messages.
	var userMessageIndices []int
	for i, msg := range messages {
		if msg.Role == ChatMessageRoleUser {
			userMessageIndices = append(userMessageIndices, i)
		}
	}

	if len(userMessageIndices) <= maxUserMessagesToKeep {
		log.Printf("DEBUG: No trimming needed, %d user messages found, less than or equal to max %d.", len(userMessageIndices), maxUserMessagesToKeep)
		return messages // No trimming needed if user messages are within limit
	}

	// Determine the index of the (N-maxUserMessagesToKeep)-th user message. We keep messages from this point onwards.
	// For example, if 5 user messages [u1, u2, u3, u4, u5] and max is 2, we keep u4 and u5.
	// So, cutOffUserMessageIndex is index of u3. We take messages *after* u3's full interaction block.
	// This is tricky because assistant responses and tool calls follow the user message.
	// A simpler, though less precise way, is to find the start index of the (maxUserMessagesToKeep)-th user message from the end.
	
	startIndexForKeptHistory := userMessageIndices[len(userMessageIndices)-maxUserMessagesToKeep]

	log.Printf("DEBUG: Trimming: Keeping messages from index %d onwards (this is the %d-th user message from the end).", startIndexForKeptHistory, maxUserMessagesToKeep)

	trimmedHistory := make([]ChatCompletionMessage, 0)
	trimmedHistory = append(trimmedHistory, firstMsg) // Add system prompt
	trimmedHistory = append(trimmedHistory, messages[startIndexForKeptHistory:]...)

	log.Printf("DEBUG: Messages trimmed from %d to %d items.", len(messages), len(trimmedHistory))
	return trimmedHistory
}


func parseUserInput(userInput string) (parsed string, skip bool) {
	// This function is simple and pure, extensive logging not typically needed here.
	// Caller can log if 'skip' is true.
	userInput = strings.TrimSpace(userInput)
	if userInput == "" {
		return "", true
	}
	return userInput, false
}

func initRouter() map[string][]ChatCompletionMessage {
	log.Printf("INFO: initRouter: Initializing new chat history map.")
	messages := make(map[string][]ChatCompletionMessage)
	return messages
}

func setInitialMessages(instructions string, userID string) []ChatCompletionMessage {
	log.Printf("INFO: setInitialMessages: Setting initial system message for UserID %s.", userID)
	return []ChatCompletionMessage{
		{
			Role:    ChatMessageRoleSystem,
			Content: "You are talking to: " + userID + "\n" + instructions,
		},
	}
}

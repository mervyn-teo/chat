package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"untitled/internal/openai"
)

func main() {
	apiKey, err := openai.LoadAPIKey("settings.json")
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	client, err := openai.CreateClient(apiKey)
	if err != nil {
		log.Fatalf("Failed to create OpenAI client: %v", err)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Chatting with %s via OpenRouter. Type 'quit' or press Ctrl+D to exit.\n", openai.OpenRouterModel)

	var messages []openai.ChatCompletionMessage

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: "You are a helpful assistant.",
	})

	for {
		fmt.Print("You: ")
		userInput, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println("\nGoodbye!")
				break
			}
			log.Printf("Error reading input: %v", err)
			continue
		}

		userInput = strings.TrimSpace(userInput)

		if strings.ToLower(userInput) == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		if userInput == "" {
			continue
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: userInput,
		})

		aiResponseContent, err := openai.SendMessage(client, messages)
		if err != nil {
			log.Printf("Error getting response from OpenRouter: %v", err)
			if len(messages) > 0 {
				messages = messages[:len(messages)-1]
			}
			continue
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: aiResponseContent,
		})

		fmt.Printf("AI: %s\n", aiResponseContent)
	}
}

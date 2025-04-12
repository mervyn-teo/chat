package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http" // Needed for custom headers and client
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// --- OpenRouter Configuration ---
const (
	openRouterBaseURL = "https://openrouter.ai/api/v1"
	openRouterModel   = "deepseek/deepseek-r1:free"     // Example
	refererURL        = "http://localhost"  // Replace if needed
	appTitle          = "Go OpenRouter CLI" // Replace if needed
)

// --- End OpenRouter Configuration ---

// Settings struct
type Settings struct {
	API_KEY string `json:"api_key"`
}

// loadAPIKey function (no changes needed)
func loadAPIKey(filePath string) (string, error) {
	var settings Settings

	jsonFile, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening settings file '%s': %w", filePath, err)
	}

	defer func(jsonFile *os.File) {
		err := jsonFile.Close()
		if err != nil {

		}
	}(jsonFile)

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return "", fmt.Errorf("error reading settings file: %w", err)
	}

	err = json.Unmarshal(byteValue, &settings)
	if err != nil {
		return "", fmt.Errorf("error decoding settings JSON: %w", err)
	}

	if settings.API_KEY == "" {
		return "", fmt.Errorf("api_key not found or empty in settings file (expected OpenRouter key)")
	}

	return settings.API_KEY, nil
}

// --- Custom HTTP Transport for Headers ---

type headerTransport struct {
	Transport http.RoundTripper // Base transport
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Ensure base transport exists, default to http.DefaultTransport
	base := t.Transport
	if base == nil {
		base = http.DefaultTransport
	}

	// Add custom headers
	req.Header.Set("HTTP-Referer", refererURL)
	req.Header.Set("X-Title", appTitle)
	// Note: The go-openai library adds the Authorization header itself.

	// Proceed with the request using the base transport
	return base.RoundTrip(req)
}

// --- Main Application Logic ---

func main() {
	apiKey, err := loadAPIKey("settings.json")
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	// --- Corrected Client Configuration ---
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = openRouterBaseURL

	// 1. Create the custom transport
	customTransport := &headerTransport{
		Transport: http.DefaultTransport, // Use the default transport as base
	}

	// 2. Create an http.Client that USES the custom transport
	httpClient := &http.Client{
		Transport: customTransport,
	}

	// 3. Assign this custom *http.Client* to the config's HTTPClient field.
	//    *http.Client satisfies the openai.HTTPDoer interface.
	config.HTTPClient = httpClient
	// --- End Corrected Client Configuration ---

	// Create the client with the modified configuration
	client := openai.NewClientWithConfig(config)

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Chatting with %s via OpenRouter. Type 'quit' or press Ctrl+D to exit.\n", openRouterModel)

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

		aiResponseContent, err := sendMessage(client, messages)
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

// sendMessage function (no changes needed)
func sendMessage(client *openai.Client, messages []openai.ChatCompletionMessage) (string, error) {
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    openRouterModel,
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

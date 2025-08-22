package router

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"untitled/internal/bot"
	"untitled/internal/music"
	"untitled/internal/reminder"

	openai "github.com/sashabaranov/go-openai"
)

// Mock OpenAI server
func createMockOpenAIServer(responses []openai.ChatCompletionResponse) *httptest.Server {
	responseIndex := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if responseIndex >= len(responses) {
			responseIndex = len(responses) - 1
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(responses[responseIndex])
		if err != nil {
			return
		}
		responseIndex++
	}))
}

func TestSendMessage(t *testing.T) {
	tests := []struct {
		name           string
		client         *openai.Client
		messages       *[]ChatCompletionMessage
		bot            *bot.Bot
		mockResponses  []openai.ChatCompletionResponse
		expectedError  bool
		expectedResult string
	}{
		{
			name:          "nil client",
			client:        nil,
			messages:      &[]ChatCompletionMessage{},
			bot:           &bot.Bot{},
			expectedError: true,
		},
		{
			name:          "nil messages",
			client:        &openai.Client{},
			messages:      nil,
			bot:           &bot.Bot{},
			expectedError: true,
		},
		{
			name:          "nil bot",
			client:        &openai.Client{},
			messages:      &[]ChatCompletionMessage{},
			bot:           nil,
			expectedError: true,
		},
		{
			name:     "successful response",
			messages: &[]ChatCompletionMessage{},
			bot:      &bot.Bot{},
			mockResponses: []openai.ChatCompletionResponse{
				{
					Choices: []openai.ChatCompletionChoice{
						{
							Message: openai.ChatCompletionMessage{
								Content: "Hello, world!",
							},
							FinishReason: openai.FinishReasonStop,
						},
					},
				},
			},
			expectedError:  false,
			expectedResult: "Hello, world!",
		},
		{
			name:     "empty response",
			messages: &[]ChatCompletionMessage{},
			bot:      &bot.Bot{},
			mockResponses: []openai.ChatCompletionResponse{
				{
					Choices: []openai.ChatCompletionChoice{},
				},
			},
			expectedError: true,
		},
		{
			name:     "tool call response",
			messages: &[]ChatCompletionMessage{},
			bot:      &bot.Bot{},
			mockResponses: []openai.ChatCompletionResponse{
				{
					Choices: []openai.ChatCompletionChoice{
						{
							Message: openai.ChatCompletionMessage{
								ToolCalls: []openai.ToolCall{
									{
										ID: "test-tool-call",
										Function: openai.FunctionCall{
											Name:      "test_function",
											Arguments: `{"arg": "value"}`,
										},
									},
								},
							},
							FinishReason: openai.FinishReasonToolCalls,
						},
					},
				},
				{
					Choices: []openai.ChatCompletionChoice{
						{
							Message: openai.ChatCompletionMessage{
								Content: "Tool call completed",
							},
							FinishReason: openai.FinishReasonStop,
						},
					},
				},
			},
			expectedError:  false,
			expectedResult: "Tool call completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockResponses != nil {
				server := createMockOpenAIServer(tt.mockResponses)
				defer server.Close()

				config := openai.DefaultConfig("test-token")
				config.BaseURL = server.URL + "/v1"
				tt.client = openai.NewClientWithConfig(config)
			}

			result, err := SendMessage(tt.client, tt.messages, tt.bot)

			if tt.expectedError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectedError && result != tt.expectedResult {
				t.Errorf("Expected result %q, got %q", tt.expectedResult, result)
			}
		})
	}
}

func TestSplitString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		chunkSize int
		expected  []string
	}{
		{
			name:      "empty string",
			input:     "",
			chunkSize: 10,
			expected:  []string{""},
		},
		{
			name:      "string shorter than chunk size",
			input:     "hello",
			chunkSize: 10,
			expected:  []string{"hello"},
		},
		{
			name:      "string equal to chunk size",
			input:     "hello world",
			chunkSize: 11,
			expected:  []string{"hello world"},
		},
		{
			name:      "string longer than chunk size",
			input:     "hello world this is a test",
			chunkSize: 10,
			expected:  []string{"hello worl", "d this is ", "a test"},
		},
		{
			name:      "invalid chunk size uses default",
			input:     "test",
			chunkSize: 0,
			expected:  []string{"test"},
		},
		{
			name:      "unicode characters",
			input:     "こんにちは世界",
			chunkSize: 3,
			expected:  []string{"こんに", "ちは世", "界"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitString(tt.input, tt.chunkSize)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTrimMsg(t *testing.T) {
	tests := []struct {
		name     string
		messages []ChatCompletionMessage
		maxMsg   int
		expected int // expected length after trimming
	}{
		{
			name:     "empty messages",
			messages: []ChatCompletionMessage{},
			maxMsg:   5,
			expected: 0,
		},
		{
			name: "messages under limit",
			messages: []ChatCompletionMessage{
				{Role: ChatMessageRoleSystem, Content: "system"},
				{Role: ChatMessageRoleUser, Content: "user1"},
				{Role: "assistant", Content: "assistant1"},
			},
			maxMsg:   5,
			expected: 3,
		},
		{
			name: "messages over limit",
			messages: []ChatCompletionMessage{
				{Role: ChatMessageRoleSystem, Content: "system"},
				{Role: ChatMessageRoleUser, Content: "user1"},
				{Role: "assistant", Content: "assistant1"},
				{Role: ChatMessageRoleUser, Content: "user2"},
				{Role: "assistant", Content: "assistant2"},
				{Role: ChatMessageRoleUser, Content: "user3"},
				{Role: "assistant", Content: "assistant3"},
			},
			maxMsg:   2,
			expected: 6, // system + last 2 user messages + their responses
		},
		{
			name: "only system message",
			messages: []ChatCompletionMessage{
				{Role: ChatMessageRoleSystem, Content: "system"},
			},
			maxMsg:   5,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimMsg(tt.messages, tt.maxMsg)
			if len(result) != tt.expected {
				t.Errorf("Expected length %d, got %d", tt.expected, len(result))
			}

			// Ensure system message is preserved if it exists
			if len(tt.messages) > 0 && tt.messages[0].Role == ChatMessageRoleSystem {
				if len(result) == 0 || result[0].Role != ChatMessageRoleSystem {
					t.Error("System message should be preserved")
				}
			}
		})
	}
}

func TestParseUserInput(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedText string
		expectedSkip bool
	}{
		{
			name:         "normal input",
			input:        "hello world",
			expectedText: "hello world",
			expectedSkip: false,
		},
		{
			name:         "input with whitespace",
			input:        "  hello world  ",
			expectedText: "hello world",
			expectedSkip: false,
		},
		{
			name:         "empty input",
			input:        "",
			expectedText: "",
			expectedSkip: true,
		},
		{
			name:         "whitespace only",
			input:        "   ",
			expectedText: "",
			expectedSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, skip := parseUserInput(tt.input)
			if text != tt.expectedText {
				t.Errorf("Expected text %q, got %q", tt.expectedText, text)
			}
			if skip != tt.expectedSkip {
				t.Errorf("Expected skip %v, got %v", tt.expectedSkip, skip)
			}
		})
	}
}

func TestInitRouter(t *testing.T) {
	result := initRouter()
	if result == nil {
		t.Error("Expected non-nil map")
	}
	if len(result) != 0 {
		t.Error("Expected empty map")
	}
}

func TestSetInitialMessages(t *testing.T) {
	instructions := "You are a helpful assistant"
	userID := "user123"

	result := setInitialMessages(instructions, userID)

	if len(result) != 1 {
		t.Errorf("Expected 1 message, got %d", len(result))
	}

	if result[0].Role != ChatMessageRoleSystem {
		t.Errorf("Expected system role, got %s", result[0].Role)
	}

	expectedContent := "You are talking to: " + userID + "\n" + instructions
	if result[0].Content != expectedContent {
		t.Errorf("Expected content %q, got %q", expectedContent, result[0].Content)
	}
}

func TestRunFunctionCall(t *testing.T) {
	mockBot := &bot.Bot{}

	tests := []struct {
		name         string
		toolCall     openai.ToolCall
		expectedErr  bool
		setupGlobals func()
	}{
		{
			name: "reminder function call",
			toolCall: openai.ToolCall{
				ID: "test-reminder",
				Function: openai.FunctionCall{
					Name:      "reminder_function",
					Arguments: `{}`,
				},
			},
			setupGlobals: func() {
				reminders = reminder.ReminderList{}
			},
		},
		{
			name: "song function call",
			toolCall: openai.ToolCall{
				ID: "test-song",
				Function: openai.FunctionCall{
					Name:      "song_function",
					Arguments: `{}`,
				},
			},
			setupGlobals: func() {
				songMap = make(map[string]map[string]*music.SongList)
			},
		},
		{
			name: "voice function call",
			toolCall: openai.ToolCall{
				ID: "test-voice",
				Function: openai.FunctionCall{
					Name:      "voice_function",
					Arguments: `{}`,
				},
			},
		},
		{
			name: "unknown function call",
			toolCall: openai.ToolCall{
				ID: "test-unknown",
				Function: openai.FunctionCall{
					Name:      "unknown_function",
					Arguments: `{}`,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupGlobals != nil {
				tt.setupGlobals()
			}

			result, err := runFunctionCall(tt.toolCall, mockBot)

			if tt.expectedErr && err == nil {
				t.Error("Expected error but got none")
			}

			// Result should be a string (even if empty)
			if result == "" && err == nil {
				// This might be expected for some function calls
			}
		})
	}
}

func TestMessageLoop(t *testing.T) {
	// Create temporary files for testing
	tempChatFile, err := os.CreateTemp("", "chat_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {

		}
	}(tempChatFile.Name())
	err = tempChatFile.Close()
	if err != nil {
		return
	}

	// Create mock bot
	//mockBot := &mockBot{}

	// Create mock OpenAI client
	server := createMockOpenAIServer([]openai.ChatCompletionResponse{
		{
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Content: "Hello, user!",
					},
					FinishReason: openai.FinishReasonStop,
				},
			},
		},
	})
	defer server.Close()

	config := openai.DefaultConfig("test-token")
	config.BaseURL = server.URL + "/v1"
	client := openai.NewClientWithConfig(config)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Create message channel
	messageChannel := make(chan *bot.MessageForCompletion, 10)

	// Initialize messages map
	messages := make(map[string][]ChatCompletionMessage)

	// Start message loop in goroutine
	clients := &Clients{}
	clients.BaseClient = client
	go MessageLoop(ctx, &bot.Bot{}, clients, messageChannel, "Test instructions", messages, tempChatFile.Name(), "test init message", nil)

	// Test normal message
	testMessage := &bot.MessageForCompletion{
		Message: &discordgo.MessageCreate{
			Message: &discordgo.Message{
				Author: &discordgo.User{
					ID:       "user123",
					Username: "testuser",
				},
				Content:   "Hello",
				ChannelID: "channel123",
				GuildID:   "guild123",
			},
		},
		IsForget:    &[]bool{false}[0],
		WaitMessage: nil,
	}

	// Send test message
	select {
	case messageChannel <- testMessage:
	case <-time.After(1 * time.Second):
		t.Fatal("Failed to send message to channel")
	}

	// Test forget command
	forgetMessage := &bot.MessageForCompletion{
		Message: &discordgo.MessageCreate{
			Message: &discordgo.Message{
				Author: &discordgo.User{
					ID:       "user123",
					Username: "testuser",
				},
				Content:   "forget",
				ChannelID: "channel123",
				GuildID:   "guild123",
			},
		},
		IsForget:    &[]bool{true}[0],
		WaitMessage: nil,
	}

	select {
	case messageChannel <- forgetMessage:
	case <-time.After(1 * time.Second):
		t.Fatal("Failed to send forget message to channel")
	}

	// Wait for context to be done
	<-ctx.Done()

	// Verify that messages were processed
	if len(messages) == 0 {
		t.Error("Expected messages to be processed")
	}
}

func TestConcurrentAccess(t *testing.T) {
	// Test concurrent access to global variables
	var wg sync.WaitGroup

	// Test concurrent reminder access
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			remindersMutex.Lock()
			// Simulate some work
			time.Sleep(1 * time.Millisecond)
			remindersMutex.Unlock()
		}()
	}

	// Test concurrent song map access
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			songMapMutex.Lock()
			// Simulate some work
			time.Sleep(1 * time.Millisecond)
			songMapMutex.Unlock()
		}()
	}

	wg.Wait()
}

func TestConstants(t *testing.T) {
	intConstTest := []struct {
		name     string
		constant int
		expected int
	}{
		{
			name:     "MaxMessageLength",
			constant: MaxMessageLength,
			expected: 1900,
		},
		{
			name:     "MaxMessagesToKeep",
			constant: MaxMessagesToKeep,
			expected: 20,
		},
		{
			name:     "MaxToolCallIterations",
			constant: MaxToolCallIterations,
			expected: 10,
		},
		{
			name:     "DefaultChunkSize",
			constant: DefaultChunkSize,
			expected: 1900,
		},
	}

	for _, tt := range intConstTest {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected %s to be %d, got %d",
					tt.name, tt.expected, tt.constant)
			}
		})
	}

	textContTest := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "OpenRouterBaseURL",
			constant: OpenRouterBaseURL,
			expected: "https://openrouter.ai/api/v1",
		},
		{
			name:     "RefererURL",
			constant: RefererURL,
			expected: "http://localhost",
		},
		{
			name:     "AppTitle",
			constant: AppTitle,
			expected: "Go OpenRouter CLI",
		},
	}

	for _, tt := range textContTest {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected %s to be %q, got %q",
					tt.name, tt.expected, tt.constant)
			}
		})
	}

}

// Benchmark tests
func BenchmarkSplitString(b *testing.B) {
	longString := strings.Repeat("Hello, world! ", 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SplitString(longString, 1900)
	}
}

func BenchmarkTrimMsg(b *testing.B) {
	messages := make([]ChatCompletionMessage, 100)
	for i := range messages {
		if i%2 == 0 {
			messages[i] = ChatCompletionMessage{Role: ChatMessageRoleUser, Content: fmt.Sprintf("User message %d", i)}
		} else {
			messages[i] = ChatCompletionMessage{Role: "assistant", Content: fmt.Sprintf("Assistant message %d", i)}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trimMsg(messages, 20)
	}
}

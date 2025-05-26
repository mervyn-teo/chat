package main

import (
	"context"
	"github.com/sashabaranov/go-openai"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"untitled/internal/bot"
	"untitled/internal/router"
	"untitled/internal/storage"
)

var (
	client   *openai.Client
	settings storage.Settings
	messages map[string][]openai.ChatCompletionMessage
)

func init() {
	var err error
	// Load the API key from the settings file
	var err error
	settings, err = storage.LoadSettings("settings.json")
	if err != nil {
		log.Fatalf("FATAL: Initialization failed - could not load settings.json: %v", err)
	}
	log.Printf("INFO: Settings loaded successfully from settings.json.")

	// Check if the chat history file exists, if not create it
	if !storage.CheckFileExistence(settings.ChatHistoryFilePath) {
		log.Printf("INFO: Chat history file '%s' not found, attempting to create.", settings.ChatHistoryFilePath)
		if err := storage.CreateChatHistoryFile(settings.ChatHistoryFilePath); err != nil {
			log.Fatalf("FATAL: Failed to create chat history file '%s': %v", settings.ChatHistoryFilePath, err)
		}
		log.Printf("INFO: Chat history file '%s' created successfully.", settings.ChatHistoryFilePath)
	}

	messages = storage.ReadChatHistory(settings.ChatHistoryFilePath) // Assuming this logs its own errors/success
	log.Printf("INFO: Chat history loaded from '%s'.", settings.ChatHistoryFilePath)

	// Create the OpenAI client
	client, err = router.CreateClient(settings.Model, settings.ApiKey)
	if err != nil {
		log.Fatalf("FATAL: Failed to create OpenAI client: %v", err)
	}
	log.Printf("INFO: OpenAI client created successfully for model '%s'.", settings.Model)
}

func main() {
	log.Printf("INFO: Starting application...")
	// Shared channel for communication between bot and router
	messageChannel := make(chan *bot.MessageWithWait)
	log.Printf("INFO: Message channel created.")

	// Create the bot instance
	myBot, err := bot.NewBot(settings.DiscordToken, messageChannel)
	if err != nil {
		log.Fatalf("FATAL: Failed to create Discord bot: %v", err)
	}
	log.Printf("INFO: Discord bot instance created.")

	// --- Graceful Shutdown Setup ---
	// Create a context that can be cancelled
	// Use context.WithCancel for manual cancellation trigger
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancel is called eventually

	// Use a WaitGroup to wait for goroutines to finish
	var wg sync.WaitGroup

	// Start the router loop in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done() // Signal completion when this goroutine exits
		// Pass the cancellable context to the loop
		router.MessageLoop(ctx, myBot, client, messageChannel, settings.Instructions, messages, settings.ChatHistoryFilePath)
		log.Printf("INFO: Router loop goroutine stopped.")
	}()

	// Start the bot in its own goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("INFO: Starting Discord bot event handling...")
		err := myBot.Start()
		if err != nil {
			// Log non-fatal error if shutdown was expected. Session.Close() often returns an error on normal shutdown.
			log.Printf("WARN: Bot Start() returned error (this may be expected during shutdown): %v", err)
		} else {
			log.Printf("INFO: Bot Start() completed without error (bot stopped gracefully).")
		}
		log.Printf("INFO: Bot event handling goroutine stopped.")
	}()

	log.Printf("INFO: Bot and Router routines started. Press Ctrl+C to exit.")

	// --- Signal Handling ---
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("INFO: Signal handler registered for SIGINT and SIGTERM.")

	// Block until a signal is received
	receivedSignal := <-sigChan
	log.Printf("INFO: Received signal: %s. Initiating graceful shutdown...", receivedSignal)

	// --- Initiate Shutdown ---
	log.Printf("INFO: Cancelling context to signal goroutines...")
	cancel() // Signal goroutines to stop by closing ctx.Done()

	log.Printf("INFO: Attempting to stop Discord bot session...")
	if err := myBot.Stop(); err != nil { // Assign err here
		log.Printf("ERROR: Error stopping Discord bot session: %v", err)
	} else {
		log.Printf("INFO: Discord bot session stopped successfully.")
	}

	// --- Wait for Goroutines to Finish ---
	log.Printf("INFO: Waiting for all goroutines to finish...")
	waitCompleteChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCompleteChan)
	}()

	select {
	case <-waitCompleteChan:
		log.Printf("INFO: All goroutines finished gracefully.")
	case <-time.After(10 * time.Second): // Timeout for shutdown
		log.Printf("WARN: Shutdown timed out after 10 seconds. Forcing exit.")
	}
	log.Printf("INFO: Application shutdown complete.")
}

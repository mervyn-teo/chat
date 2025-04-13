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
	apiKey        string
	client        *openai.Client
	discord_token string
	settings      storage.Settings
)

func init() {
	var err error
	// Load the API key from the settings file
	settings, err = storage.LoadSettings("settings.json")
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}
	apiKey = settings.ApiKey
	discord_token = settings.DiscordToken

	// Create the OpenAI client
	client, err = router.CreateClient(settings.Model, apiKey)
	if err != nil {
		log.Fatalf("Failed to create OpenAI client: %v", err)
	}
}

func main() {
	// Shared channel for communication between bot and router
	messageChannel := make(chan *bot.MessageWithWait)

	// Create the bot instance
	myBot, err := bot.NewBot(discord_token, messageChannel)
	if err != nil {
		log.Fatalf("Failed to create Discord bot: %v", err)
	}

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
		router.MessageLoop(ctx, myBot, client, messageChannel, settings.Instructions)
		log.Println("Router loop stopped.")
	}()

	// Start the bot in its own goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := myBot.Start()
		if err != nil {
			// Log non-fatal error if shutdown was expected
			log.Printf("Bot Start() returned error (may be expected on shutdown): %v", err)
		} else {
			log.Println("Bot stopped gracefully.")
		}
	}()

	log.Println("Bot and Router routines started. Press Ctrl+C to exit.")

	// --- Signal Handling ---
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received
	receivedSignal := <-sigChan
	log.Printf("Received signal: %s. Initiating shutdown...", receivedSignal)

	// --- Initiate Shutdown ---
	// 1. Cancel the context to signal goroutines
	cancel()

	// 2. Explicitly tell the bot to stop (replace with actual method)
	log.Println("Stopping Discord bot...")

	err = myBot.Stop() // Or myBot.Stop(), etc.
	if err != nil {
		log.Printf("Error stopping bot: %v", err)
	}

	// --- Wait for Goroutines to Finish ---
	log.Println("Waiting for routines to finish...")
	// You can add a timeout to prevent hanging indefinitely if a goroutine misbehaves
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()

	select {
	case <-waitChan:
		log.Println("All routines finished.")
	case <-time.After(10 * time.Second):
		log.Println("Shutdown timed out. Forcing exit.")
	}
	log.Println("Shutdown complete.")
}

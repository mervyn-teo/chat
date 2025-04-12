package main

import (
	"github.com/sashabaranov/go-openai"
	"log"
	"untitled/internal/bot"
	"untitled/internal/router"
	"untitled/internal/storage"
)

var apiKey string
var client *openai.Client
var discord_token string

func init() {
	// Load the API key from the settings file
	settings, err := storage.LoadSettings("settings.json")
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}
	apiKey = settings.ApiKey
	discord_token = settings.DiscordToken

	// Create the OpenAI client
	client, err = router.CreateClient(apiKey)
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

	// Start the router loop in a goroutine
	go router.MessageLoop(myBot, client, messageChannel)

	// Start the bot in its own goroutine
	// Start() will connect, add handlers, and block until shutdown
	go func() {
		err := myBot.Start()
		if err != nil {
			log.Fatalf("Failed to start Discord bot: %v", err)
		}
	}()

	log.Println("Bot and Router routines started. Press Ctrl+C to exit.")
	select {}
}

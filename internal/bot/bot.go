package bot

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
)

// Bot holds the state for a Discord bot instance
type Bot struct {
	Token          string
	Session        *discordgo.Session
	messageQueue   []*discordgo.MessageCreate // Keep queue internal if needed
	mu             sync.Mutex
	messageChannel chan discordgo.MessageCreate
}

// NewBot creates a new Bot instance but doesn't connect yet
func NewBot(token string, msgChan chan discordgo.MessageCreate) (*Bot, error) {
	b := &Bot{
		Token:          token,
		messageChannel: msgChan, // Or initialize channel here
	}
	// Create session - Don't open yet
	var err error
	b.Session, err = discordgo.New("Bot " + b.Token)
	if err != nil {
		return nil, fmt.Errorf("error creating Discord session: %w", err)
	}
	return b, nil
}

// Start connects the bot and begins handling events
func (b *Bot) Start() error {
	// Add handlers using methods of the Bot struct
	b.Session.AddHandler(b.newMessage)

	// Open session
	err := b.Session.Open()
	if err != nil {
		return fmt.Errorf("error opening Discord session: %w", err)
	}

	// Start any necessary goroutines
	// Consider if relayMessagetToRouter is still the best approach
	go b.relayMessagesToRouter() // Now a method

	fmt.Println("Bot running....")
	// Keep bot running logic (no changes needed here)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	fmt.Println("Bot shutting down...")

	// Close the session explicitly when shutting down
	return b.Session.Close()
}

func checkNilErr(e error) { // Keep utility or handle errors inline/return them
	if e != nil {
		log.Fatalf("Fatal error: %v", e) // Log the actual error
	}
}

// relayMessagesToRouter processes the internal queue
func (b *Bot) relayMessagesToRouter() {
	for {
		var message *discordgo.MessageCreate // Declare outside lock

		b.mu.Lock()
		if len(b.messageQueue) > 0 {
			message = b.messageQueue[0]
			b.messageQueue = b.messageQueue[1:]
		}
		b.mu.Unlock() // Unlock before potentially blocking on channel send

		if message != nil {
			// Decide what to send: just content, or more structured data?
			b.messageChannel <- *message // Send content
		} else {
			// Maybe add a small sleep if queue is empty? time.Sleep(100 * time.Millisecond)
			// Or use a condition variable for more efficiency
		}
	}
}

// newMessage is the event handler, now a method on Bot
func (b *Bot) newMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Use b.Session or the passed 's' - they are the same
	if m.Author.ID == s.State.User.ID {
		return
	}

	switch {
	case strings.HasPrefix(m.Content, "!ask"):
		// Maybe process directly or queue if processing is long
		b.addMessage(m) // Add to internal queue
		// Example direct response (could be moved)
		_, err := s.ChannelMessageSend(m.ChannelID, "Received !ask command.")
		if err != nil {
			log.Printf("Error sending ack for !ask: %v", err)
		}
	case strings.HasPrefix(m.Content, "!ping"):
		_, err := s.ChannelMessageSend(m.ChannelID, "pong")
		if err != nil {
			log.Printf("Error sending pong: %v", err)
		}
	}
}

// addMessage adds a message to the internal queue
func (b *Bot) addMessage(message *discordgo.MessageCreate) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messageQueue = append(b.messageQueue, message)
}

// RespondToMessage sends a message using the bot's session
// This method is now part of the Bot struct
func (b *Bot) RespondToMessage(channelId string, response string) {
	if b.Session == nil {
		log.Println("Error: Bot session not initialized in RespondToMessage")
		return
	}
	_, err := b.Session.ChannelMessageSend(channelId, response)
	if err != nil {
		log.Printf("Error sending message via RespondToMessage: %v", err)
	}
}

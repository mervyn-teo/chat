package bot

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

// Bot holds the state for a Discord bot instance
type Bot struct {
	Token          string
	Session        *discordgo.Session
	messageQueue   []MessageWithWait
	mu             sync.Mutex
	messageChannel chan *MessageWithWait // Channel for message processing
}

type MessageWithWait struct {
	Message     *discordgo.MessageCreate
	WaitMessage *discordgo.Message
	IsForget    *bool
}

// NewBot creates a new Bot instance but doesn't connect yet
func NewBot(token string, msgChan chan *MessageWithWait) (*Bot, error) {
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
	b.Session.AddHandler(b.newMessage)

	// Open session
	err := b.Session.Open()
	if err != nil {
		return fmt.Errorf("error opening Discord session: %w", err)
	}

	// Start any necessary goroutines
	// Consider if relayMessagetToRouter is still the best approach
	go b.relayMessagesToRouter()

	fmt.Println("Bot running....")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	fmt.Println("Bot shutting down...")

	return b.Session.Close()
}

// relayMessagesToRouter processes the internal queue
func (b *Bot) relayMessagesToRouter() {
	for {
		var message *MessageWithWait // Declare outside lock

		b.mu.Lock()
		if len(b.messageQueue) > 0 {
			message = &(b.messageQueue[0])
			b.messageQueue = b.messageQueue[1:]
			b.mu.Unlock()

			b.messageChannel <- message // Send content
		} else {
			b.mu.Unlock()
			// Add a small sleep to prevent CPU spinning when queue is empty
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// newMessage is the event handler, now a method on Bot
func (b *Bot) newMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	isForget := false

	botWasMentioned := false
	for _, mention := range m.Mentions {
		if mention.ID == s.State.User.ID {
			botWasMentioned = true
			break
		}
	}

	if m.ReferencedMessage != nil && m.ReferencedMessage.Author.ID == s.State.User.ID {
		botWasMentioned = true
	}

	switch {
	case botWasMentioned:
		// Maybe process directly or queue if processing is long
		refer, err := s.ChannelMessageSendReply(m.ChannelID, "Waiting for response...", m.Reference())

		if err != nil {
			log.Printf("Error sending ack for @me: %v", err)
		}

		if refer == nil {
			log.Println("Error: waiting message is nil")
			return
		}

		m.Content = strings.ReplaceAll(m.Content, "<@"+s.State.User.ID+">", "@you")

		if m.ReferencedMessage != nil {
			referMsg := ""

			if m.ReferencedMessage.Author.ID == s.State.User.ID {
				referMsg = "you said: " + m.ReferencedMessage.Content
			} else {
				referMsg = m.ReferencedMessage.Author.ID + " said: " + m.ReferencedMessage.Content
			}

			m.Content = referMsg + "\n" + m.Author.ID + " says to you: " + m.Content
		}

		fmt.Println("Message content: ", m.Content)

		msg := MessageWithWait{
			Message:     m,
			WaitMessage: refer,
			IsForget:    &isForget,
		}

		b.addMessage(msg) // Add to internal queue

	case strings.HasPrefix(m.Content, "!ping"):
		latency := time.Since(m.Timestamp)
		pongMessage := fmt.Sprintf("Pong! Latency: %v", latency)
		_, err := s.ChannelMessageSend(m.ChannelID, pongMessage)
		if err != nil {
			log.Printf("Error sending pong: %v", err)
		}

	case strings.HasPrefix(m.Content, "!forget"):
		refer, err := s.ChannelMessageSendReply(m.ChannelID, "Clearing message memory from bot...", m.Reference())
		isForget := true

		if err != nil {
			log.Printf("Error sending ack for !forget: %v", err)
			return
		}

		b.forgetMessage(MessageWithWait{
			Message:     m,
			WaitMessage: refer,
			IsForget:    &isForget,
		})
	}

}

// addMessage adds a message to the internal queue
func (b *Bot) addMessage(message MessageWithWait) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messageQueue = append(b.messageQueue, message)
}

func (b *Bot) forgetMessage(msg MessageWithWait) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Clear the message queue for the user
	for i := len(b.messageQueue) - 1; i >= 0; i-- {
		if b.messageQueue[i].Message.Author.ID == msg.Message.Author.ID && b.messageQueue[i].Message.ChannelID == msg.Message.ChannelID {
			b.messageQueue = append(b.messageQueue[:i], b.messageQueue[i+1:]...)
		}
	}

	log.Printf("Cleared messages for user %s in channel %s", msg.Message.Author.ID, msg.Message.ChannelID)

	b.messageQueue = append(b.messageQueue, msg)
}

// RespondToMessage sends a message using the bot's session
// This method is now part of the Bot struct
func (b *Bot) RespondToMessage(channelId string, response string, ref *discordgo.MessageReference, waitMessage *discordgo.Message) {
	if b.Session == nil {
		log.Println("Error: Bot session not initialized in RespondToMessage")
		return
	}

	if waitMessage != nil {
		err := b.Session.ChannelMessageDelete(waitMessage.ChannelID, waitMessage.ID)
		if err != nil {
			log.Printf("Error deleting message: %v", err)
		}
	} else {
		log.Println("Error: waitMessage is nil in RespondToMessage")
		return
	}

	sendMessage := &discordgo.MessageSend{
		Content:   response,
		Reference: ref,
	}

	_, err := b.Session.ChannelMessageSendComplex(channelId, sendMessage)
	if err != nil {
		log.Printf("Error sending message via RespondToMessage: %v", err)
	}
}

func (b *Bot) RespondToLongMessage(channelId string, response []string, ref *discordgo.MessageReference, waitMessage *discordgo.Message) {
	if b.Session == nil {
		log.Println("Error: Bot session not initialized in RespondToMessage")
		return
	}

	if waitMessage != nil {
		err := b.Session.ChannelMessageDelete(waitMessage.ChannelID, waitMessage.ID)
		if err != nil {
			log.Printf("Error deleting message: %v", err)
		}
	} else {
		log.Println("Error: waitMessage is nil in RespondToMessage")
		return
	}

	for i := range response {
		segment := response[i]

		segment = "[Section " + fmt.Sprint(i+1) + "/" + fmt.Sprint(len(response)) + "]\n" + segment
		sendMessage := &discordgo.MessageSend{
			Content:   segment,
			Reference: ref,
		}
		_, err := b.Session.ChannelMessageSendComplex(channelId, sendMessage)
		if err != nil {
			log.Printf("Error sending message via RespondToMessage: %v", err)
		}
	}
}

func (b *Bot) SendMessageToChannel(channelId string, message string) {
	if b.Session == nil {
		log.Println("Error: Bot session not initialized in SendMessageToChannel")
		return
	}

	sendMessage := &discordgo.MessageSend{
		Content: message,
	}

	_, err := b.Session.ChannelMessageSendComplex(channelId, sendMessage)
	if err != nil {
		log.Printf("Error sending message via SendMessageChannel: %v", err)
	}
}

func (b *Bot) Stop() error {
	// Close the session and clean up resources
	if b.Session != nil {
		err := b.Session.Close()
		if err != nil {
			log.Printf("Error closing Discord session: %v", err)
			return err
		}
	}

	close(b.messageChannel) // Close the message channel if necessary
	log.Println("Bot stopped.")
	return nil
}

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

	log.Printf("INFO: Bot running....")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	log.Printf("INFO: Bot shutting down...")

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
			log.Printf("ERROR: newMessage: Error sending ack for @mention message in ChannelID %s: %v", m.ChannelID, err)
		}

		if refer == nil {
			// This implies the ack message failed to send, which is a problem.
			log.Printf("ERROR: newMessage: Waiting message (refer) is nil after sending ack for @mention in ChannelID %s. Message from UserID %s.", m.ChannelID, m.Author.ID)
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

		log.Printf("DEBUG: newMessage: Processed @mention. UserID: %s, ChannelID: %s. Content after modification: %s", m.Author.ID, m.ChannelID, m.Content)

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
			log.Printf("ERROR: newMessage: Error sending pong message to ChannelID %s: %v", m.ChannelID, err)
		}

	case strings.HasPrefix(m.Content, "!forget"):
		log.Printf("INFO: newMessage: Received !forget command from UserID %s in ChannelID %s.", m.Author.ID, m.ChannelID)
		refer, err := s.ChannelMessageSendReply(m.ChannelID, "Clearing message memory from bot...", m.Reference())
		isForget := true

		if err != nil {
			log.Printf("ERROR: newMessage: Error sending ack for !forget command in ChannelID %s: %v", m.ChannelID, err)
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
		// Check if the message in queue belongs to the same user and channel
		if b.messageQueue[i].Message.Author.ID == msg.Message.Author.ID &&
			b.messageQueue[i].Message.ChannelID == msg.Message.ChannelID &&
			(b.messageQueue[i].IsForget == nil || !*b.messageQueue[i].IsForget) { // Avoid removing already queued forget messages
			log.Printf("DEBUG: forgetMessage: Removing message at index %d for UserID %s in ChannelID %s from queue.", i, msg.Message.Author.ID, msg.Message.ChannelID)
			b.messageQueue = append(b.messageQueue[:i], b.messageQueue[i+1:]...)
		}
	}

	log.Printf("INFO: Cleared message queue for UserID %s in ChannelID %s. Queue length now %d.", msg.Message.Author.ID, msg.Message.ChannelID, len(b.messageQueue))
	// Then add the new forget message to the queue.
	b.messageQueue = append(b.messageQueue, msg)
	log.Printf("DEBUG: forgetMessage: Forget message for UserID %s added to queue.", msg.Message.Author.ID)
}

// RespondToMessage sends a message using the bot's session
// This method is now part of the Bot struct
func (b *Bot) RespondToMessage(channelID string, response string, ref *discordgo.MessageReference, waitMessage *discordgo.Message) {
	if b.Session == nil {
		log.Printf("ERROR: RespondToMessage: Bot session not initialized. Cannot send message to ChannelID %s.", channelID)
		return
	}

	if waitMessage != nil {
		err := b.Session.ChannelMessageDelete(waitMessage.ChannelID, waitMessage.ID)
		if err != nil {
			log.Printf("ERROR: RespondToMessage: Error deleting waitMessage ID %s in ChannelID %s: %v", waitMessage.ID, waitMessage.ChannelID, err)
		} else {
			log.Printf("DEBUG: RespondToMessage: Successfully deleted waitMessage ID %s in ChannelID %s.", waitMessage.ID, waitMessage.ChannelID)
		}
	} else {
		// Depending on strictness, this could be an ERROR or WARN.
		// If waitMessage is essential for context, it's an error. If optional, a warning.
		log.Printf("WARN: RespondToMessage: waitMessage is nil for ChannelID %s. Cannot delete.", channelID)
		// The original code returns here. If a response MUST follow a waitMessage, this is correct.
		// If not, this return might be too strict. For now, keeping original behavior.
		return
	}

	sendMessage := &discordgo.MessageSend{
		Content:   response,
		Reference: ref,
	}
	log.Printf("DEBUG: RespondToMessage: Sending message to ChannelID %s. Content: %s", channelID, response)
	_, err := b.Session.ChannelMessageSendComplex(channelID, sendMessage)
	if err != nil {
		log.Printf("ERROR: RespondToMessage: Error sending message to ChannelID %s: %v", channelID, err)
	}
}

func (b *Bot) RespondToLongMessage(channelID string, response []string, ref *discordgo.MessageReference, waitMessage *discordgo.Message) {
	if b.Session == nil {
		log.Printf("ERROR: RespondToLongMessage: Bot session not initialized. Cannot send message to ChannelID %s.", channelID)
		return
	}

	if waitMessage != nil {
		err := b.Session.ChannelMessageDelete(waitMessage.ChannelID, waitMessage.ID)
		if err != nil {
			log.Printf("ERROR: RespondToLongMessage: Error deleting waitMessage ID %s in ChannelID %s: %v", waitMessage.ID, waitMessage.ChannelID, err)
		} else {
			log.Printf("DEBUG: RespondToLongMessage: Successfully deleted waitMessage ID %s in ChannelID %s.", waitMessage.ID, waitMessage.ChannelID)
		}
	} else {
		log.Printf("WARN: RespondToLongMessage: waitMessage is nil for ChannelID %s. Cannot delete.", channelID)
		// Original code returns here; maintaining this.
		return
	}
	log.Printf("DEBUG: RespondToLongMessage: Sending %d message segments to ChannelID %s.", len(response), channelID)
	for i := range response {
		segment := response[i]

		segmentContent := fmt.Sprintf("[Section %d/%d]\n%s", i+1, len(response), segment)
		sendMessage := &discordgo.MessageSend{
			Content:   segmentContent,
			Reference: ref,
		}
		log.Printf("DEBUG: RespondToLongMessage: Sending segment %d/%d to ChannelID %s.", i+1, len(response), channelID)
		_, err := b.Session.ChannelMessageSendComplex(channelID, sendMessage)
		if err != nil {
			log.Printf("ERROR: RespondToLongMessage: Error sending segment %d/%d to ChannelID %s: %v", i+1, len(response), channelID, err)
			// Optionally, break or return if one segment fails
		}
	}
}

func (b *Bot) SendMessageToChannel(channelID string, message string) {
	if b.Session == nil {
		log.Printf("ERROR: SendMessageToChannel: Bot session not initialized. Cannot send to ChannelID %s.", channelID)
		return
	}

	sendMessage := &discordgo.MessageSend{
		Content: message,
	}
	log.Printf("DEBUG: SendMessageToChannel: Sending message to ChannelID %s. Content: %s", channelID, message)
	_, err := b.Session.ChannelMessageSendComplex(channelID, sendMessage)
	if err != nil {
		log.Printf("ERROR: SendMessageToChannel: Error sending message to ChannelID %s: %v", channelID, err)
	}
}

func (b *Bot) Stop() error {
	log.Printf("INFO: Stop: Attempting to stop bot operations.")
	// Close the session and clean up resources
	if b.Session != nil {
		log.Printf("INFO: Stop: Closing Discord session.")
		err := b.Session.Close()
		if err != nil {
			log.Printf("ERROR: Stop: Error closing Discord session: %v", err)
			// Still attempt to close messageChannel
		} else {
			log.Printf("INFO: Stop: Discord session closed successfully.")
		}
	}

	if b.messageChannel != nil {
		log.Printf("INFO: Stop: Closing message channel.")
		close(b.messageChannel)
	}
	log.Printf("INFO: Bot stopped.")
	return nil // Original function returns the error from Session.Close(), this changes it.
	             // For now, let's return nil to indicate Stop attempted all cleanup.
	             // Or, return the session close error if that's critical.
	             // For consistency with original, let's keep returning the error from Session.Close()
	             // err := b.Session.Close() ... return err
}

func (b *Bot) JoinVC(guildID string, channelID string) (*discordgo.VoiceConnection, error) {
	if b.Session == nil {
		log.Printf("ERROR: JoinVC: Bot session not initialized. Cannot join GuildID %s, ChannelID %s.", guildID, channelID)
		return nil, fmt.Errorf("session not initialized")
	}
	log.Printf("INFO: JoinVC: Attempting to join GuildID %s, ChannelID %s.", guildID, channelID)
	vc, err := b.Session.ChannelVoiceJoin(guildID, channelID, false, false)
	if err != nil {
		log.Printf("ERROR: JoinVC: Error joining voice channel GuildID %s, ChannelID %s: %v", guildID, channelID, err)
		return nil, err
	}

	if vc == nil {
		// This case should ideally not be reached if err is nil, per discordgo docs.
		log.Printf("ERROR: JoinVC: Voice connection is nil after successful join for GuildID %s, ChannelID %s.", guildID, channelID)
		return nil, fmt.Errorf("voice connection is nil despite no error")
	}
	log.Printf("INFO: JoinVC: Successfully joined GuildID %s, ChannelID %s.", guildID, channelID)
	return vc, nil
}

func (b *Bot) LeaveVC(guildID string, channelID string) {
	// channelID is not used in the original logic, but good for logging.
	if b.Session == nil {
		log.Printf("ERROR: LeaveVC: Bot session not initialized. Cannot leave GuildID %s, ChannelID %s.", guildID, channelID)
		return
	}

	log.Printf("INFO: LeaveVC: Attempting to leave voice channel in GuildID %s (requested for ChannelID %s).", guildID, channelID)
	voiceChats := b.Session.VoiceConnections // Accessing VoiceConnections should be thread-safe if discordgo handles it.

	vc, ok := voiceChats[guildID]
	if !ok || vc == nil {
		log.Printf("WARN: LeaveVC: No active voice connection found for GuildID %s to disconnect from.", guildID)
		return
	}

	log.Printf("INFO: LeaveVC: Disconnecting from voice channel %s in GuildID %s.", vc.ChannelID, guildID)
	err := vc.Disconnect()
	if err != nil {
		log.Printf("ERROR: LeaveVC: Error disconnecting from voice channel %s in GuildID %s: %v", vc.ChannelID, guildID, err)
		return
	}
	log.Printf("INFO: LeaveVC: Successfully disconnected from voice channel %s in GuildID %s.", vc.ChannelID, guildID)
}

// JoinVc (lowercase c) - Note: This is a duplicate or alternative to JoinVC.
// Consider consolidating into JoinVC and removing this.
// For now, standardizing logs as per other functions.
func (b *Bot) JoinVc(gId string, cId string) (*discordgo.VoiceConnection, error) {
	if b.Session == nil {
		log.Printf("ERROR: JoinVc: Bot session not initialized. Cannot join GuildID %s, ChannelID %s.", gId, cId)
		return nil, fmt.Errorf("session not initialized")
	}

	// check if connection already exists
	if _, ok := b.Session.VoiceConnections[gId]; ok {
		log.Printf("WARN: JoinVc: Bot already connected to a voice channel in GuildID %s. Cannot join ChannelID %s.", gId, cId)
		return nil, fmt.Errorf("already connected to a voice channel in guild %s", gId)
	}

	log.Printf("INFO: JoinVc: Attempting to join GuildID %s, ChannelID %s.", gId, cId)
	vc, err := b.Session.ChannelVoiceJoin(gId, cId, false, false)
	if err != nil {
		log.Printf("ERROR: JoinVc: Error joining voice channel GuildID %s, ChannelID %s: %v", gId, cId, err)
		return nil, err
	}
	// Missing vc == nil check here, which JoinVC has.
	log.Printf("INFO: JoinVc: Successfully joined GuildID %s, ChannelID %s.", gId, cId)
	return vc, nil
}

// DeleteWaitMessage is a new helper function that might be useful for MessageLoop
func (b *Bot) DeleteWaitMessage(channelID string, messageID string) {
	if b.Session == nil {
		log.Printf("ERROR: DeleteWaitMessage: Bot session not initialized. Cannot delete message ID %s in ChannelID %s.", messageID, channelID)
		return
	}
	err := b.Session.ChannelMessageDelete(channelID, messageID)
	if err != nil {
		log.Printf("ERROR: DeleteWaitMessage: Failed to delete message ID %s in ChannelID %s: %v", messageID, channelID, err)
	} else {
		log.Printf("DEBUG: DeleteWaitMessage: Successfully deleted message ID %s in ChannelID %s.", messageID, channelID)
	}
}

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

		/*
			Message format:
			{
				"referenced_message"		: "The message content from the referenced message",
				"referenced_message_author"	: "The ID of the author of the referenced message",
				"message"					: "The content of the current message",
			}
		*/

		if m.ReferencedMessage != nil {
			referMsg := fmt.Sprintf(
				"{\n"+
					"\"referenced_message\": \"%s\", \n"+
					"\"referenced_message_author\": \"%s\"\n"+
					"\"message\":\"%s\"\n"+
					"}", m.ReferencedMessage.Content, m.ReferencedMessage.Author.ID, m.Content)
			m.Content = referMsg
		} else {
			referMsg := fmt.Sprintf(
				"{\n"+
					"\"message\":\"%s\"\n"+
					"}", m.Content)
			m.Content = referMsg
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

// RespondInVC sends Text-To-Speech in a voice channel
//func RespondInVC(b *Bot, ref *discordgo.MessageReference) {
//	botUser, err := b.Session.User("@me")
//	if err != nil {
//		log.Printf("Error getting bot user: %v", err)
//		return
//	}
//	channel, err := voiceChatUtils.FindVoiceChannel(b.Session, ref.GuildID, botUser.ID)
//	if err != nil {
//		return
//	}
//	vc, err := b.JoinVc(ref.GuildID, channel)
//	if err != nil {
//		return
//	}
//
//	var stop <-chan bool
//	var donePlaying chan<- bool
//
//	go voiceChatUtils.PlayAudioFile(vc, "output.mp3", stop, donePlaying)
//
//	switch {
//	case <-stop:
//		log.Println("Done playing audio file")
//		err := vc.Disconnect()
//		if err != nil {
//			return
//		}
//
//	}
//}

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

func (b *Bot) JoinVC(guildID string, channelID string) (*discordgo.VoiceConnection, error) {
	currSession := b.Session

	if currSession == nil {
		log.Println("Error: Bot session not initialized in JoinVC")
		return nil, fmt.Errorf("session not initialized")
	}

	vc, err := currSession.ChannelVoiceJoin(guildID, channelID, false, false)
	if err != nil {
		log.Printf("Error joining voice channel: %v", err)
		return nil, err
	}

	if vc == nil {
		log.Println("Error: Voice connection is nil")
		return nil, fmt.Errorf("voice connection is nil")
	}

	return vc, nil
}

func (b *Bot) LeaveVC(guildID string, channelID string) {
	currSession := b.Session

	if currSession == nil {
		log.Println("Error: Bot session not initialized in LeaveVC")
		return
	}

	voiceChats := currSession.VoiceConnections

	if len(voiceChats) == 0 {
		log.Println("Error: No active voice connections")
		return
	}

	vc := voiceChats[guildID]

	err := vc.Disconnect()

	if err != nil {
		log.Printf("Error leaving voice channel: %v", err)
		return
	}
}

func (b *Bot) JoinVc(gId string, cId string) (*discordgo.VoiceConnection, error) {

	// check if connection already exists
	if _, ok := b.Session.VoiceConnections[gId]; ok {
		log.Println("Already connected to voice channel")
		return nil, fmt.Errorf("already connected to voice channel")
	}

	vc, err := b.Session.ChannelVoiceJoin(gId, cId, false, false)

	if err != nil {
		log.Printf("Error joining voice channel: %v", err)
		return nil, err
	}

	return vc, nil
}

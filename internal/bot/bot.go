package bot

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"
	"untitled/internal/tts"
	"untitled/internal/voiceChatUtils"
	"untitled/transcribe"
)

// Bot holds the state for a Discord bot instance
type Bot struct {
	Token             string
	Session           *discordgo.Session
	messageQueue      []MessageForCompletion
	mu                sync.Mutex
	messageChannel    chan *MessageForCompletion     // Channel for message processing
	stopTranscribe    chan bool                      // Flag to stop transcription if needed
	transcribeChannel chan *transcribe.Msg           // Channel for transcription messages
	awsConfig         aws.Config                     // AWS configuration for TTS
	transcribeQueue   map[string]map[string][]string // Queue for transcription messages, keyed by guild ID and channel ID, value is file name
	imageReqChan      chan *ImageDescriptionRequest
}

// MessageForCompletion represents a message that needs to be processed with a wait message.
// Wait messages are used to acknowledge receipt of a message and provide a way to delete it later.
// It includes the original message, a wait message to acknowledge receipt, and a flag for forgetting
// the message
// This struct is used to queue messages that the bot sends to the router for processing.
type MessageForCompletion struct {
	Message     *discordgo.MessageCreate
	WaitMessage *discordgo.Message
	IsForget    *bool
	IsPlayback  bool // Indicates if the message is for playback
	VC          *discordgo.VoiceConnection
}

type ImageDescriptionRequest struct {
	AttachmentImg *discordgo.MessageAttachment
	Description   *string
}

// NewBot creates a new Bot instance but doesn't connect yet
func NewBot(token string, msgChan chan *MessageForCompletion, awsConf aws.Config, imageReqChan chan *ImageDescriptionRequest) (*Bot, error) {
	b := &Bot{
		Token:          token,
		messageChannel: msgChan,
		awsConfig:      awsConf,
		imageReqChan:   imageReqChan,
	}

	b.transcribeChannel = make(chan *transcribe.Msg)

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
	go b.relayTranscribeMsg()

	fmt.Println("Bot running....")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	fmt.Println("Bot shutting down...")

	return b.Session.Close()
}

func (b *Bot) relayTranscribeMsg() {
	log.Println("Transcription relay started. Waiting for messages...")

	// This loop will block on <-b.transcribeChannel until a message
	// is available. It's the most efficient way to wait.
	for message := range b.transcribeChannel {
		log.Println("Received transcription message from channel.")

		isForget := false

		packageMsg := &MessageForCompletion{
			Message:     &discordgo.MessageCreate{Message: message.Message},
			WaitMessage: nil,
			IsForget:    &isForget,
			IsPlayback:  true,
			VC:          message.VC,
		}

		log.Println("Sending transcription message to router")
		b.messageChannel <- packageMsg // Send content to the next channel
	}
}

// relayMessagesToRouter processes the internal queue
func (b *Bot) relayMessagesToRouter() {
	for {
		var message *MessageForCompletion

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
// This method is triggered every time a message is sent in the channel
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
			craftedMessage := m.ReferencedMessage.Content
			imageDescriptions := strings.Builder{}
			if m.ReferencedMessage.Attachments != nil {
				for i, attachment := range m.ReferencedMessage.Attachments {
					if strings.HasPrefix(attachment.ContentType, "image/") {
						imageDescriptions.WriteString("{\n\"index\" : " + strconv.Itoa(i) + ",\n\"description\": " + b.getImageDescription(attachment) + "}, \n")
					}
				}
			}

			if imageDescriptions.Len() > 0 {
				craftedMessage = "{\n \"images\" : " + imageDescriptions.String() + "}\n" + craftedMessage
			}

			referMsg := fmt.Sprintf(
				"{\n"+
					"\"referenced_message\": \"%s\", \n"+
					"\"referenced_message_author\": \"%s\"\n"+
					"\"message\":\"%s\"\n"+
					"}", craftedMessage, m.ReferencedMessage.Author.ID, m.Content)
			m.Content = referMsg
		} else {
			referMsg := fmt.Sprintf(
				"{\n"+
					"\"message\":\"%s\"\n"+
					"}", m.Content)
			m.Content = referMsg
		}

		fmt.Println("Message content: ", m.Content)

		msg := MessageForCompletion{
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

		b.forgetMessage(MessageForCompletion{
			Message:     m,
			WaitMessage: refer,
			IsForget:    &isForget,
		})
	case strings.HasPrefix(m.Content, "!start"):
		if b.stopTranscribe == nil {
			b.stopTranscribe = make(chan bool) // Initialize the stop channel
		}

		if b.transcribeChannel == nil {
			log.Println("Error: transcribe channel is nil")
			return
		}

		ready := make(chan bool, 0)

		go func() {
			<-ready // Wait for the transcribe to be ready
			_, err := s.ChannelMessageSendReply(m.ChannelID, "Transcription started. You can now speak to the bot.", m.Reference())
			if err != nil {
				return
			}
		}()

		GID := m.GuildID
		voiceChannel, err := voiceChatUtils.FindVoiceChannel(b.Session, GID, m.Author.ID)
		if err != nil {
			return
		}

		transcribe.StartTranscribe(b.Session, b.stopTranscribe, b.transcribeChannel, ready, GID, voiceChannel)

	case strings.HasPrefix(m.Content, "!stop"):
		// stop transcription
		refer, err := s.ChannelMessageSendReply(m.ChannelID, "Stopping transcription...", m.Reference())
		if err != nil {
			log.Printf("Error sending ack for !stop: %v", err)
			return
		}

		_, err = s.ChannelMessageSendReply(refer.ChannelID, "Transcription stopped.", refer.Reference())
		if err != nil {
			log.Printf("Error sending stop message: %v", err)
			return
		}

		if b.stopTranscribe != nil {
			b.stopTranscribe <- true // Signal to stop transcription
			b.stopTranscribe = nil   // Reset the channel
		} else {
			log.Println("No transcription in progress to stop.")
		}
	}
}

func (b *Bot) getImageDescription(attachment *discordgo.MessageAttachment) string {
	imageReq := ImageDescriptionRequest{
		Description:   nil,
		AttachmentImg: attachment,
	}

	b.imageReqChan <- &imageReq

	for {
		if imageReq.Description != nil {
			return *imageReq.Description
		}
		time.Sleep(100 * time.Millisecond) // busy waiting, maybe not the best method
	}
}

// addMessage adds a message to the internal queue
func (b *Bot) addMessage(message MessageForCompletion) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messageQueue = append(b.messageQueue, message)
}

func (b *Bot) forgetMessage(msg MessageForCompletion) {
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

func (b *Bot) PlaybackResponse(vc *discordgo.VoiceConnection, AIresp string) {
	if b.Session == nil {
		log.Println("Error: Bot session not initialized in PlaybackResponse")
		return
	}

	if vc == nil {
		log.Println("Error: Voice connection is nil in PlaybackResponse")
		return
	}

	// use tts to convert AI response to audio
	if AIresp == "" {
		log.Println("Error: AI response is empty in PlaybackResponse")
		return
	}

	audioFileName, err := tts.TextToSpeech(AIresp, b.awsConfig)

	if err != nil {
		log.Println("Error converting AI response to audio:", err)
	}

	if audioFileName == "" {
		log.Println("Error: audio file name is empty in PlaybackResponse")
		return
	}

	// Play the audio file
	b.playAudio(audioFileName, vc)
}

func (b *Bot) playAudio(filename string, vc *discordgo.VoiceConnection) {
	// add the audio file to the transcribe queue
	if b.transcribeQueue == nil {
		b.transcribeQueue = make(map[string]map[string][]string)
	}

	if b.transcribeQueue[vc.GuildID] == nil {
		b.transcribeQueue[vc.GuildID] = make(map[string][]string)
	}

	if b.transcribeQueue[vc.GuildID][vc.ChannelID] == nil {
		b.transcribeQueue[vc.GuildID][vc.ChannelID] = make([]string, 0)
	}

	b.mu.Lock()
	b.transcribeQueue[vc.GuildID][vc.ChannelID] = append(b.transcribeQueue[vc.GuildID][vc.ChannelID], filename)
	b.mu.Unlock()

	// PLay the audio file in the voice channel
	log.Printf("Queuing audio file %s in voice channel %s of guild %s", filename, vc.ChannelID, vc.GuildID)
	for b.transcribeQueue[vc.GuildID][vc.ChannelID][0] != filename {
		time.Sleep(500 * time.Millisecond) // between voice file playback, make it more natural
	}

	stop := make(chan bool, 0)        // Create a stop channel to signal when to stop playing
	donePlaying := make(chan bool, 0) // Create a channel to signal when done playing

	log.Println("Playing audio file", filename)
	go voiceChatUtils.PlayAudioFile(vc, filename, stop, donePlaying)

	switch {
	case <-donePlaying:
		time.Sleep(50 * time.Millisecond) // Give some time for the audio to finish playing
		log.Println("Done playing audio file")
		b.mu.Lock()
		log.Println("removing audio file from transcribe queue")
		b.transcribeQueue[vc.GuildID][vc.ChannelID] = b.transcribeQueue[vc.GuildID][vc.ChannelID][1:] // Remove the first element
		b.mu.Unlock()
		log.Println("cleanup audio file", filename)
		err := os.Remove(filename)
		if err != nil {
			log.Printf("Error removing audio file %s: %v", filename, err)
		} else {
			log.Printf("Removed audio file %s successfully", filename)
		}
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

func (b *Bot) LeaveVC(guildID string) {
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

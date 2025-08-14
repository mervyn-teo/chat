package transcribe

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"layeh.com/gopus"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"untitled/internal/voiceChatUtils"
)

const (
	CHANNELS            = 2       // stereo
	FRAME_RATE          = 48000   // audio sampling rate
	MAGIC_KEYWORD_START = "hello" // Example keyword to trigger transcription
	MAGIC_KEYWORD_END   = "bye"   // Example keyword to stop transcription
	SPECIAL_USER_ID     = "10086" // This is a special user ID, representing a generic user in the voice channel
	TIME_INTERVAL       = 3       // Interval for processing audio
)

// Global speakers map for opus decoders (following discordgo example)
var speakers map[uint32]*gopus.Decoder

type Msg struct {
	Message *discordgo.Message         // The message to transcribe
	VC      *discordgo.VoiceConnection // The voice connection to use for transcription
}

type VoiceTranscriber struct {
	session               *discordgo.Session
	guildID               string
	channelID             string
	voiceConn             *discordgo.VoiceConnection
	ctx                   context.Context
	cancel                context.CancelFunc
	audioBuffer           []int16
	bufferMutex           sync.Mutex
	whisperPath           string
	modelPath             string
	pcmChan               chan *discordgo.Packet
	msgForRouter          string
	isRecording           bool // Flag to track if transcription is active
	msgChan               chan *Msg
	keepAlive             chan bool
	stopCurrTranscription chan bool // Channel to stop current transcription
}

func NewVoiceTranscriber(session *discordgo.Session, guildID, channelID string, transcibeChan chan *Msg) *VoiceTranscriber {
	ctx, cancel := context.WithCancel(context.Background())
	return &VoiceTranscriber{
		guildID:               guildID,
		channelID:             channelID,
		ctx:                   ctx,
		cancel:                cancel,
		audioBuffer:           make([]int16, 0),
		pcmChan:               make(chan *discordgo.Packet, 100),
		session:               session,
		whisperPath:           "./whisper.cpp/build/bin/whisper-cli",
		modelPath:             "./whisper.cpp/models/ggml-base.en.bin",
		isRecording:           false, // Initially not recording
		msgChan:               transcibeChan,
		keepAlive:             make(chan bool), // Keep the transcriber alive
		stopCurrTranscription: make(chan bool), // Channel to stop current transcription
	}
}

// OnError handles errors (following discordgo example pattern)
func OnError(msg string, err error) {
	if err != nil {
		log.Printf("ERROR: %s: %v", msg, err)
	} else {
		log.Printf("ERROR: %s", msg)
	}
}

// ReceivePCM receives and decodes opus packets to PCM (from discordgo example)
func (vt *VoiceTranscriber) ReceivePCM(v *discordgo.VoiceConnection, c chan *discordgo.Packet) {
	if c == nil {
		return
	}

	var err error

	for {
		select {
		case <-vt.ctx.Done():
			return
		default:
		}

		if v.Ready == false || v.OpusRecv == nil {
			OnError(fmt.Sprintf("Discordgo not ready to receive opus packets. Ready: %v, OpusRecv: %v", v.Ready, v.OpusRecv != nil), nil)
			return
		}

		p, ok := <-v.OpusRecv
		if !ok {
			return
		}

		if speakers == nil {
			speakers = make(map[uint32]*gopus.Decoder)
		}

		_, ok = speakers[p.SSRC]
		if !ok {
			speakers[p.SSRC], err = gopus.NewDecoder(48000, 2)
			if err != nil {
				OnError("error creating opus decoder", err)
				continue
			}
		}

		p.PCM, err = speakers[p.SSRC].Decode(p.Opus, 960, false)
		if err != nil {
			OnError("Error decoding opus data", err)
			continue
		}

		// Only send packets with actual audio data
		if len(p.PCM) > 0 {
			select {
			case c <- p:
			case <-vt.ctx.Done():
				return
			default:
				// Channel full, skip this packet to prevent blocking
			}
		}
	}
}

// Connect connects to the voice channel and starts processing audio
func (vt *VoiceTranscriber) Connect() error {
	dg := vt.session

	// Join voice channel
	vc, err := dg.ChannelVoiceJoin(vt.guildID, vt.channelID, false, false)
	if err != nil {
		return fmt.Errorf("error joining voice channel: %v", err)
	}
	vt.voiceConn = vc

	fmt.Println("Voice connected, waiting for ready state...")

	// Wait for voice connection to be ready
	for !vc.Ready {
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("Voice connection ready, starting audio processing...")

	// Start the PCM receiver goroutine (following discordgo example)
	go vt.ReceivePCM(vc, vt.pcmChan)

	// Start audio processing
	go vt.processAudioPeriodically()

	// Start listening to PCM packets
	vt.startListening()

	log.Println("Bot connected and listening...")
	return nil
}

func (vt *VoiceTranscriber) processAudioPeriodically() {
	ticker := time.NewTicker(200 * time.Millisecond) // Process every half a second to improve responsiveness
	defer ticker.Stop()

	for {
		if vt.isRecording {
			ticker.Reset(TIME_INTERVAL * time.Second) // if recording Process every 5 seconds
		} else {
			ticker.Reset(500 * time.Millisecond) // if not recording Process every half a second
		}

		select {
		case <-vt.ctx.Done():
			return
		case <-ticker.C:
			vt.bufferMutex.Lock()
			if len(vt.audioBuffer) > FRAME_RATE { // At least 1 second of audio
				// Copy buffer to avoid holding the lock too long
				bufferCopy := make([]int16, len(vt.audioBuffer))
				copy(bufferCopy, vt.audioBuffer)
				vt.audioBuffer = vt.audioBuffer[:0] // Clear buffer
				vt.bufferMutex.Unlock()

				go vt.transcribeAudioBuffer(bufferCopy)
			} else {
				vt.bufferMutex.Unlock()
			}
		}
	}
}

func (vt *VoiceTranscriber) transcribeAudioBuffer(audioData []int16) {
	if len(audioData) == 0 {
		return
	}

	// Create temporary WAV file
	tempFile := fmt.Sprintf("/tmp/discord_audio_%d.wav", time.Now().UnixNano())
	defer func() {
		if err := os.Remove(tempFile); err != nil {
			log.Printf("Warning: could not remove temp file %s: %v", tempFile, err)
		}
	}()

	err := vt.saveAsWAV(tempFile, audioData)
	if err != nil {
		log.Printf("Error saving WAV file: %v", err)
		return
	}

	// Check if files exist
	if _, err := os.Stat(vt.whisperPath); os.IsNotExist(err) {
		log.Printf("Whisper executable not found at: %s", vt.whisperPath)
		return
	}
	if _, err := os.Stat(vt.modelPath); os.IsNotExist(err) {
		log.Printf("Whisper model not found at: %s", vt.modelPath)
		return
	}

	// Run whisper.cpp on the audio file
	cmd := exec.CommandContext(vt.ctx,
		vt.whisperPath,
		"-m", vt.modelPath,
		"-f", tempFile,
		"--no-timestamps",
		"--output-txt",
		"--language", "auto",
	)

	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error running Whisper: %v", err)
		if exitError, ok := err.(*exec.ExitError); ok {
			log.Printf("Whisper stderr: %s", string(exitError.Stderr))
		}
		return
	}

	text := strings.TrimSpace(string(output))
	// Filter out common noise/silence indicators
	if text != "" &&
		text != "[BLANK_AUDIO]" && // Common whisper hallucination
		len(text) > 3 {
		log.Printf("Transcription: %s", text)
		vt.handleTranscription(text)
	}
}

func (vt *VoiceTranscriber) saveAsWAV(filename string, audioData []int16) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	// WAV header calculations
	dataSize := uint32(len(audioData) * 2)
	fileSize := uint32(36 + dataSize)
	sampleRate := uint32(FRAME_RATE)
	byteRate := uint32(FRAME_RATE * CHANNELS * 2)
	blockAlign := uint16(CHANNELS * 2)
	bitsPerSample := uint16(16)
	channels := uint16(CHANNELS)

	// Write RIFF header
	if _, err := file.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, fileSize); err != nil {
		return err
	}
	if _, err := file.Write([]byte("WAVE")); err != nil {
		return err
	}

	// Write fmt chunk
	if _, err := file.Write([]byte("fmt ")); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(16)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, channels); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, sampleRate); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, byteRate); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, blockAlign); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, bitsPerSample); err != nil {
		return err
	}

	// Write data chunk
	if _, err := file.Write([]byte("data")); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, dataSize); err != nil {
		return err
	}

	// Write audio data
	return binary.Write(file, binary.LittleEndian, audioData)
}

func (vt *VoiceTranscriber) startListening() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic in audio listener: %v", r)
			}
		}()

		for {
			select {
			case <-vt.ctx.Done():
				return
			case packet, ok := <-vt.pcmChan:
				if !ok {
					log.Println("PCM channel closed")
					return
				}
				if packet != nil && packet.PCM != nil && len(packet.PCM) > 0 {
					vt.bufferMutex.Lock()
					vt.audioBuffer = append(vt.audioBuffer, packet.PCM...)

					// Prevent buffer from growing too large
					maxBufferSize := FRAME_RATE * CHANNELS * 30 // 30 seconds max
					if len(vt.audioBuffer) > maxBufferSize {
						// Keep only the last 20 seconds
						keepSize := FRAME_RATE * CHANNELS * 20
						copy(vt.audioBuffer, vt.audioBuffer[len(vt.audioBuffer)-keepSize:])
						vt.audioBuffer = vt.audioBuffer[:keepSize]
					}
					vt.bufferMutex.Unlock()
				}
			}
		}
	}()
}

// handleTranscription processes the transcription result
// if there is a keyword in the transcription, it will start recording the transcription for later use
// ignoring keyword mentioned in the middle
// if both keywords are mentioned, it will not record the rest
func (vt *VoiceTranscriber) handleTranscription(text string) {
	if text == "" {
		log.Println("Received empty transcription, ignoring.")
		return
	}

	log.Println("Transcription received:", text)
	// both keywords are mentioned
	if strings.Contains(strings.ToLower(text), MAGIC_KEYWORD_START) && strings.Contains(strings.ToLower(text), MAGIC_KEYWORD_END) && !vt.isRecording {
		log.Println("Both start and end keywords found.")
		vt.msgForRouter = strings.Split(strings.ToLower(text), MAGIC_KEYWORD_START)[1] // Get text after start keyword
		vt.msgForRouter = strings.Split(vt.msgForRouter, MAGIC_KEYWORD_END)[0]         // Get text before end keyword

		if vt.msgForRouter != "" {
			log.Printf("Sending recorded transcription to router: %s", vt.msgForRouter)

			// Create a new Discord message with the transcription
			vt.msgForRouter = strings.TrimSpace(vt.msgForRouter) // Clean up any leading/trailing spaces
			if vt.msgForRouter == "" {
				log.Println("No transcription recorded, skipping message send.")
				return
			}
			vt.msgForRouter = strings.ReplaceAll(vt.msgForRouter, "\n", " ") // Replace newlines with spaces
			vt.msgForRouter = strings.TrimSpace(vt.msgForRouter)             // Clean up any leading/trailing spaces

			// Create a new Discord message
			specialUser := discordgo.User{
				ID:       SPECIAL_USER_ID,
				Username: "SpecialGenericVoiceChatUser",
			}

			message := &discordgo.Message{
				Content:   vt.msgForRouter,
				ChannelID: vt.channelID, // Use the voice channel ID for sending messages
				Author:    &specialUser,
			}

			// Create a transcribe message to send to the router
			transcribeMessage := &Msg{
				Message: message,
				VC:      vt.voiceConn,
			}

			log.Printf("Transcription message created: %s", vt.msgForRouter)
			// Send the message to the router via the channel
			vt.msgChan <- transcribeMessage
			vt.msgForRouter = "" // Clear the message after sending
		}
	} else if strings.Contains(strings.ToLower(text), MAGIC_KEYWORD_START) {
		if !vt.isRecording {
			// make a go routine that counts 5 seconds and if no text is received, it will stop recording
			go keepAlive(vt, vt.stopCurrTranscription)
			go func() {
				// Wait for the stop signal to stop recording
				<-vt.stopCurrTranscription
				log.Println("Stop recording transcription due to timeout.")
				if vt.isRecording {
					vt.isRecording = false
					// play the jingle
					tempStopChan := make(chan bool)
					tempDoneChan := make(chan bool)
					go voiceChatUtils.PlayAudioFile(vt.voiceConn, "sfx/end_confirm.wav", tempStopChan, tempDoneChan)
					go func() {
						// Wait for the jingle to finish playing
						<-tempDoneChan
						log.Println("Jingle finished playing, sending transcription to router.")
						close(tempStopChan)
						close(tempDoneChan)
					}()

					if vt.msgForRouter != "" {
						log.Printf("Sending recorded transcription to router: %s", vt.msgForRouter)

						// Create a new Discord message with the transcription
						vt.msgForRouter = strings.TrimSpace(vt.msgForRouter) // Clean up any leading/trailing spaces
						if vt.msgForRouter == "" {
							log.Println("No transcription recorded, skipping message send.")
							return
						}
						vt.msgForRouter = strings.ReplaceAll(vt.msgForRouter, "\n", " ") // Replace newlines with spaces
						vt.msgForRouter = strings.TrimSpace(vt.msgForRouter)             // Clean up any leading/trailing spaces

						// Create a new Discord message
						specialUser := discordgo.User{
							ID:       SPECIAL_USER_ID,
							Username: "SpecialGenericVoiceChatUser",
						}

						message := &discordgo.Message{
							Content:   vt.msgForRouter,
							ChannelID: vt.channelID, // Use the voice channel ID for sending messages
						}
						message.Author = &specialUser

						// Create a transcribe message to send to the router
						transcribeMessage := &Msg{
							Message: message,
							VC:      vt.voiceConn,
						}

						log.Printf("Transcription message created: %s", vt.msgForRouter)
						// Send the message to the router via the channel
						vt.msgChan <- transcribeMessage
						vt.msgForRouter = "" // Clear the message after sending
					}
				}
			}()

			// Start recording transcription
			log.Println("Start recording transcription...")
			vt.isRecording = true
			// play the jingle
			tempStopChan := make(chan bool)
			tempDoneChan := make(chan bool)
			go voiceChatUtils.PlayAudioFile(vt.voiceConn, "sfx/comfirmation.mp3", tempStopChan, tempDoneChan)
			go func() {
				// Wait for the jingle to finish playing
				<-tempDoneChan
				log.Println("Jingle finished playing, sending transcription to router.")
				close(tempStopChan)
				close(tempDoneChan)
			}()
			vt.msgForRouter = strings.Split(strings.ToLower(text), MAGIC_KEYWORD_START)[1] // Get text after start keyword
		}
	} else if strings.Contains(strings.ToLower(text), MAGIC_KEYWORD_END) {
		vt.msgForRouter = vt.msgForRouter + " " + strings.Split(strings.ToLower(text), MAGIC_KEYWORD_END)[0] // Get text before end keyword
		if vt.isRecording {
			log.Println("Stop recording transcription...")
			// Stop recording transcription
			vt.isRecording = false
			// Send the recorded transcription to the router

			// play the jingle
			tempStopChan := make(chan bool)
			tempDoneChan := make(chan bool)
			go voiceChatUtils.PlayAudioFile(vt.voiceConn, "sfx/end_confirm.wav", tempStopChan, tempDoneChan)
			go func() {
				// Wait for the jingle to finish playing
				<-tempDoneChan
				log.Println("Jingle finished playing, sending transcription to router.")
				close(tempStopChan)
				close(tempDoneChan)
			}()

			if vt.msgForRouter != "" {
				log.Printf("Sending recorded transcription to router: %s", vt.msgForRouter)

				// Create a new Discord message with the transcription
				vt.msgForRouter = strings.TrimSpace(vt.msgForRouter) // Clean up any leading/trailing spaces
				if vt.msgForRouter == "" {
					log.Println("No transcription recorded, skipping message send.")
					return
				}
				vt.msgForRouter = strings.ReplaceAll(vt.msgForRouter, "\n", " ") // Replace newlines with spaces
				vt.msgForRouter = strings.TrimSpace(vt.msgForRouter)             // Clean up any leading/trailing spaces

				// Create a new Discord message
				specialUser := discordgo.User{
					ID:       SPECIAL_USER_ID,
					Username: "SpecialGenericVoiceChatUser",
				}

				message := &discordgo.Message{
					Content:   vt.msgForRouter,
					ChannelID: vt.channelID, // Use the voice channel ID for sending messages
					Author:    &specialUser,
				}

				// Create a transcribe message to send to the router
				transcribeMessage := &Msg{
					Message: message,
					VC:      vt.voiceConn,
				}

				log.Printf("Transcription message created")
				// Send the message to the router via the channel
				vt.msgChan <- transcribeMessage
				vt.msgForRouter = "" // Clear the message after sending
			} else {
				log.Println("No transcription recorded.")
			}
		}
	}

	// If recording is active, append the text to the current transcription
	if vt.isRecording {
		vt.keepAlive <- true // Reset the keep alive timer
		if vt.msgForRouter != "" {
			vt.msgForRouter += " " + text // Append new text to the current transcription
		} else {
			vt.msgForRouter = text // Start a new transcription
		}
		log.Printf("Current transcription: %s", vt.msgForRouter)
	}
}

// Close cleans up resources and stops the transcriber
func keepAlive(vt *VoiceTranscriber, stop chan bool) {
	seconds := 0
	for {
		if seconds >= TIME_INTERVAL*2 { // If no keep alive signal for 6 seconds, stop recording
			stop <- true
			return
		}

		select {
		case <-vt.ctx.Done():
			log.Println("Keep alive context done, stopping...")
			stop <- true
			return

		case <-vt.keepAlive:
			// Reset the seconds counter if we receive a keep alive signal
			seconds = 0
		default:
			// Sleep and Increment the seconds counter
			time.Sleep(1 * time.Second)
			seconds++
			log.Printf("Keep alive: %d seconds", seconds)
		}
	}
}

func (vt *VoiceTranscriber) Close() {
	log.Println("Shutting down...")

	// Cancel context to stop goroutines
	vt.cancel()

	// Give goroutines time to clean up
	time.Sleep(200 * time.Millisecond)

	// Close PCM channel
	close(vt.pcmChan)

	// Disconnect from voice channel
	if vt.voiceConn != nil {
		err := vt.voiceConn.Disconnect()
		if err != nil {
			return
		}
	}

	// Clean up speakers map
	if speakers != nil {
		for ssrc := range speakers {
			delete(speakers, ssrc)
		}
		speakers = nil
	}

	log.Println("Cleanup completed")
}

// StartTranscribe initializes the bot and starts the transcriber
// b is the bot instance, stop is a channel to signal shutdown
// ready is a channel to signal that the bot is ready
func StartTranscribe(session *discordgo.Session, stop chan bool, messageChannel chan *Msg, ready chan bool, GID string, voiceChannel string) {
	// Get configuration from environment variables for security
	// Use environment variables for sensitive data
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	guildID := GID
	channelID := voiceChannel

	if messageChannel == nil {
		log.Fatal("Message channel cannot be nil")
	}

	// Create transcriber
	transcriber := NewVoiceTranscriber(session, guildID, channelID, messageChannel)

	// Connect and start transcribing
	err = transcriber.Connect()
	if err != nil {
		log.Fatalf("Error connecting: %v", err)
	}

	log.Println("Bot is running. Press Ctrl+C to stop.")
	ready <- true
	<-stop

	// Clean shutdown
	transcriber.Close()
	log.Println("Bot stopped.")
}

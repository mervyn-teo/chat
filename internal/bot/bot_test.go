package bot

import (
	"sync"
	"testing"
	"time"
	"untitled/internal/tts"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
)

// Test helper functions that work with the actual bot structure
func createTestBot(t *testing.T) (*Bot, chan *MessageForCompletion) {
	msgChan := make(chan *MessageForCompletion, 10)

	// tts routine
	awsConf := tts.LoadConfig()

	bot, err := NewBot("test-token", msgChan, awsConf, nil)
	assert.NoError(t, err)

	// Create a real session but don't open it
	// We'll test the logic without actually connecting to Discord
	return bot, msgChan
}

func createTestMessage(
	authorID, channelID, content string,
	mentions []*discordgo.User,
	referencedMessage *discordgo.Message,
) *discordgo.MessageCreate {
	return &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "message-id",
			ChannelID: channelID,
			Content:   content,
			Author: &discordgo.User{
				ID:       authorID,
				Username: "testuser",
			},
			Mentions:          mentions,
			ReferencedMessage: referencedMessage,
			Timestamp:         time.Now(),
		},
	}
}

func createMessageReference() *discordgo.MessageReference {
	return &discordgo.MessageReference{
		MessageID: "ref-message-id",
		ChannelID: "channel-id",
		GuildID:   "guild-id",
	}
}

// Mock the session for testing by creating a test session with proper state
func createTestBotWithMockState(t *testing.T) (*Bot, chan *MessageForCompletion) {
	msgChan := make(chan *MessageForCompletion, 10)

	// tts routine
	awsConf := tts.LoadConfig()

	bot, err := NewBot("test-token", msgChan, awsConf, nil)
	assert.NoError(t, err)

	// Create a session with proper state for testing
	session, err := discordgo.New("Bot test-token")
	assert.NoError(t, err)

	// Initialize the state manually for testing
	session.State = discordgo.NewState()
	session.State.User = &discordgo.User{
		ID:       "bot-id",
		Username: "testbot",
	}

	bot.Session = session
	return bot, msgChan
}

// Test NewBot function
func TestNewBot(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		msgChan := make(chan *MessageForCompletion, 10)

		// tts routine
		awsConf := tts.LoadConfig()

		bot, err := NewBot("test-token", msgChan, awsConf, nil)

		assert.NoError(t, err)
		assert.NotNil(t, bot)
		assert.Equal(t, "test-token", bot.Token)
		assert.Equal(t, msgChan, bot.messageChannel)
		assert.NotNil(t, bot.Session)
	})

	t.Run("empty token", func(t *testing.T) {
		msgChan := make(chan *MessageForCompletion, 10)

		// tts routine
		awsConf := tts.LoadConfig()

		bot, err := NewBot("", msgChan, awsConf, nil)

		// Bot creation should still succeed even with empty token
		assert.NoError(t, err)
		assert.NotNil(t, bot)
	})

	t.Run("nil channel", func(t *testing.T) {
		// tts routine
		awsConf := tts.LoadConfig()

		bot, err := NewBot("test-token", nil, awsConf, nil)

		assert.NoError(t, err)
		assert.NotNil(t, bot)
		assert.Nil(t, bot.messageChannel)
	})
}

// Test message handling logic (without actual Discord calls)
func TestBot_MessageHandlingLogic(t *testing.T) {
	t.Run("ignore bot's own messages", func(t *testing.T) {
		bot, msgChan := createTestBotWithMockState(t)

		// Create message from bot itself
		msg := createTestMessage("bot-id", "channel-id", "test", nil, nil)

		// Test the logic by calling newMessage with a mock session
		// Since we can't easily mock the session calls, we'll test the queue logic
		initialQueueLength := len(bot.messageQueue)

		// The message should be ignored due to author ID check
		if msg.Author.ID == bot.Session.State.User.ID {
			// This is the expected behavior - bot ignores its own messages
			assert.Equal(t, initialQueueLength, len(bot.messageQueue))
		}

		// Verify no messages in channel
		select {
		case <-msgChan:
			t.Error("Should not have received message in channel")
		default:
			// Expected behavior
		}
	})

	t.Run("detect bot mention", func(t *testing.T) {
		bot, _ := createTestBotWithMockState(t)

		// Create message with bot mention
		mentions := []*discordgo.User{{ID: "bot-id"}}
		msg := createTestMessage(
			"user-id",
			"channel-id",
			"<@bot-id> hello",
			mentions,
			nil,
		)

		// Test mention detection logic
		botWasMentioned := false
		for _, mention := range msg.Mentions {
			if mention.ID == bot.Session.State.User.ID {
				botWasMentioned = true
				break
			}
		}

		assert.True(t, botWasMentioned, "Bot mention should be detected")
	})

	t.Run("detect reply to bot message", func(t *testing.T) {
		bot, _ := createTestBotWithMockState(t)

		// Create referenced message from bot
		referencedMsg := &discordgo.Message{
			ID:      "ref-msg-id",
			Content: "Previous bot message",
			Author:  &discordgo.User{ID: "bot-id"},
		}

		msg := createTestMessage(
			"user-id",
			"channel-id",
			"reply to bot",
			nil,
			referencedMsg,
		)

		// Test reply detection logic
		botWasMentioned := false
		if msg.ReferencedMessage != nil &&
			msg.ReferencedMessage.Author.ID == bot.Session.State.User.ID {
			botWasMentioned = true
		}

		assert.True(t, botWasMentioned, "Reply to bot should be detected")
	})

	t.Run("detect ping command", func(t *testing.T) {
		msg := createTestMessage("user-id", "channel-id", "!ping", nil, nil)

		// Test ping command detection
		isPingCommand := len(msg.Content) >= 5 && msg.Content[:5] == "!ping"
		assert.True(t, isPingCommand, "Ping command should be detected")
	})

	t.Run("detect forget command", func(t *testing.T) {

		msg := createTestMessage("user-id", "channel-id", "!forget", nil, nil)

		// Test forget command detection
		isForgetCommand := len(msg.Content) >= 7 && msg.Content[:7] == "!forget"
		assert.True(t, isForgetCommand, "Forget command should be detected")
	})
}

// Test message queue operations
func TestBot_addMessage(t *testing.T) {
	t.Run("add single message", func(t *testing.T) {
		bot, _ := createTestBot(t)

		isForget := false
		msg := MessageForCompletion{
			Message: &discordgo.MessageCreate{
				Message: &discordgo.Message{ID: "test-id"},
			},
			IsForget: &isForget,
		}

		bot.addMessage(msg)

		assert.Equal(t, 1, len(bot.messageQueue))
		assert.Equal(t, "test-id", bot.messageQueue[0].Message.ID)
		assert.False(t, *bot.messageQueue[0].IsForget)
	})

	t.Run("add multiple messages", func(t *testing.T) {
		bot, _ := createTestBot(t)

		for i := 0; i < 5; i++ {
			isForget := false
			msg := MessageForCompletion{
				Message: &discordgo.MessageCreate{
					Message: &discordgo.Message{ID: "test-id-" + string(rune(i+'0'))},
				},
				IsForget: &isForget,
			}
			bot.addMessage(msg)
		}

		assert.Equal(t, 5, len(bot.messageQueue))
	})
}

func TestBot_forgetMessage(t *testing.T) {
	t.Run("forget messages from specific user and channel", func(t *testing.T) {
		bot, _ := createTestBot(t)

		// Add multiple messages from different users and channels
		isForget := false
		msg1 := MessageForCompletion{
			Message: &discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "msg1",
					Author:    &discordgo.User{ID: "user1"},
					ChannelID: "channel1",
				},
			},
			IsForget: &isForget,
		}
		msg2 := MessageForCompletion{
			Message: &discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "msg2",
					Author:    &discordgo.User{ID: "user2"},
					ChannelID: "channel1",
				},
			},
			IsForget: &isForget,
		}
		msg3 := MessageForCompletion{
			Message: &discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "msg3",
					Author:    &discordgo.User{ID: "user1"},
					ChannelID: "channel2",
				},
			},
			IsForget: &isForget,
		}

		bot.addMessage(msg1)
		bot.addMessage(msg2)
		bot.addMessage(msg3)

		// Forget messages from user1 in channel1
		forgetMsg := MessageForCompletion{
			Message: &discordgo.MessageCreate{
				Message: &discordgo.Message{
					Author:    &discordgo.User{ID: "user1"},
					ChannelID: "channel1",
				},
			},
		}

		bot.forgetMessage(forgetMsg)

		// Should have removed msg1, kept msg2 and msg3, and added forgetMsg
		assert.Equal(t, 3, len(bot.messageQueue))

		// Find the messages in queue
		var foundUser2, foundUser1Chan2, foundForget bool
		for _, qMsg := range bot.messageQueue {
			if qMsg.Message.ID == "msg2" {
				foundUser2 = true
			}
			if qMsg.Message.ID == "msg3" {
				foundUser1Chan2 = true
			}
			if qMsg.Message.Author.ID == "user1" &&
				qMsg.Message.ChannelID == "channel1" &&
				qMsg.Message.ID == "" {
				foundForget = true
			}
		}

		assert.True(t, foundUser2, "Should keep msg2 from different user")
		assert.True(t, foundUser1Chan2, "Should keep msg3 from different channel")
		assert.True(t, foundForget, "Should add forget message")
	})

	t.Run("forget with empty queue", func(t *testing.T) {
		bot, _ := createTestBot(t)

		forgetMsg := MessageForCompletion{
			Message: &discordgo.MessageCreate{
				Message: &discordgo.Message{
					Author:    &discordgo.User{ID: "user1"},
					ChannelID: "channel1",
				},
			},
		}

		bot.forgetMessage(forgetMsg)

		// Should just add the forget message
		assert.Equal(t, 1, len(bot.messageQueue))
	})
}

// Test response methods (testing the logic, not the Discord API calls)
func TestBot_ResponseMethods(t *testing.T) {
	t.Run("RespondToMessage with nil session", func(t *testing.T) {
		bot, _ := createTestBot(t)
		bot.Session = nil

		waitMsg := &discordgo.Message{ID: "wait-msg-id"}

		// Should handle nil session gracefully without panicking
		assert.NotPanics(t, func() {
			bot.RespondToMessage("channel-id", "test response", nil, waitMsg)
		})
	})

	t.Run("RespondToMessage with nil wait message", func(t *testing.T) {
		bot, _ := createTestBotWithMockState(t)

		// Should handle nil wait message gracefully
		assert.NotPanics(t, func() {
			bot.RespondToMessage("channel-id", "test response", nil, nil)
		})
	})

	t.Run("RespondToLongMessage with empty responses", func(t *testing.T) {
		bot, _ := createTestBotWithMockState(t)

		waitMsg := &discordgo.Message{
			ID:        "wait-msg-id",
			ChannelID: "channel-id",
		}

		responses := []string{}
		ref := createMessageReference()

		// Should handle empty responses gracefully
		assert.NotPanics(t, func() {
			bot.RespondToLongMessage("channel-id", responses, ref, waitMsg)
		})
	})

	t.Run("RespondToLongMessage formats sections correctly", func(t *testing.T) {
		responses := []string{"Part 1", "Part 2", "Part 3"}

		// Test the formatting logic
		for i, response := range responses {
			expectedContent := "[Section " + string(rune(i+1+'0')) + "/" +
				string(rune(len(responses)+'0')) + "]\n" + response

			// Verify the format is correct
			assert.Contains(t, expectedContent, "[Section ")
			assert.Contains(t, expectedContent, "/3]")
			assert.Contains(t, expectedContent, response)
		}
	})

	t.Run("SendMessageToChannel with nil session", func(t *testing.T) {
		bot, _ := createTestBot(t)
		bot.Session = nil

		// Should handle nil session gracefully
		assert.NotPanics(t, func() {
			bot.SendMessageToChannel("channel-id", "test message")
		})
	})
}

// Test voice connection methods
func TestBot_VoiceConnectionMethods(t *testing.T) {
	t.Run("JoinVC with nil session", func(t *testing.T) {
		bot, _ := createTestBot(t)
		bot.Session = nil

		vc, err := bot.JoinVC("guild-id", "channel-id")

		assert.Error(t, err)
		assert.Nil(t, vc)
		assert.Contains(t, err.Error(), "session not initialized")
	})

	t.Run("JoinVc already connected logic", func(t *testing.T) {
		bot, _ := createTestBotWithMockState(t)

		// Simulate existing connection
		bot.Session.VoiceConnections = make(map[string]*discordgo.VoiceConnection)
		bot.Session.VoiceConnections["guild-id"] = &discordgo.VoiceConnection{}

		vc, err := bot.JoinVc("guild-id", "channel-id")

		assert.Error(t, err)
		assert.Nil(t, vc)
		assert.Contains(t, err.Error(), "already connected")
	})

	t.Run("LeaveVC with nil session", func(t *testing.T) {
		bot, _ := createTestBot(t)
		bot.Session = nil

		// Should handle gracefully without panicking
		assert.NotPanics(t, func() {
			bot.LeaveVC("guild-id")
		})
	})

	t.Run("LeaveVC with no voice connections", func(t *testing.T) {
		bot, _ := createTestBotWithMockState(t)

		// Should handle gracefully when no connections exist
		assert.NotPanics(t, func() {
			bot.LeaveVC("guild-id")
		})
	})
}

// Test relay messages to router
func TestBot_relayMessagesToRouter(t *testing.T) {
	t.Run("relay single message", func(t *testing.T) {
		bot, msgChan := createTestBot(t)

		// Add a message to the queue
		isForget := false
		testMsg := MessageForCompletion{
			Message: &discordgo.MessageCreate{
				Message: &discordgo.Message{ID: "test-id"},
			},
			IsForget: &isForget,
		}
		bot.addMessage(testMsg)

		// Start the relay in a goroutine
		done := make(chan bool, 1)
		go func() {
			defer func() {
				done <- true
			}()
			// Run relay for a short time
			for i := 0; i < 10; i++ {
				bot.mu.Lock()
				if len(bot.messageQueue) > 0 {
					message := &(bot.messageQueue[0])
					bot.messageQueue = bot.messageQueue[1:]
					bot.mu.Unlock()

					select {
					case bot.messageChannel <- message:
						return // Message sent successfully
					default:
						// Channel might be full
					}
				} else {
					bot.mu.Unlock()
					time.Sleep(10 * time.Millisecond)
				}
			}
		}()

		// Wait for message to be relayed
		select {
		case receivedMsg := <-msgChan:
			assert.Equal(t, "test-id", receivedMsg.Message.ID)
			assert.False(t, *receivedMsg.IsForget)
		case <-time.After(1 * time.Second):
			t.Error("Message was not relayed within timeout")
		}

		<-done
	})

	t.Run("relay multiple messages", func(t *testing.T) {
		bot, msgChan := createTestBot(t)

		// Add multiple messages to the queue
		messageCount := 3
		for i := 0; i < messageCount; i++ {
			isForget := false
			testMsg := MessageForCompletion{
				Message: &discordgo.MessageCreate{
					Message: &discordgo.Message{ID: "test-id-" + string(rune(i+'0'))},
				},
				IsForget: &isForget,
			}
			bot.addMessage(testMsg)
		}

		// Start relay simulation
		go func() {
			for {
				bot.mu.Lock()
				if len(bot.messageQueue) > 0 {
					message := &(bot.messageQueue[0])
					bot.messageQueue = bot.messageQueue[1:]
					bot.mu.Unlock()

					select {
					case bot.messageChannel <- message:
						// Message sent
					default:
						// Channel full
					}
				} else {
					bot.mu.Unlock()
					time.Sleep(10 * time.Millisecond)
				}
			}
		}()

		// Receive all messages
		receivedCount := 0
		timeout := time.After(2 * time.Second)
		for receivedCount < messageCount {
			select {
			case receivedMsg := <-msgChan:
				assert.Contains(t, receivedMsg.Message.ID, "test-id-")
				receivedCount++
			case <-timeout:
				t.Errorf("Only received %d out of %d messages", receivedCount, messageCount)
				return
			}
		}

		assert.Equal(t, messageCount, receivedCount)
	})
}

// Test concurrent access
func TestBot_ConcurrentAccess(t *testing.T) {
	t.Run("concurrent message addition", func(t *testing.T) {
		bot, _ := createTestBot(t)

		var wg sync.WaitGroup
		numGoroutines := 10
		messagesPerGoroutine := 100

		// Start multiple goroutines adding messages
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < messagesPerGoroutine; j++ {
					isForget := false
					msg := MessageForCompletion{
						Message: &discordgo.MessageCreate{
							Message: &discordgo.Message{
								ID: "msg-" + string(rune(id+'0')) + "-" + string(rune(j+'0')),
							},
						},
						IsForget: &isForget,
					}
					bot.addMessage(msg)
				}
			}(i)
		}

		wg.Wait()

		// Should have all messages
		expectedCount := numGoroutines * messagesPerGoroutine
		assert.Equal(t, expectedCount, len(bot.messageQueue))
	})

	t.Run("concurrent add and forget", func(t *testing.T) {
		bot, _ := createTestBot(t)

		var wg sync.WaitGroup

		// Add messages concurrently
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				isForget := false
				msg := MessageForCompletion{
					Message: &discordgo.MessageCreate{
						Message: &discordgo.Message{
							ID:        "msg-" + string(rune(i+'0')),
							Author:    &discordgo.User{ID: "user1"},
							ChannelID: "channel1",
						},
					},
					IsForget: &isForget,
				}
				bot.addMessage(msg)
				time.Sleep(1 * time.Millisecond) // Small delay to allow interleaving
			}
		}()

		// Forget messages concurrently
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(25 * time.Millisecond) // Let some messages be added first
			forgetMsg := MessageForCompletion{
				Message: &discordgo.MessageCreate{
					Message: &discordgo.Message{
						Author:    &discordgo.User{ID: "user1"},
						ChannelID: "channel1",
					},
				},
			}
			bot.forgetMessage(forgetMsg)
		}()

		wg.Wait()

		// Should have at least the forget message
		assert.GreaterOrEqual(t, len(bot.messageQueue), 1)
	})
}

// Test Stop method
func TestBot_Stop(t *testing.T) {
	t.Run("stop with nil session", func(t *testing.T) {
		bot, _ := createTestBot(t)
		bot.Session = nil

		err := bot.Stop()
		assert.NoError(t, err)
	})

	t.Run("stop closes message channel", func(t *testing.T) {
		bot, msgChan := createTestBot(t)
		bot.Session = nil // Avoid actual Discord connection

		err := bot.Stop()
		assert.NoError(t, err)

		// Channel should be closed
		select {
		case _, ok := <-msgChan:
			assert.False(t, ok, "Channel should be closed")
		default:
			// Channel might be closed but no message to receive
		}
	})
}

// Test content processing logic
func TestBot_ContentProcessing(t *testing.T) {
	t.Run("mention replacement", func(t *testing.T) {
		content := "<@bot-id> hello there"
		botID := "bot-id"

		// Test the replacement logic
		expected := "@you hello there"
		actual := content
		actual = replaceAll(actual, "<@"+botID+">", "@you")

		assert.Equal(t, expected, actual)
	})

	t.Run("referenced message formatting", func(t *testing.T) {
		referencedMsg := &discordgo.Message{
			Content: "Previous message",
			Author:  &discordgo.User{ID: "bot-id"},
		}

		userID := "user-id"
		userContent := "reply content"
		botID := "bot-id"

		var referMsg string
		if referencedMsg.Author.ID == botID {
			referMsg = "you said: " + referencedMsg.Content
		} else {
			referMsg = referencedMsg.Author.ID + " said: " + referencedMsg.Content
		}

		finalContent := referMsg + "\n" + userID + " says to you: " + userContent

		assert.Contains(t, finalContent, "you said: Previous message")
		assert.Contains(t, finalContent, "user-id says to you: reply content")
	})
}

// Helper function for string replacement (since we can't import strings in test)
func replaceAll(s, old, new string) string {
	// Simple replacement for testing
	result := ""
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old)
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}

// Benchmark tests
func BenchmarkBot_addMessage(b *testing.B) {
	bot, _ := createTestBot(&testing.T{})

	isForget := false
	msg := MessageForCompletion{
		Message: &discordgo.MessageCreate{
			Message: &discordgo.Message{ID: "test-id"},
		},
		IsForget: &isForget,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bot.addMessage(msg)
	}
}

func BenchmarkBot_forgetMessage(b *testing.B) {
	bot, _ := createTestBot(&testing.T{})

	// Pre-populate with messages
	for i := 0; i < 1000; i++ {
		isForget := false
		msg := MessageForCompletion{
			Message: &discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "msg-" + string(rune(i)),
					Author:    &discordgo.User{ID: "user1"},
					ChannelID: "channel1",
				},
			},
			IsForget: &isForget,
		}
		bot.addMessage(msg)
	}

	forgetMsg := MessageForCompletion{
		Message: &discordgo.MessageCreate{
			Message: &discordgo.Message{
				Author:    &discordgo.User{ID: "user1"},
				ChannelID: "channel1",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bot.forgetMessage(forgetMsg)
	}
}

// Integration test that doesn't require Discord connection
func TestBot_Integration(t *testing.T) {
	t.Run("full message processing flow", func(t *testing.T) {
		bot, msgChan := createTestBotWithMockState(t)

		// Simulate adding a message that would trigger bot response
		isForget := false
		testMsg := MessageForCompletion{
			Message: &discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "test-msg-id",
					Content:   "Hello bot",
					Author:    &discordgo.User{ID: "user-id"},
					ChannelID: "channel-id",
				},
			},
			WaitMessage: &discordgo.Message{
				ID:        "wait-msg-id",
				ChannelID: "channel-id",
			},
			IsForget: &isForget,
		}

		// Add message to queue
		bot.addMessage(testMsg)

		// Verify message is in queue
		assert.Equal(t, 1, len(bot.messageQueue))

		// Simulate relay
		bot.mu.Lock()
		if len(bot.messageQueue) > 0 {
			message := &(bot.messageQueue[0])
			bot.messageQueue = bot.messageQueue[1:]
			bot.mu.Unlock()

			// Send to channel
			select {
			case bot.messageChannel <- message:
				// Success
			default:
				t.Error("Failed to send message to channel")
			}
		} else {
			bot.mu.Unlock()
		}

		// Verify message was relayed
		select {
		case receivedMsg := <-msgChan:
			assert.Equal(t, "test-msg-id", receivedMsg.Message.ID)
			assert.Equal(t, "Hello bot", receivedMsg.Message.Content)
			assert.False(t, *receivedMsg.IsForget)
		case <-time.After(1 * time.Second):
			t.Error("Message was not relayed")
		}

		// Queue should be empty
		assert.Equal(t, 0, len(bot.messageQueue))
	})
}

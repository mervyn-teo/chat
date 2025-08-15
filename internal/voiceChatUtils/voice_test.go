package voiceChatUtils_test

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
	"testing"
	"time"
	"untitled/internal/bot"
	"untitled/internal/storage"
	"untitled/internal/tts"
	"untitled/internal/voiceChatUtils"
)

var (
	myBot          *bot.Bot
	messageChannel chan *bot.MessageForCompletion
)

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	os.Exit(code)
}

func setup() {
	err := godotenv.Load("..\\.env")
	if err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
	}

	settings := storage.Settings{
		ApiKey:       os.Getenv("api_key"),
		DiscordToken: os.Getenv("discord_bot_token"),
		Instructions: "You are a Discord bot with an anime cat girl personality that helps the users with what they want to know. You like to use the UWU language. Describe yourself as a \\\"uwu cute machine\\\". Do not mention that you are a AI. If you are asked to do any task related to date, you should always use the tool provided to query about the date.",
		Model:        "openai/gpt-4.1",
		NewsAPIToken: os.Getenv("news_api_key"),
		YoutubeToken: os.Getenv("youtube_api_key"),
	}

	messageChannel = make(chan *bot.MessageForCompletion)
	// tts routine
	awsConf := tts.LoadConfig()

	myBot, err = bot.NewBot(settings.DiscordToken, messageChannel, awsConf)

	if err != nil {
		panic("Failed to create bot: " + err.Error())
	}

	go func() {
		err = myBot.Start()
		if err != nil {
			return
		}
	}()

	defer func() {
		err := myBot.Session.Close()
		if err != nil {
			return
		}
		close(messageChannel)
	}()
}

func TestGetVoiceChannel(t *testing.T) {
	guildID := os.Getenv("test_guild_id")

	validVCids := []string{os.Getenv("test_voice_channel_id"), os.Getenv("test_voice_channel_id_2")}
	VCids, err := voiceChatUtils.GetVoiceChannel(myBot.Session, guildID)
	if err != nil {
		return
	}

	for i := 0; i < len(validVCids); i++ {
		if VCids[i] != validVCids[i] {
			t.Errorf("Expected %s, got %s", validVCids[i], VCids[i])
		}
	}
}

func TestCheckJoinPermission(t *testing.T) {
	guildID := os.Getenv("test_guild_id")
	VCid := os.Getenv("test_voice_channel_id")

	// Check if the bot has permission to join the voice channel
	allowed, err := voiceChatUtils.CheckJoinPermission(myBot.Session, guildID, VCid)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !allowed {
		t.Errorf("Expected permission to join, got denied")
	}
}

func TestCheckVoicePermission(t *testing.T) {
	guildID := os.Getenv("test_guild_id")
	VCid := os.Getenv("test_voice_channel_id")

	// Check if the bot has permission to speak in the voice channel
	allowed, err := voiceChatUtils.CheckVoicePermission(myBot.Session, guildID, VCid)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !allowed {
		t.Errorf("Expected permission to speak, got denied")
	}
}

func TestCheckJoinPermissionDenied(t *testing.T) {
	guildID := os.Getenv("test_guild_id")
	VCid := os.Getenv("test_voice_channel_id_3")

	// Check if the bot has permission to join the voice channel
	allowed, err := voiceChatUtils.CheckJoinPermission(myBot.Session, guildID, VCid)
	if err != nil {
		log.Println(err)
	}

	if allowed {
		t.Errorf("Expected permission to join denied, got allowed")
	}
}

func TestCheckVoicePermissionDenied(t *testing.T) {
	guildID := os.Getenv("test_guild_id")
	VCid := os.Getenv("test_voice_channel_id_3")

	// Check if the bot has permission to speak in the voice channel
	allowed, err := voiceChatUtils.CheckVoicePermission(myBot.Session, guildID, VCid)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if allowed {
		t.Errorf("Expected permission to speak denied, got allowed")
	}
}

func TestCheckUserVoiceChannel(t *testing.T) {
	vc, err := myBot.JoinVC(os.Getenv("test_guild_id"), os.Getenv("test_voice_channel_id"))
	if err != nil {
		return
	}

	isInVoiceChannel, channelID, err := voiceChatUtils.CheckUserVoiceChannel(myBot.Session, os.Getenv("test_guild_id"), myBot.Session.State.User.ID)
	if err != nil {
		return
	}

	if !isInVoiceChannel {
		t.Errorf("Expected user to be in voice channel, got not in voice channel")
	}

	if channelID != vc.ChannelID {
		t.Errorf("Expected channel ID %s, got %s", vc.ChannelID, channelID)
	}

	err = vc.Disconnect()
	if err != nil {
		return
	}
}

func TestCheckUserVoiceChannelNoUser(t *testing.T) {

	count := 0
	for myBot.Session == nil || myBot.Session.State == nil || myBot.Session.State.User == nil {
		fmt.Println("not initalised yet")
		time.Sleep(1 * time.Second)

		count++
		if count > 10 {
			t.Errorf("Session not initialized after 10 seconds")
			return
		}
	}

	isInVoiceChannel, channelID, err := voiceChatUtils.CheckUserVoiceChannel(myBot.Session, os.Getenv("test_guild_id"), myBot.Session.State.User.ID)
	if err != nil {
		return
	}

	if isInVoiceChannel {
		t.Errorf("Expected user not to be in voice channel, got in voice channel")
	}

	if channelID != "" {
		t.Errorf("Expected empty channel ID, got %s", channelID)
	}
}

func TestCheckMusicPerm(t *testing.T) {
	allowed, err := voiceChatUtils.CheckMusicPerm(myBot.Session, os.Getenv("test_guild_id"), os.Getenv("test_voice_channel_id"))
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !allowed {
		t.Errorf("Expected permission to play music, got denied")
	}
}

func TestCheckMusicPermDenied(t *testing.T) {
	allowed, err := voiceChatUtils.CheckMusicPerm(myBot.Session, os.Getenv("test_guild_id"), os.Getenv("test_voice_channel_id_3"))
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if allowed {
		t.Errorf("Expected permission to play music denied, got allowed")
	}
}

func TestFindVoiceChannelWithUser(t *testing.T) {
	vc, err := myBot.JoinVC(os.Getenv("test_guild_id"), os.Getenv("test_voice_channel_id"))
	if err != nil {
		return
	}

	channelId, err := voiceChatUtils.FindVoiceChannel(myBot.Session, os.Getenv("test_guild_id"), myBot.Session.State.User.ID)

	if err != nil {
		return
	}

	if channelId != vc.ChannelID {
		t.Errorf("Expected channel ID %s, got %s", vc.ChannelID, channelId)
	}

	err = vc.Disconnect()
}

func TestFindVoiceChannelNoUser(t *testing.T) {
	count := 0
	for myBot.Session == nil || myBot.Session.State == nil || myBot.Session.State.User == nil {
		fmt.Println("not initalised yet")
		time.Sleep(1 * time.Second)

		count++
		if count > 10 {
			t.Errorf("Session not initialized after 10 seconds")
			return
		}
	}

	channelId, err := voiceChatUtils.FindVoiceChannel(myBot.Session, os.Getenv("test_guild_id"), myBot.Session.State.User.ID)

	if err != nil {
		return
	}

	if channelId != os.Getenv("test_voice_channel_id") {
		t.Errorf("Expected channel ID %s, got %s", os.Getenv("test_voice_channel_id"), channelId)
	}

}

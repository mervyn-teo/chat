package tools

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"untitled/internal/bot"
	"untitled/internal/music"
	"untitled/internal/storage"
)

type MusicTestSuite struct {
	suite.Suite
	bot            *bot.Bot
	messageChannel chan *bot.MessageWithWait
	songMap        map[string]map[string]*music.SongList
	testGuildID    string
	testChannelID  string
}

func TestMusicTestSuite(t *testing.T) {
	suite.Run(t, new(MusicTestSuite))
}

func (suite *MusicTestSuite) SetupSuite() {
	// Load environment variables
	err := godotenv.Load("../../.env")
	if err != nil {
		suite.T().Logf("Warning: Error loading .env file: %v", err)
	}

	// Validate required environment variables
	suite.testGuildID = os.Getenv("test_guild_id")
	suite.testChannelID = os.Getenv("test_channel_id")

	require.NotEmpty(suite.T(), suite.testGuildID, "test_guild_id must be set")
	require.NotEmpty(suite.T(), suite.testChannelID, "test_channel_id must be set")

	// Setup bot only if not skipping tests
	if os.Getenv("SKIP_YOUTUBE_TESTS") != "true" {
		suite.setupBot()
	}
}

func (suite *MusicTestSuite) setupBot() {
	settings := storage.Settings{
		ApiKey:       os.Getenv("api_key"),
		DiscordToken: os.Getenv("discord_bot_token"),
		Instructions: "Test bot instance",
		Model:        "openai/gpt-4.1",
		NewsAPIToken: os.Getenv("news_api_key"),
		YoutubeToken: os.Getenv("youtube_api_key"),
	}

	require.NotEmpty(suite.T(), settings.DiscordToken, "discord_bot_token must be set")

	suite.messageChannel = make(chan *bot.MessageWithWait, 10)

	var err error
	suite.bot, err = bot.NewBot(settings.DiscordToken, suite.messageChannel)
	require.NoError(suite.T(), err, "Failed to create bot")

	// Start bot in goroutine
	go func() {
		if err := suite.bot.Start(); err != nil {
			suite.T().Logf("Bot start error: %v", err)
		}
	}()

	// Give bot time to start
	time.Sleep(2 * time.Second)
}

func (suite *MusicTestSuite) TearDownSuite() {
	if suite.bot != nil && suite.bot.Session != nil {
		suite.bot.Session.Close()
	}
	if suite.messageChannel != nil {
		close(suite.messageChannel)
	}
}

func (suite *MusicTestSuite) SetupTest() {
	suite.songMap = make(map[string]map[string]*music.SongList)
	suite.createMockSonglist()
}

func (suite *MusicTestSuite) TearDownTest() {
	// Clean up any playing songs
	if songlist, exists := suite.getSongList(); exists && songlist.IsPlaying {
		songlist.StopSong()
		time.Sleep(100 * time.Millisecond) // Brief pause for cleanup
	}
}

func (suite *MusicTestSuite) createMockSonglist() {
	songlist := music.SongList{
		Songs:     make([]music.Song, 0),
		IsPlaying: false,
		Mu:        sync.Mutex{},
		StopSig:   make(chan bool, 1),
	}

	testSongs := []music.Song{
		{
			Title: "Song 1",
			Id:    "d3J3uJpCgos",
			Url:   "https://www.youtube.com/watch?v=d3J3uJpCgos",
		},
		{
			Title: "Song 2",
			Id:    "1MvvFXBWFjI",
			Url:   "https://www.youtube.com/watch?v=1MvvFXBWFjI",
		},
		{
			Title: "Song 3",
			Id:    "kzZ6KXDM1RI",
			Url:   "https://www.youtube.com/watch?v=kzZ6KXDM1RI",
		},
		{
			Title: "Song 4",
			Id:    "02Q4yUMw3Ds",
			Url:   "https://www.youtube.com/watch?v=02Q4yUMw3Ds",
		},
	}

	songlist.Songs = append(songlist.Songs, testSongs...)

	if suite.songMap[suite.testGuildID] == nil {
		suite.songMap[suite.testGuildID] = make(map[string]*music.SongList)
	}
	suite.songMap[suite.testGuildID][suite.testChannelID] = &songlist
}

func (suite *MusicTestSuite) getSongList() (*music.SongList, bool) {
	if guildMap, exists := suite.songMap[suite.testGuildID]; exists {
		if songlist, exists := guildMap[suite.testChannelID]; exists {
			return songlist, true
		}
	}
	return nil, false
}

func (suite *MusicTestSuite) createToolCall(functionName, args string) openai.ToolCall {
	if args == "" {
		args = fmt.Sprintf(`{"gid": "%s", "cid": "%s"}`,
			suite.testGuildID, suite.testChannelID)
	}

	return openai.ToolCall{
		Function: openai.FunctionCall{
			Name:      functionName,
			Arguments: args,
		},
	}
}

func (suite *MusicTestSuite) TestAddSong() {
	args := fmt.Sprintf(`{"gid": "%s", "cid": "%s", "title": "New Song", "url": "https://www.youtube.com/watch?v=cLdWfNbBMvc"}`,
		suite.testGuildID, suite.testChannelID)

	call := suite.createToolCall("add_song", args)

	msg, err := addSong(call, &suite.songMap)

	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), msg, "Song added successfully")

	songlist, exists := suite.getSongList()
	require.True(suite.T(), exists)
	assert.Equal(suite.T(), "New Song", songlist.Songs[len(songlist.Songs)-1].Title)
}

func (suite *MusicTestSuite) TestRemoveSong() {
	args := fmt.Sprintf(`{"gid": "%s", "cid": "%s", "uuid": "d3J3uJpCgos"}`,
		suite.testGuildID, suite.testChannelID)

	call := suite.createToolCall("remove_song", args)

	// Get initial count
	songlist, exists := suite.getSongList()
	require.True(suite.T(), exists)
	initialCount := len(songlist.Songs)

	msg, err := removeSong(call, &suite.songMap)

	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), msg, "Song removed successfully")
	assert.Len(suite.T(), songlist.Songs, initialCount-1)

	// Verify the specific song was removed
	for _, song := range songlist.Songs {
		assert.NotEqual(suite.T(), "d3J3uJpCgos", song.Id)
	}
}

func (suite *MusicTestSuite) TestGetCurrentSongList() {
	call := suite.createToolCall("get_current_song_list", "")

	msg, err := getCurrentSongList(call, &suite.songMap)

	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), msg, "Song 1")
	assert.Contains(suite.T(), msg, "Song 2")
	assert.Contains(suite.T(), msg, "Song 3")
	assert.Contains(suite.T(), msg, "Song 4")
	assert.Contains(suite.T(), msg, "d3J3uJpCgos")
}

func (suite *MusicTestSuite) TestPlayMusic() {
	if os.Getenv("SKIP_YOUTUBE_TESTS") == "true" {
		suite.T().Skip("Skipping YouTube tests")
	}

	require.NotNil(suite.T(), suite.bot, "Bot must be initialized for play tests")

	call := suite.createToolCall("play_song", "")

	msg, err := playSong(call, &suite.songMap, suite.bot)

	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), msg, "Playing song successfully")
	assert.Contains(suite.T(), msg, "Song 1")
	assert.Contains(suite.T(), msg, "d3J3uJpCgos")

	// Verify song is actually playing
	songlist, exists := suite.getSongList()
	require.True(suite.T(), exists)

	// Wait a moment for playback to start
	time.Sleep(2 * time.Second)
	assert.True(suite.T(), songlist.IsPlaying)
}

func (suite *MusicTestSuite) TestSkipSong() {
	if os.Getenv("SKIP_YOUTUBE_TESTS") == "true" {
		suite.T().Skip("Skipping YouTube tests")
	}

	require.NotNil(suite.T(), suite.bot, "Bot must be initialized for skip tests")

	call := suite.createToolCall("", "")

	// Start playing first
	_, err := playSong(call, &suite.songMap, suite.bot)
	require.NoError(suite.T(), err)

	time.Sleep(2 * time.Second) // Let song start

	// Skip to next song
	msg, err := skipSong(call, &suite.songMap, suite.bot)

	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), msg, "Song skipped successfully")
	assert.Contains(suite.T(), msg, "Song 2") // Should be playing second song

	time.Sleep(1 * time.Second) // Brief pause before cleanup
}

func (suite *MusicTestSuite) TestPauseSong() {
	if os.Getenv("SKIP_YOUTUBE_TESTS") == "true" {
		suite.T().Skip("Skipping YouTube tests")
	}

	require.NotNil(suite.T(), suite.bot, "Bot must be initialized for pause tests")

	call := suite.createToolCall("", "")

	// Start playing first
	_, err := playSong(call, &suite.songMap, suite.bot)
	require.NoError(suite.T(), err)

	time.Sleep(2 * time.Second) // Let song start

	// Pause the song
	msg, err := pauseSong(call, &suite.songMap)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "Song paused successfully", msg)
}

func (suite *MusicTestSuite) TestStopSong() {
	if os.Getenv("SKIP_YOUTUBE_TESTS") == "true" {
		suite.T().Skip("Skipping YouTube tests")
	}

	require.NotNil(suite.T(), suite.bot, "Bot must be initialized for stop tests")

	call := suite.createToolCall("", "")

	// Start playing first
	_, err := playSong(call, &suite.songMap, suite.bot)
	require.NoError(suite.T(), err)

	time.Sleep(2 * time.Second) // Let song start

	// Stop the song
	msg, err := stopSong(call, &suite.songMap)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "Song stopped successfully", msg)

	// Verify song is actually stopped
	songlist, exists := suite.getSongList()
	require.True(suite.T(), exists)

	time.Sleep(500 * time.Millisecond) // Brief pause for stop to take effect
	assert.False(suite.T(), songlist.IsPlaying)
}

// Benchmark tests
func BenchmarkAddSong(b *testing.B) {
	suite := &MusicTestSuite{}
	suite.SetupSuite()
	defer suite.TearDownSuite()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		suite.SetupTest()
		args := fmt.Sprintf(`{"gid": "%s", "cid": "%s", "title": "Benchmark Song %d", "url": "https://www.youtube.com/watch?v=test%d"}`,
			suite.testGuildID, suite.testChannelID, i, i)
		call := suite.createToolCall("add_song", args)
		addSong(call, &suite.songMap)
	}
}

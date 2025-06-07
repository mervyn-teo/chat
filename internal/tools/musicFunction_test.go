package tools

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
	"untitled/internal/tts"

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
	messageChannel chan *bot.MessageForCompletion
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

	suite.messageChannel = make(chan *bot.MessageForCompletion, 10)

	var err error
	// tts routine
	awsConf := tts.LoadConfig()

	suite.bot, err = bot.NewBot(settings.DiscordToken, suite.messageChannel, awsConf)
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
		err := suite.bot.Session.Close()
		if err != nil {
			return
		}
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
	// Clean up any playing songs safely
	if songlist, exists := suite.getSongList(); exists {
		// Only try to stop if there's actually a voice connection
		// This prevents panics when we mock IsPlaying without a real connection
		if songlist.IsPlaying {
			// Reset the playing state manually for mocked tests
			songlist.Mu.Lock()
			songlist.IsPlaying = false
			songlist.Mu.Unlock()

		}
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
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      functionName,
			Arguments: args,
		},
	}
}

// Helper method to safely set playing state for tests
func (suite *MusicTestSuite) setPlayingState(isPlaying bool) {
	songlist, exists := suite.getSongList()
	require.True(suite.T(), exists)

	songlist.Mu.Lock()
	songlist.IsPlaying = isPlaying
	songlist.Mu.Unlock()
}

// Test HandleMusicCall function
func (suite *MusicTestSuite) TestHandleMusicCall_UnsupportedToolType() {
	call := openai.ToolCall{
		Type: "unsupported_type",
		Function: openai.FunctionCall{
			Name:      "test_function",
			Arguments: "{}",
		},
	}

	msg, err := HandleMusicCall(call, &suite.songMap, suite.bot)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), msg, "Unsupported tool type")
	assert.Contains(suite.T(), err.Error(), "unsupported tool type")
}

func (suite *MusicTestSuite) TestHandleMusicCall_UnknownFunction() {
	call := suite.createToolCall("unknown_function", "")

	msg, err := HandleMusicCall(call, &suite.songMap, suite.bot)

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), msg)
	assert.Contains(suite.T(), err.Error(), "function unknown_function not found")
}

func (suite *MusicTestSuite) TestHandleMusicCall_ValidFunctions() {
	testCases := []struct {
		functionName string
		args         string
		expectError  bool
	}{
		{
			functionName: "get_current_songList",
			args:         "",
			expectError:  false,
		},
		{
			functionName: "add_song",
			args: fmt.Sprintf(`{"gid": "%s", "cid": "%s", "title": "Test Song", "url": "https://www.youtube.com/watch?v=test"}`,
				suite.testGuildID, suite.testChannelID),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.functionName, func(t *testing.T) {
			call := suite.createToolCall(tc.functionName, tc.args)

			msg, err := HandleMusicCall(call, &suite.songMap, suite.bot)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, msg)
			}
		})
	}
}

// Test addSong function
func (suite *MusicTestSuite) TestAddSong_Success() {
	args := fmt.Sprintf(`{"gid": "%s", "cid": "%s", "title": "New Song", "url": "https://www.youtube.com/watch?v=cLdWfNbBMvc"}`,
		suite.testGuildID, suite.testChannelID)

	call := suite.createToolCall("add_song", args)

	msg, err := addSong(call, &suite.songMap)

	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), msg, "Song added successfully")
	assert.Contains(suite.T(), msg, "New Song")
	assert.Contains(suite.T(), msg, "https://www.youtube.com/watch?v=cLdWfNbBMvc")

	songlist, exists := suite.getSongList()
	require.True(suite.T(), exists)
	assert.Equal(suite.T(), "New Song", songlist.Songs[len(songlist.Songs)-1].Title)
}

func (suite *MusicTestSuite) TestAddSong_InvalidJSON() {
	call := suite.createToolCall("add_song", "invalid json")

	msg, err := addSong(call, &suite.songMap)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), msg, "Failed to parse arguments")
	assert.Contains(suite.T(), err.Error(), "argument parsing failed")
}

func (suite *MusicTestSuite) TestAddSong_NilSongMap() {
	// Create a nil song map to test the nil check
	var nilSongMap map[string]map[string]*music.SongList

	args := fmt.Sprintf(`{"gid": "%s", "cid": "%s", "title": "Test Song", "url": "https://www.youtube.com/watch?v=test"}`,
		suite.testGuildID, suite.testChannelID)
	call := suite.createToolCall("add_song", args)

	// This should handle the nil case gracefully
	msg, err := addSong(call, &nilSongMap)

	// The function should either handle this gracefully or return an appropriate error
	if err != nil {
		assert.Contains(suite.T(), err.Error(), "failed to add song")
	} else {
		assert.Contains(suite.T(), msg, "Song added successfully")
	}
}

// Test removeSong function
func (suite *MusicTestSuite) TestRemoveSong_Success() {
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

func (suite *MusicTestSuite) TestRemoveSong_SongListNotFound() {
	args := fmt.Sprintf(`{"gid": "nonexistent", "cid": "nonexistent", "uuid": "test"}`)
	call := suite.createToolCall("remove_song", args)

	msg, err := removeSong(call, &suite.songMap)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), msg, "Cannot remove song, song list not found")
	assert.Contains(suite.T(), err.Error(), "song list not found")
}

func (suite *MusicTestSuite) TestRemoveSong_CannotRemovePlayingSong() {
	// Set the song as playing safely
	suite.setPlayingState(true)

	args := fmt.Sprintf(`{"gid": "%s", "cid": "%s", "uuid": "d3J3uJpCgos"}`,
		suite.testGuildID, suite.testChannelID)
	call := suite.createToolCall("remove_song", args)

	msg, err := removeSong(call, &suite.songMap)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), msg, "Cannot remove song while playing")
	assert.Contains(suite.T(), err.Error(), "cannot remove song while playing")

	// Reset state
	suite.setPlayingState(false)
}

// Test getCurrentSongList function
func (suite *MusicTestSuite) TestGetCurrentSongList_Success() {
	call := suite.createToolCall("get_current_songList", "")

	msg, err := getCurrentSongList(call, &suite.songMap)

	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), msg, "songs: [")
	assert.Contains(suite.T(), msg, "Song 1")
	assert.Contains(suite.T(), msg, "Song 2")
	assert.Contains(suite.T(), msg, "Song 3")
	assert.Contains(suite.T(), msg, "Song 4")
	assert.Contains(suite.T(), msg, "d3J3uJpCgos")
}

func (suite *MusicTestSuite) TestGetCurrentSongList_EmptyList() {
	// Create empty songlist
	suite.songMap[suite.testGuildID][suite.testChannelID].Songs = []music.Song{}

	call := suite.createToolCall("get_current_songList", "")

	msg, err := getCurrentSongList(call, &suite.songMap)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "songs: []", msg)
}

func (suite *MusicTestSuite) TestGetCurrentSongList_NilSongList() {
	// Remove the songlist to test nil handling
	delete(suite.songMap[suite.testGuildID], suite.testChannelID)

	call := suite.createToolCall("get_current_songList", "")

	msg, err := getCurrentSongList(call, &suite.songMap)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "songs: []", msg)

	// Verify that a new songlist was created
	songlist, exists := suite.getSongList()
	assert.True(suite.T(), exists)
	assert.NotNil(suite.T(), songlist)
}

// Test playSong function (requires bot)
func (suite *MusicTestSuite) TestPlaySong_Success() {
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

func (suite *MusicTestSuite) TestPlaySong_SongListNotFound() {
	args := fmt.Sprintf(`{"gid": "nonexistent", "cid": "nonexistent"}`)
	call := suite.createToolCall("play_song", args)

	msg, err := playSong(call, &suite.songMap, suite.bot)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), msg, "Cannot play song, song list not found")
	assert.Contains(suite.T(), err.Error(), "song list not found")
}

func (suite *MusicTestSuite) TestPlaySong_AlreadyPlaying() {
	// Set song as already playing safely
	suite.setPlayingState(true)

	call := suite.createToolCall("play_song", "")

	msg, err := playSong(call, &suite.songMap, suite.bot)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), msg, "Cannot play song while already playing")
	assert.Contains(suite.T(), err.Error(), "cannot play song while already playing")

	// Reset state for cleanup
	suite.setPlayingState(false)
}

// Test skipSong function
func (suite *MusicTestSuite) TestSkipSong_Success() {
	if os.Getenv("SKIP_YOUTUBE_TESTS") == "true" {
		suite.T().Skip("Skipping YouTube tests")
	}

	require.NotNil(suite.T(), suite.bot, "Bot must be initialized for skip tests")

	call := suite.createToolCall("skip_song", "")

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

func (suite *MusicTestSuite) TestSkipSong_NoSongsToSkip() {
	// Create songlist with only one song
	songlist, exists := suite.getSongList()
	require.True(suite.T(), exists)
	songlist.Songs = songlist.Songs[:1] // Keep only first song

	call := suite.createToolCall("skip_song", "")

	msg, err := skipSong(call, &suite.songMap, suite.bot)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), msg, "No songs to skip")
	assert.Contains(suite.T(), err.Error(), "no songs to skip")
}

func (suite *MusicTestSuite) TestSkipSong_SongListNotFound() {
	args := fmt.Sprintf(`{"gid": "nonexistent", "cid": "nonexistent"}`)
	call := suite.createToolCall("skip_song", args)

	msg, err := skipSong(call, &suite.songMap, suite.bot)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), msg, "Cannot skip song, song list not found")
	assert.Contains(suite.T(), err.Error(), "song list not found")
}

// Test pauseSong function
func (suite *MusicTestSuite) TestPauseSong_Success() {
	if os.Getenv("SKIP_YOUTUBE_TESTS") == "true" {
		suite.T().Skip("Skipping YouTube tests")
	}

	require.NotNil(suite.T(), suite.bot, "Bot must be initialized for pause tests")

	call := suite.createToolCall("pause_song", "")

	// Start playing first
	_, err := playSong(call, &suite.songMap, suite.bot)
	require.NoError(suite.T(), err)

	time.Sleep(2 * time.Second) // Let song start

	// Pause the song
	msg, err := pauseSong(call, &suite.songMap)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "Song paused successfully", msg)
}

func (suite *MusicTestSuite) TestPauseSong_NotPlaying() {
	call := suite.createToolCall("pause_song", "")

	msg, err := pauseSong(call, &suite.songMap)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), msg, "Cannot pause song while not playing")
	assert.Contains(suite.T(), err.Error(), "cannot pause song while not playing")
}

func (suite *MusicTestSuite) TestPauseSong_SongListNotFound() {
	args := fmt.Sprintf(`{"gid": "nonexistent", "cid": "nonexistent"}`)
	call := suite.createToolCall("pause_song", args)

	msg, err := pauseSong(call, &suite.songMap)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), msg, "Cannot pause song, song list not found")
	assert.Contains(suite.T(), err.Error(), "song list not found")
}

// Test stopSong function
func (suite *MusicTestSuite) TestStopSong_Success() {
	if os.Getenv("SKIP_YOUTUBE_TESTS") == "true" {
		suite.T().Skip("Skipping YouTube tests")
	}

	require.NotNil(suite.T(), suite.bot, "Bot must be initialized for stop tests")

	call := suite.createToolCall("stop_song", "")

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

func (suite *MusicTestSuite) TestStopSong_NotPlaying() {
	call := suite.createToolCall("stop_song", "")

	msg, err := stopSong(call, &suite.songMap)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), msg, "Cannot stop song while not playing")
	assert.Contains(suite.T(), err.Error(), "cannot stop song while not playing")
}

func (suite *MusicTestSuite) TestStopSong_SongListNotFound() {
	args := fmt.Sprintf(`{"gid": "nonexistent", "cid": "nonexistent"}`)
	call := suite.createToolCall("stop_song", args)

	msg, err := stopSong(call, &suite.songMap)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), msg, "Cannot stop song, song list not found")
	assert.Contains(suite.T(), err.Error(), "song list not found")
}

// Test argument parsing errors for all functions
func (suite *MusicTestSuite) TestArgumentParsingErrors() {
	functions := []string{
		"get_current_songList",
		"add_song",
		"remove_song",
		"skip_song",
		"play_song",
		"pause_song",
		"stop_song",
	}

	for _, funcName := range functions {
		suite.T().Run(funcName+"_InvalidJSON", func(t *testing.T) {
			call := suite.createToolCall(funcName, "invalid json")

			var msg string
			var err error

			switch funcName {
			case "get_current_songList":
				msg, err = getCurrentSongList(call, &suite.songMap)
			case "add_song":
				msg, err = addSong(call, &suite.songMap)
			case "remove_song":
				msg, err = removeSong(call, &suite.songMap)
			case "skip_song":
				msg, err = skipSong(call, &suite.songMap, suite.bot)
			case "play_song":
				msg, err = playSong(call, &suite.songMap, suite.bot)
			case "pause_song":
				msg, err = pauseSong(call, &suite.songMap)
			case "stop_song":
				msg, err = stopSong(call, &suite.songMap)
			}

			assert.Error(t, err)
			assert.Contains(t, msg, "Failed to parse arguments")
			assert.Contains(t, err.Error(), "argument parsing failed")
		})
	}
}

// Benchmark tests
func BenchmarkAddSong(b *testing.B) {
	s := &MusicTestSuite{}
	s.SetupSuite()
	defer s.TearDownSuite()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.SetupTest()
		args := fmt.Sprintf(`{"gid": "%s", "cid": "%s", "title": "Benchmark Song %d", "url": "https://www.youtube.com/watch?v=test%d"}`,
			s.testGuildID, s.testChannelID, i, i)
		call := s.createToolCall("add_song", args)
		_, err := addSong(call, &s.songMap)
		if err != nil {
			return
		}
	}
}

func BenchmarkGetCurrentSongList(b *testing.B) {
	s := &MusicTestSuite{}
	s.SetupSuite()
	s.SetupTest()
	defer s.TearDownSuite()

	call := s.createToolCall("get_current_songList", "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := getCurrentSongList(call, &s.songMap)
		if err != nil {
			return
		}
	}
}

func BenchmarkHandleMusicCall(b *testing.B) {
	s := &MusicTestSuite{}
	s.SetupSuite()
	s.SetupTest()
	defer s.TearDownSuite()

	call := s.createToolCall("get_current_songList", "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := HandleMusicCall(call, &s.songMap, s.bot)
		if err != nil {
			return
		}
	}
}

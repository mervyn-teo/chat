package tools

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/sashabaranov/go-openai"
	"os"
	"sync"
	"testing"
	"time"
	"untitled/internal/bot"
	"untitled/internal/music"
	"untitled/internal/storage"
)

var (
	myBot          *bot.Bot
	messageChannel chan *bot.MessageWithWait
	songlist       music.SongList
	songMap        map[string]map[string]*music.SongList = make(map[string]map[string]*music.SongList)
)

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	os.Exit(code)
}

func setup() {
	err := godotenv.Load("../../.env")
	if err != nil {
		panic("Error loading .env file")
	}

	settings := storage.Settings{
		ApiKey:       os.Getenv("api_key"),
		DiscordToken: os.Getenv("discord_bot_token"),
		Instructions: "You are a Discord bot with an anime cat girl personality that helps the users with what they want to know. You like to use the UWU language. Describe yourself as a \\\"uwu cute machine\\\". Do not mention that you are a AI. If you are asked to do any task related to date, you should always use the tool provided to query about the date.",
		Model:        "openai/gpt-4.1",
		NewsAPIToken: os.Getenv("news_api_key"),
		YoutubeToken: os.Getenv("youtube_api_key"),
	}

	messageChannel = make(chan *bot.MessageWithWait)
	myBot, err = bot.NewBot(settings.DiscordToken, messageChannel)
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

func mockSonglist() {
	songlist = music.SongList{
		Songs:     make([]music.Song, 0),
		IsPlaying: false,
		Mu:        sync.Mutex{},
		StopSig:   make(chan bool),
	}

	songlist.Songs = append(songlist.Songs, music.Song{
		Title: "Song 1",
		Id:    "d3J3uJpCgos",
		Url:   "https://www.youtube.com/watch?v=d3J3uJpCgos&list=PLwCTYY94JxbZHrJ-anoUuFkNHSFQqe438&index=6&pp=gAQBiAQB8AUB",
	})

	songlist.Songs = append(songlist.Songs, music.Song{
		Title: "Song 2",
		Id:    "1MvvFXBWFjI",
		Url:   "https://www.youtube.com/watch?v=1MvvFXBWFjI&list=PLwCTYY94JxbZHrJ-anoUuFkNHSFQqe438&index=2&pp=gAQBiAQB8AUB",
	})

	songlist.Songs = append(songlist.Songs, music.Song{
		Title: "Song 3",
		Id:    "kzZ6KXDM1RI",
		Url:   "https://www.youtube.com/watch?v=kzZ6KXDM1RI&pp=ygUQeXV1cmkgZHJ5IGZsb3dlctIHCQmGCQGHKiGM7w%3D%3D",
	})

	songlist.Songs = append(songlist.Songs, music.Song{
		Title: "Song 4",
		Id:    "02Q4yUMw3Ds",
		Url:   "https://www.youtube.com/watch?v=02Q4yUMw3Ds&list=RDMM&start_radio=1",
	})

	songMap[os.Getenv("test_guild_id")] = make(map[string]*music.SongList)
	songMap[os.Getenv("test_guild_id")][os.Getenv("test_channel_id")] = &songlist
}

func TestPlayMusic(t *testing.T) {
	mockSonglist()
	call := openai.ToolCall{
		Function: openai.FunctionCall{
			Arguments: `{"gid": "` + os.Getenv("test_guild_id") + `", "cid": "` + os.Getenv("test_channel_id") + `"}`,
		},
	}
	msg, err := playSong(call, &songMap, myBot)
	time.Sleep(10 * time.Second)
	if err != nil {
		return
	}
	if msg != `Playing song successfully, song title: Song 1, song Id: d3J3uJpCgos` {
		t.Errorf("Expected song to be played successfully, got: %s", msg)
	}
	err = songlist.StopSong()
	if err != nil {
		return
	}
}

func TestAddSong(t *testing.T) {
	mockSonglist()
	mockFunctionCall := openai.ToolCall{
		Function: openai.FunctionCall{
			Name:      "add_song",
			Arguments: `{"gid": "` + os.Getenv("test_guild_id") + `", "cid": "` + os.Getenv("test_channel_id") + `", "title": "Song 4", "url": "https://www.youtube.com/watch?v=cLdWfNbBMvc"}`,
		},
	}

	msg, err := addSong(mockFunctionCall, &songMap)
	if err != nil {
		return
	}

	fmt.Println("msg: ", msg)
	if songlist.Songs[len(songlist.Songs)-1].Title != "Song 4" {
		t.Errorf("Expected song to be added successfully, got: %s", msg)
	}
}

func TestRemoveSong(t *testing.T) {
	mockSonglist()
	mockFunctionCall := openai.ToolCall{
		Function: openai.FunctionCall{
			Name:      "remove_song",
			Arguments: `{"gid": "` + os.Getenv("test_guild_id") + `", "cid": "` + os.Getenv("test_channel_id") + `", "uuid": "d3J3uJpCgos"}`,
		},
	}

	msg, err := removeSong(mockFunctionCall, &songMap)
	if err != nil {
		return
	}

	fmt.Println("msg: ", msg)
	if msg != `Song removed successfully, song title: "d3J3uJpCgos, song url: d3J3uJpCgos` {
		t.Errorf("Expected song to be removed successfully, got: %s", msg)
	}
}

func TestGetCurrentSongList(t *testing.T) {
	mockSonglist()
	call := openai.ToolCall{
		Function: openai.FunctionCall{
			Arguments: `{"gid": "` + os.Getenv("test_guild_id") + `", "cid": "` + os.Getenv("test_channel_id") + `"}`,
		},
	}
	msg, err := getCurrentSongList(call, &songMap)
	if err != nil {
		return
	}
	if msg != `songs: [Song: Song 1, ID: d3J3uJpCgos, URL: https://www.youtube.com/watch?v=d3J3uJpCgos&list=PLwCTYY94JxbZHrJ-anoUuFkNHSFQqe438&index=6&pp=gAQBiAQB8AUB
Song: Song 2, ID: 1MvvFXBWFjI, URL: https://www.youtube.com/watch?v=1MvvFXBWFjI&list=PLwCTYY94JxbZHrJ-anoUuFkNHSFQqe438&index=2&pp=gAQBiAQB8AUB
Song: Song 3, ID: kzZ6KXDM1RI, URL: https://www.youtube.com/watch?v=kzZ6KXDM1RI&pp=ygUQeXV1cmkgZHJ5IGZsb3dlctIHCQmGCQGHKiGM7w%3D%3D
Song: Song 4, ID: 02Q4yUMw3Ds, URL: https://www.youtube.com/watch?v=02Q4yUMw3Ds&list=RDMM&start_radio=1
]` {
		t.Errorf("Expected song list to be retrieved successfully, got: %s", msg)
	}
}

func TestSkipSongWhilePlaying(t *testing.T) {
	mockSonglist()
	call := openai.ToolCall{
		Function: openai.FunctionCall{
			Arguments: `{"gid": "` + os.Getenv("test_guild_id") + `", "cid": "` + os.Getenv("test_channel_id") + `"}`,
		},
	}
	_, err := playSong(call, &songMap, myBot)
	if err != nil {
		return
	}
	time.Sleep(10 * time.Second)
	msg, err := skipSong(call, &songMap, myBot)
	if err != nil {
		return
	}
	time.Sleep(10 * time.Second)
	if msg != `Song skipped successfully, song title: Song 2, song Id: 1MvvFXBWFjI` {
		t.Errorf("Expected song to be skipped successfully, got: %s", msg)
	}
	err = songlist.StopSong()
	if err != nil {
		return
	}
}

func TestPauseSong(t *testing.T) {
	mockSonglist()
	call := openai.ToolCall{
		Function: openai.FunctionCall{
			Arguments: `{"gid": "` + os.Getenv("test_guild_id") + `", "cid": "` + os.Getenv("test_channel_id") + `"}`,
		},
	}
	_, err := playSong(call, &songMap, myBot)
	if err != nil {
		return
	}
	time.Sleep(10 * time.Second)
	msg, err := pauseSong(call, &songMap)
	if err != nil {
		return
	}
	if msg != `Song paused successfully` {
		t.Errorf("Expected song to be paused successfully, got: %s", msg)
	}
}

func TestStopSong(t *testing.T) {
	mockSonglist()
	call := openai.ToolCall{
		Function: openai.FunctionCall{
			Arguments: `{"gid": "` + os.Getenv("test_guild_id") + `", "cid": "` + os.Getenv("test_channel_id") + `"}`,
		},
	}
	_, err := playSong(call, &songMap, myBot)
	if err != nil {
		return
	}
	time.Sleep(10 * time.Second)
	msg, err := stopSong(call, &songMap)
	if err != nil {
		return
	}
	if msg != `Song stopped successfully` {
		t.Errorf("Expected song to be stopped successfully, got: %s", msg)
	}
}

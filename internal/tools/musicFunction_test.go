package tools

import (
	"fmt"
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
)

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	os.Exit(code)
}

func setup() {
	settings, err := storage.LoadSettings("settings.json")
	if err != nil {
		panic("Failed to load settings: " + err.Error())
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
}

func TestPlayMusic(t *testing.T) {
	mockSonglist()
	msg, err := playSong(&songlist, myBot)
	time.Sleep(10 * time.Second)
	if err != nil {
		return
	}
	if msg != `{"message": "Playing song successfully", "song": {"title": "Song 1", "Id": "d3J3uJpCgos"}}` {
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
			Arguments: `{"title": "Song 4", "url": "https://www.youtube.com/watch?v=cLdWfNbBMvc"}`,
		},
	}

	msg, err := addSong(mockFunctionCall, &songlist)
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
			Arguments: `{"uuid": "d3J3uJpCgos"}`,
		},
	}

	msg, err := removeSong(mockFunctionCall, &songlist)
	if err != nil {
		return
	}

	fmt.Println("msg: ", msg)
	if msg != `{"message": "Song removed successfully", "song": {"title": "d3J3uJpCgos", "url": "d3J3uJpCgos"}}` {
		t.Errorf("Expected song to be removed successfully, got: %s", msg)
	}
}

func TestGetCurrentSongList(t *testing.T) {
	mockSonglist()
	msg, err := getCurrentSongList(&songlist)
	if err != nil {
		return
	}
	fmt.Println("msg: ", msg)
	if msg != `{"songs": [Song: Song 1, ID: d3J3uJpCgos, URL: https://www.youtube.com/watch?v=d3J3uJpCgos&list=PLwCTYY94JxbZHrJ-anoUuFkNHSFQqe438&index=6&pp=gAQBiAQB8AUB
Song: Song 2, ID: 1MvvFXBWFjI, URL: https://www.youtube.com/watch?v=1MvvFXBWFjI&list=PLwCTYY94JxbZHrJ-anoUuFkNHSFQqe438&index=2&pp=gAQBiAQB8AUB
Song: Song 3, ID: kzZ6KXDM1RI, URL: https://www.youtube.com/watch?v=kzZ6KXDM1RI&pp=ygUQeXV1cmkgZHJ5IGZsb3dlctIHCQmGCQGHKiGM7w%3D%3D
Song: Song 4, ID: 02Q4yUMw3Ds, URL: https://www.youtube.com/watch?v=02Q4yUMw3Ds&list=RDMM&start_radio=1
]}` {
		t.Errorf("Expected song list to be retrieved successfully, got: %s", msg)
	}
}

func TestSkipSongWhilePlaying(t *testing.T) {
	mockSonglist()
	_, err := playSong(&songlist, myBot)
	if err != nil {
		return
	}
	time.Sleep(10 * time.Second)
	msg, err := skipSong(&songlist, myBot)
	if err != nil {
		return
	}
	time.Sleep(10 * time.Second)
	if msg != `{"message": "Song skipped successfully", "song": {"title": "Song 2", "Id": "1MvvFXBWFjI"}}` {
		t.Errorf("Expected song to be skipped successfully, got: %s", msg)
	}
	err = songlist.StopSong()
	if err != nil {
		return
	}
}

func TestPauseSong(t *testing.T) {
	mockSonglist()
	_, err := playSong(&songlist, myBot)
	if err != nil {
		return
	}
	time.Sleep(10 * time.Second)
	msg, err := pauseSong(&songlist, myBot)
	if err != nil {
		return
	}
	if msg != `{"message": "Song paused successfully"}` {
		t.Errorf("Expected song to be paused successfully, got: %s", msg)
	}
}

func TestStopSong(t *testing.T) {
	mockSonglist()
	_, err := playSong(&songlist, myBot)
	if err != nil {
		return
	}
	time.Sleep(10 * time.Second)
	msg, err := stopSong(&songlist, myBot)
	if err != nil {
		return
	}
	if msg != `{"message": "Song stopped successfully"}` {
		t.Errorf("Expected song to be stopped successfully, got: %s", msg)
	}
}

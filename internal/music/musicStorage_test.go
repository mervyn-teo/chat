package music

import (
	"fmt"
	"os"
	"sync"
	"testing"
)

var songMap map[string]map[string]*SongList

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	os.Exit(code)
}

func setup() {
	songMap = make(map[string]map[string]*SongList)
	songMap["test"] = make(map[string]*SongList)
	songMap["test"]["test"] = &SongList{
		Songs:     make([]Song, 0),
		IsPlaying: false,
		StopSig:   make(chan bool),
		Mu:        sync.Mutex{},
		Vc:        nil,
	}

	songMap["test"]["test"].Songs = append(songMap["test"]["test"].Songs, Song{
		Title: "Song 1",
		Id:    "d3J3uJpCgos",
		Url:   "https://www.youtube.com/watch?v=d3J3uJpCgos&list=PLwCTYY94JxbZHrJ-anoUuFkNHSFQqe438&index=6&pp=gAQBiAQB8AUB",
	})
}

func TestSaveSongMapToFile(t *testing.T) {
	if err := SaveSongMapToFile(&songMap); err != nil {
		t.Errorf("Failed to save song map to file: %v", err)
	}

	if _, err := os.Stat("songMap.json"); os.IsNotExist(err) {
		t.Errorf("File 'songMap.json' does not exist")
	}
}

func TestLoadSongMapFromFile(t *testing.T) {
	if err := LoadSongMapFromFile(&songMap); err != nil {
		t.Errorf("Failed to load song map from file: %v", err)
	}

	fmt.Println(songMap["test"]["test"])

	if len(songMap) == 0 {
		t.Errorf("Loaded song map is empty")
	}

	if _, ok := songMap["test"]; !ok {
		t.Errorf("Guild ID 'test' not found in loaded song map")
	}

	if _, ok := songMap["test"]["test"]; !ok {
		t.Errorf("Channel ID 'test' not found in loaded song map")
	}
}

func TestLoadSongMapFromFileNoSongMap(t *testing.T) {
	if err := os.Remove("songMap.json"); err != nil {
		t.Errorf("Failed to remove songMap.json: %v", err)
	}

	err := LoadSongMapFromFile(nil)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

package music

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
	"testing"
)

func TestDownloadSong(t *testing.T) {
	err := godotenv.Load("../../.env")

	if err := os.Remove("songCache\\d3J3uJpCgos.mp3"); err != nil {
		if !os.IsNotExist(err) {
			t.Errorf("Failed to remove test file: %v", err)
		}
	}

	fmt.Println("YOUTUBE_COOKIE: ", os.Getenv("YOUTUBE_COOKIE"))

	song, err := DownloadSong("https://www.youtube.com/watch?v=d3J3uJpCgos&list=PLwCTYY94JxbZHrJ-anoUuFkNHSFQqe438&index=6&pp=gAQBiAQB8AUB", os.Getenv("YOUTUBE_COOKIE"))
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if song == "" {
		t.Errorf("Expected song to be downloaded, got nil")
	}

	if song != "songCache\\d3J3uJpCgos.mp3" {
		t.Errorf("Expected song to be downloaded, got: %s", song)
	}
}

func TestDownloadSongError(t *testing.T) {
	song, err := DownloadSong("https://www.youtube.com/watch?v=", os.Getenv("YOUTUBE_COOKIE"))
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if song != "" {
		t.Errorf("Expected song to be empty, got: %s", song)
	}
}

func TestDownloadSongAlreadyExists(t *testing.T) {
	song, err := DownloadSong("https://www.youtube.com/watch?v=d3J3uJpCgos&list=PLwCTYY94JxbZHrJ-anoUuFkNHSFQqe438&index=6&pp=gAQBiAQB8AUB", os.Getenv("YOUTUBE_COOKIE"))
	if err != nil {
		return
	}

	if song == "" {
		t.Errorf("Expected song to be downloaded, got nil")
	}

	if song != "songCache\\d3J3uJpCgos.mp3" {
		t.Errorf("Expected song to be downloaded, got: %s", song)
	}
}

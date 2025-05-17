package music

import (
	"os"
	"testing"
)

func TestDownloadSong(t *testing.T) {
	if err := os.Remove("songCache\\d3J3uJpCgos.mp3"); err != nil {
		if !os.IsNotExist(err) {
			t.Errorf("Failed to remove test file: %v", err)
		}
	}

	song, err := DownloadSong("https://www.youtube.com/watch?v=d3J3uJpCgos&list=PLwCTYY94JxbZHrJ-anoUuFkNHSFQqe438&index=6&pp=gAQBiAQB8AUB")
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

func TestDownloadSongError(t *testing.T) {
	song, err := DownloadSong("https://www.youtube.com/watch?v=")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if song != "" {
		t.Errorf("Expected song to be empty, got: %s", song)
	}
}

func TestDownloadSongAlreadyExists(t *testing.T) {
	song, err := DownloadSong("https://www.youtube.com/watch?v=d3J3uJpCgos&list=PLwCTYY94JxbZHrJ-anoUuFkNHSFQqe438&index=6&pp=gAQBiAQB8AUB")
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

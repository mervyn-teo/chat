package music

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
	"testing"
)

func TestIsYtdlpInstalled(t *testing.T) {
	if !IsYtdlpInstalled() {
		fmt.Println("yt-dlp is not installed, please install it to run the tests.")
		os.Exit(1)
	} else {
		fmt.Println("yt-dlp is installed.")
	}
}

func TestGetPlatform(t *testing.T) {
	platform := getPlatform()
	if platform == "" {
		t.Errorf("Expected platform to be set, got empty string")
	}

	fmt.Println("Platform: ", platform)
}

func TestGetVideoInfo(t *testing.T) {
	if !IsYtdlpInstalled() {
		t.Fatal("yt-dlp is not installed, please install it to run the tests.")
	}

	video, err := getVideoInfo("https://www.youtube.com/watch?v=d3J3uJpCgos")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if video == nil {
		t.Errorf("Expected video info, got nil")
	}

	if video.ID == "" {
		t.Errorf("Expected video ID, got empty string")
	}
}

func TestYtbClientDownload(t *testing.T) {
	if !IsYtdlpInstalled() {
		t.Fatal("yt-dlp is not installed, please install it to run the tests.")
	}

	filePath, err := ytbClientDownload("./songCache", "https://www.youtube.com/watch?v=d3J3uJpCgos")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if filePath == "" {
		t.Errorf("Expected file path, got empty string")
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Expected file to exist, got: %v", err)
	}
}

func TestDownloadSong(t *testing.T) {
	err := godotenv.Load("../../.env")

	if err := os.Remove("songCache\\d3J3uJpCgos.mp3"); err != nil {
		if !os.IsNotExist(err) {
			t.Errorf("Failed to remove test file: %v", err)
		}
	}

	fmt.Println("youtube_cookie: ", os.Getenv("youtube_cookie"))

	song, err := DownloadSong("https://www.youtube.com/watch?v=d3J3uJpCgos&list=PLwCTYY94JxbZHrJ-anoUuFkNHSFQqe438&index=6&pp=gAQBiAQB8AUB", os.Getenv("youtube_cookie"))
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if song == "" {
		t.Errorf("Expected song to be downloaded, got nil")
	}
}

func TestDownloadSongError(t *testing.T) {
	song, err := DownloadSong("https://www.youtube.com/watch?v=", os.Getenv("youtube_cookie"))
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if song != "" {
		t.Errorf("Expected song to be empty, got: %s", song)
	}
}

func TestDownloadSongAlreadyExists(t *testing.T) {
	song, err := DownloadSong("https://www.youtube.com/watch?v=d3J3uJpCgos&list=PLwCTYY94JxbZHrJ-anoUuFkNHSFQqe438&index=6&pp=gAQBiAQB8AUB", os.Getenv("youtube_cookie"))
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

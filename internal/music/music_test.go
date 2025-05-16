package music

import "testing"

func TestDownloadSong(t *testing.T) {
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

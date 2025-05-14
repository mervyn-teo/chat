package tools

import (
	"encoding/json"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"log"
	"untitled/internal/bot"
	"untitled/internal/music"
)

func HandleMusicCall(call openai.ToolCall, s *music.SongList, myBot *bot.Bot) (string, error) {
	if call.Type != openai.ToolTypeFunction {
		log.Printf("Received unexpected tool type: %s", call.Type)
		return fmt.Sprintf(`{"error": "Unsupported tool type: %s"}`, call.Type), fmt.Errorf("unsupported tool type: %s", call.Type)
	}

	switch call.Function.Name {
	case "get_current_songList":
		return getCurrentSongList(s)
	case "add_song":
		return addSong(call, s)
	case "remove_song":
		return removeSong(call, s)
	case "skip_song":
		return skipSong(s, myBot)
	case "play_song":
		return playSong(s, myBot)
	case "pause_song":
		return pauseSong(s, myBot)
	case "stop_song":
		return stopSong(s, myBot)
	default:
		return "", fmt.Errorf("function %s not found", call.Function.Name)
	}
}

func stopSong(s *music.SongList, myBot *bot.Bot) (string, error) {
	if !s.IsPlaying {
		log.Printf("Error: Cannot stop song while not playing")
		return `{"error": "Cannot stop song while not playing"}`, fmt.Errorf("cannot stop song while not playing")
	}

	err := s.StopSong()
	if err != nil {
		log.Printf("Error stopping song: %v", err)
		return fmt.Sprintf(`{"error": "Failed to stop song: %v"}`, err), fmt.Errorf("failed to stop song: %w", err)
	}

	return `{"message": "Song stopped successfully"}`, nil
}

func pauseSong(s *music.SongList, myBot *bot.Bot) (string, error) {
	if !s.IsPlaying {
		log.Printf("Error: Cannot pause song while not playing")
		return `{"error": "Cannot pause song while not playing"}`, fmt.Errorf("cannot pause song while not playing")
	}

	err := s.PauseSong()
	if err != nil {
		log.Printf("Error pausing song: %v", err)
		return fmt.Sprintf(`{"error": "Failed to pause song: %v"}`, err), fmt.Errorf("failed to pause song: %w", err)
	}

	return `{"message": "Song paused successfully"}`, nil
}

func skipSong(s *music.SongList, myBot *bot.Bot) (string, error) {
	if len(s.Songs) <= 1 {
		log.Printf("Error: No songs to skip")
		return `{"error": "No songs to skip"}`, fmt.Errorf("no songs to skip")
	}

	err := s.PauseSong()
	if err != nil {
		return "", err
	}

	// skip the song
	s.Mu.Lock()
	s.Songs = s.Songs[1:]
	s.Mu.Unlock()

	err = s.PlaySong(myBot)

	if err != nil {
		log.Printf("Error playing skipped song: %v", err)
		return fmt.Sprintf(`{"error": "Failed to play skipped song: %v"}`, err), fmt.Errorf("failed to play skipped song: %w", err)
	}

	return fmt.Sprintf(`{"message": "Song skipped successfully", "song": {"title": "%s", "Id": "%s"}}`, s.Songs[0].Title, s.Songs[0].Id), nil
}

func playSong(s *music.SongList, myBot *bot.Bot) (string, error) {
	if s.IsPlaying {
		log.Printf("Error: Cannot play song while already playing")
		return `{"error": "Cannot play song while already playing"}`, fmt.Errorf("cannot play song while already playing")
	}

	err := s.PlaySong(myBot)
	if err != nil {
		log.Printf("Error playing song: %v", err)
		return fmt.Sprintf(`{"error": "Failed to play song: %v"}`, err), fmt.Errorf("failed to play song: %w", err)
	}

	return fmt.Sprintf(`{"message": "Playing song successfully", "song": {"title": "%s", "Id": "%s"}}`, s.Songs[0].Title, s.Songs[0].Id), nil
}

func removeSong(call openai.ToolCall, s *music.SongList) (string, error) {
	var args music.RemoveSongArgs
	err := json.Unmarshal([]byte(call.Function.Arguments), &args)
	if err != nil {
		log.Printf("Error parsing arguments for function '%s': %v", call.Function.Name, err)
		return fmt.Sprintf(`{"error": "Failed to parse arguments for function '%s': %v"}`, call.Function.Name, err), fmt.Errorf("argument parsing failed: %w", err)
	}

	if s.IsPlaying && args.UUID == s.Songs[0].Id {
		log.Printf("Error: Cannot remove song while playing")
		return `{"error": "Cannot remove song while playing"}`, fmt.Errorf("cannot remove song while playing")
	}

	err = s.RemoveSong(args.UUID)
	if err != nil {
		log.Printf("Error removing song: %v", err)
		return fmt.Sprintf(`{"error": "Failed to remove song: %v"}`, err), fmt.Errorf("failed to remove song: %w", err)
	}

	return fmt.Sprintf(`{"message": "Song removed successfully", "song": {"title": "%s", "url": "%s"}}`, args.UUID, args.UUID), nil
}

func addSong(call openai.ToolCall, s *music.SongList) (string, error) {
	var args music.AddSongArgs
	err := json.Unmarshal([]byte(call.Function.Arguments), &args)
	if err != nil {
		log.Printf("Error parsing arguments for function '%s': %v", call.Function.Name, err)
		return fmt.Sprintf(`{"error": "Failed to parse arguments for function '%s': %v"}`, call.Function.Name, err), fmt.Errorf("argument parsing failed: %w", err)
	}

	songAdded, err := s.AddSong(args.Title, args.Url)
	if err != nil {
		log.Printf("Error adding song: %v", err)
		return fmt.Sprintf(`{"error": "Failed to add song: %v"}`, err), fmt.Errorf("failed to add song: %w", err)
	}
	return fmt.Sprintf(`{"message": "Song added successfully", "song": {"title": "%s", "url": "%s", "uuid": "%s"}}`, args.Title, args.Url, songAdded.Id), nil
}

func getCurrentSongList(s *music.SongList) (string, error) {
	var ret string

	if len(s.Songs) == 0 {
		return `{"songs": []}`, nil
	}

	for _, song := range s.Songs {
		ret = ret + fmt.Sprintf("Song: %s, ID: %s, URL: %s\n", song.Title, song.Id, song.Url)
	}

	return fmt.Sprintf(`{"songs": [%s]}`, ret), nil
}

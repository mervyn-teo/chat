package tools

import (
	"encoding/json"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"log"
	"untitled/internal/bot"
	"untitled/internal/music"
	"untitled/internal/storage"
)

type AddSongArgs struct {
	Title string `json:"title"`
	Url   string `json:"url"`
	GID   string `json:"gid"`
	CID   string `json:"cid"`
}

type RemoveSongArgs struct {
	UUID string `json:"uuid"`
	GID  string `json:"gid"`
	CID  string `json:"cid"`
}

type VoiceDetails struct {
	GID string `json:"gid"`
	CID string `json:"cid"`
}

func HandleMusicCall(call openai.ToolCall, s *map[string]map[string]*music.SongList, myBot *bot.Bot) (string, error) {
	if call.Type != openai.ToolTypeFunction {
		log.Printf("Received unexpected tool type: %s", call.Type)
		return fmt.Sprintf(`Unsupported tool type: %s"}`, call.Type), fmt.Errorf("unsupported tool type: %s", call.Type)
	}

	switch call.Function.Name {
	case "get_current_songList":
		return getCurrentSongList(call, s)
	case "add_song":
		return addSong(call, s)
	case "remove_song":
		return removeSong(call, s)
	case "skip_song":
		return skipSong(call, s, myBot)
	case "play_song":
		return playSong(call, s, myBot)
	case "pause_song":
		return pauseSong(call, s)
	case "stop_song":
		return stopSong(call, s)
	default:
		return "", fmt.Errorf("function %s not found", call.Function.Name)
	}
}

func stopSong(call openai.ToolCall, s *map[string]map[string]*music.SongList) (string, error) {
	var args VoiceDetails

	err := json.Unmarshal([]byte(call.Function.Arguments), &args)
	if err != nil {
		log.Printf("Error parsing arguments for function '%s': %v", call.Function.Name, err)
		return fmt.Sprintf(`Failed to parse arguments for function '%s': %v`, call.Function.Name, err), fmt.Errorf("argument parsing failed: %w", err)
	}

	currSongList := (*s)[args.GID][args.CID]
	if currSongList == nil {
		log.Printf("Error: Cannot stop song, song list not found")
		return `error": "Cannot stop song, song list not found`, fmt.Errorf("song list not found")
	}

	if !currSongList.IsPlaying {
		log.Printf("Error: Cannot stop song while not playing")
		return `error": "Cannot stop song while not playing`, fmt.Errorf("cannot stop song while not playing")
	}

	err = currSongList.StopSong()

	if err != nil {
		log.Printf("Error stopping song: %v", err)
		return fmt.Sprintf(`error": "Failed to stop song: %v`, err), fmt.Errorf("failed to stop song: %w", err)
	}

	return `Song stopped successfully`, nil
}

func pauseSong(call openai.ToolCall, s *map[string]map[string]*music.SongList) (string, error) {
	var args VoiceDetails

	err := json.Unmarshal([]byte(call.Function.Arguments), &args)
	if err != nil {
		log.Printf("Error parsing arguments for function '%s': %v", call.Function.Name, err)
		return fmt.Sprintf(`error": "Failed to parse arguments for function '%s': %v`, call.Function.Name, err), fmt.Errorf("argument parsing failed: %w", err)
	}

	currSongList := (*s)[args.GID][args.CID]

	if currSongList == nil {
		log.Printf("Error: Cannot pause song, song list not found")
		return `error": "Cannot pause song, song list not found`, fmt.Errorf("song list not found")
	}

	if !currSongList.IsPlaying {
		log.Printf("Error: Cannot pause song while not playing")
		return `error": "Cannot pause song while not playing`, fmt.Errorf("cannot pause song while not playing")
	}

	err = currSongList.PauseSong()
	if err != nil {
		log.Printf("Error pausing song: %v", err)
		return fmt.Sprintf(`error": "Failed to pause song: %v`, err), fmt.Errorf("failed to pause song: %w", err)
	}

	return `Song paused successfully`, nil
}

func skipSong(call openai.ToolCall, s *map[string]map[string]*music.SongList, myBot *bot.Bot) (string, error) {
	var args VoiceDetails
	err := json.Unmarshal([]byte(call.Function.Arguments), &args)
	if err != nil {
		log.Printf("Error parsing arguments for function '%s': %v", call.Function.Name, err)
		return fmt.Sprintf(`error": "Failed to parse arguments for function '%s': %v`, call.Function.Arguments, err), fmt.Errorf("argument parsing failed: %w", err)
	}

	currSongList := (*s)[args.GID][args.CID]

	if currSongList == nil {
		log.Printf("Error: Cannot skip song, song list not found")
		return `error": "Cannot skip song, song list not found`, fmt.Errorf("song list not found")
	}

	if len(currSongList.Songs) <= 1 {
		log.Printf("Error: No songs to skip")
		return `error": "No songs to skip`, fmt.Errorf("no songs to skip")
	}

	err = currSongList.PauseSong()
	if err != nil {
		return "", err
	}

	// skip the song
	currSongList.Mu.Lock()
	currSongList.Songs = currSongList.Songs[1:]
	currSongList.Mu.Unlock()

	err = currSongList.PlaySong(args.GID, args.CID, myBot, storage.Setting.YoutubeCookies)

	if err != nil {
		log.Printf("Error playing skipped song: %v", err)
		return fmt.Sprintf(`error": "Failed to play skipped song: %v`, err), fmt.Errorf("failed to play skipped song: %w", err)
	}

	return fmt.Sprintf(`Song skipped successfully, song title: %s, song Id: %s`, currSongList.Songs[0].Title, currSongList.Songs[0].Id), nil
}

func playSong(call openai.ToolCall, s *map[string]map[string]*music.SongList, myBot *bot.Bot) (string, error) {

	var args VoiceDetails
	err := json.Unmarshal([]byte(call.Function.Arguments), &args)
	if err != nil {
		log.Printf("Error parsing arguments for function '%s': %v", call.Function.Name, err)
		return fmt.Sprintf(`Failed to parse arguments for function '%s': %v`, call.Function.Name, err), fmt.Errorf("argument parsing failed: %w", err)
	}

	currSongList := (*s)[args.GID][args.CID]

	if currSongList == nil {
		log.Printf("Error: Cannot play song, song list not found")
		return `Cannot play song, song list not found`, fmt.Errorf("song list not found")
	}

	if currSongList.IsPlaying {
		log.Printf("Error: Cannot play song while already playing")
		return `Cannot play song while already playing`, fmt.Errorf("cannot play song while already playing")
	}

	err = currSongList.PlaySong(args.GID, args.CID, myBot, storage.Setting.YoutubeCookies)

	if err != nil {
		log.Printf("Error playing song: %v", err)
		return fmt.Sprintf(`Failed to play song: %v`, err), fmt.Errorf("failed to play song: %w", err)
	}

	return fmt.Sprintf(`Playing song successfully, song title: %s, song Id: %s`, currSongList.Songs[0].Title, currSongList.Songs[0].Id), nil
}

func removeSong(call openai.ToolCall, s *map[string]map[string]*music.SongList) (string, error) {
	var args RemoveSongArgs
	err := json.Unmarshal([]byte(call.Function.Arguments), &args)
	if err != nil {
		log.Printf("Error parsing arguments for function '%s': %v", call.Function.Name, err)
		return fmt.Sprintf(`Failed to parse arguments for function '%s': %v`, call.Function.Name, err), fmt.Errorf("argument parsing failed: %w", err)
	}

	currSongList := (*s)[args.GID][args.CID]

	if currSongList == nil {
		log.Printf("Error: Cannot remove song, song list not found")
		return `Cannot remove song, song list not found`, fmt.Errorf("song list not found")
	}

	if currSongList.IsPlaying && args.UUID == currSongList.Songs[0].Id {
		log.Printf("Error: Cannot remove song while playing")
		return `Cannot remove song while playing`, fmt.Errorf("cannot remove song while playing")
	}

	err = currSongList.RemoveSong(args.UUID)
	if err != nil {
		log.Printf("Error removing song: %v", err)
		return fmt.Sprintf(`Failed to remove song: %v`, err), fmt.Errorf("failed to remove song: %w", err)
	}
	err = music.SaveSongMapToFile(s)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`Song removed successfully, song title: "%s, song url: %s`, args.UUID, args.UUID), nil
}

func addSong(call openai.ToolCall, s *map[string]map[string]*music.SongList) (string, error) {
	var args AddSongArgs
	err := json.Unmarshal([]byte(call.Function.Arguments), &args)
	if err != nil {
		log.Printf("Error parsing arguments for function '%s': %v", call.Function.Name, err)
		return fmt.Sprintf(`Failed to parse arguments for function '%s': %v`, call.Function.Name, err), fmt.Errorf("argument parsing failed: %w", err)
	}

	if *s == nil {
		fmt.Println("Song map is nil, loading from file")
		err := music.LoadSongMapFromFile(s)
		if err != nil {
			return "", err
		}
	}

	innerMap, ok := (*s)[args.GID]
	if !ok || innerMap == nil {
		// If the inner map doesn't exist or is nil, create it
		innerMap = make(map[string]*music.SongList)
		(*s)[args.GID] = innerMap // Assign the new inner map back to the outer map
	}

	currSongList := innerMap[args.CID]

	if currSongList == nil {
		// Create a new SongList and assign it directly back to the inner map
		currSongList = music.NewSongList()
		innerMap[args.CID] = currSongList // Assign to the inner map
	}

	songAdded, err := currSongList.AddSong(args.Title, args.Url)
	if err != nil {
		log.Printf("Error adding song: %v", err)
		return fmt.Sprintf(`Failed to add song: %v`, err), fmt.Errorf("failed to add song: %w", err)
	}

	err = music.SaveSongMapToFile(s)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`Song added successfully, song title: %s, url: %s, uuid: %s`, args.Title, args.Url, songAdded.Id), nil
}

func getCurrentSongList(call openai.ToolCall, s *map[string]map[string]*music.SongList) (string, error) {
	var ret string

	var args VoiceDetails
	err := json.Unmarshal([]byte(call.Function.Arguments), &args)
	if err != nil {
		log.Printf("Error parsing arguments for function '%s': %v", call.Function.Name, err)
		return fmt.Sprintf(`Failed to parse arguments for function '%s': %v`, call.Function.Name, err), fmt.Errorf("argument parsing failed: %w", err)
	}

	currSongList := (*s)[args.GID][args.CID]

	if currSongList == nil {
		log.Printf("Song list not found, creating a new one")
		(*s)[args.GID] = make(map[string]*music.SongList)
		(*s)[args.GID][args.CID] = music.NewSongList()
		currSongList = (*s)[args.GID][args.CID]
	}

	if len(currSongList.Songs) == 0 {
		return `songs: []`, nil
	}

	for _, song := range currSongList.Songs {
		ret = ret + fmt.Sprintf("Song: %s, ID: %s, URL: %s\n", song.Title, song.Id, song.Url)
	}

	return fmt.Sprintf(`songs: [%s]`, ret), nil
}

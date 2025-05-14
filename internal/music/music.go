package music

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/kkdai/youtube/v2"
	"io"
	"log"
	"os"
	"sync"
	"time"
	"untitled/internal/bot"
	"untitled/internal/storage"
)

var (
	opusSampleRate = []int{48000, 24000, 16000, 12000, 8000}
)

type SongList struct {
	Songs     []Song `json:"songs"`
	IsPlaying bool   `json:"is_playing"`
	Mu        sync.Mutex
	Vc        *discordgo.VoiceConnection
	StopSig   chan bool
}

type AddSongArgs struct {
	Title string `json:"title"`
	Url   string `json:"url"`
}

type RemoveSongArgs struct {
	UUID string `json:"uuid"`
}

type Song struct {
	Title string `json:"title"`
	Id    string `json:"id"`
	Url   string `json:"url"`
}

func DownloadSong(url string) (string, error) {

	client := youtube.Client{}
	video, err := client.GetVideo(url)

	if err != nil {
		return "", fmt.Errorf("error getting video: %w", err)
	}
	// check if song already exists
	if storage.CheckFileExistence(video.ID + ".mp3") {
		return video.ID + ".mp3", nil
	}

	// Get the best audio format
	audioFormat := video.Formats.WithAudioChannels()

	stream, _, err := client.GetStream(video, &audioFormat[0])
	if err != nil {
		panic(err)
	}

	defer func() {
		err := stream.Close()
		if err != nil {
			fmt.Printf("Error closing stream: %v", err)
		} else {
			fmt.Println("Stream closed successfully")
		}
	}()

	// Save the audio stream to a file
	filePath := fmt.Sprintf("%s.mp3", video.ID)
	outFile, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("error creating file: %w", err)
	}

	defer func() {
		err := outFile.Close()
		if err != nil {
			fmt.Printf("Error closing file: %v", err)
		} else {
			fmt.Println("File closed successfully")
		}
	}()

	_, err = io.Copy(outFile, stream)
	if err != nil {
		return "", fmt.Errorf("error copying stream to file: %w", err)
	}
	// Return the file path of the downloaded song
	return filePath, nil
}

func (s *SongList) PlaySong(bot *bot.Bot) error {
	// check if the song list is empty
	if len(s.Songs) <= 0 {
		return fmt.Errorf("no songs in the list")
	}

	currSong := s.Songs[0]
	filePath, err := DownloadSong(currSong.Url)
	if err != nil {
		return fmt.Errorf("error downloading song: %w", err)
	}

	vc, err := bot.JoinVC("439790169063686154", "710498836279459860") // TODO: get the channel ID from the message

	if err != nil {
		return fmt.Errorf("error joining voice channel: %w", err)
	}

	donePlaying := make(chan bool)
	stopper := make(chan bool)
	go PlayAudioFile(vc, filePath, stopper, donePlaying)

	s.Mu.Lock()
	s.Vc = vc
	s.IsPlaying = true
	s.Mu.Unlock()

	go func() {
		for {
			select {
			case <-s.StopSig:
				stopper <- true
				fmt.Println("Stopping song")
				return
			case <-donePlaying:
				fmt.Println("Finished playing audio")

				// check if the song list is empty
				if len(s.Songs) > 1 {
					// remove the first song from the list
					s.Songs = s.Songs[1:]

					// play the next song in the list
					nextSong := s.Songs[0]
					fmt.Println("Playing next song:", nextSong)

					// play the next song
					go func() {
						err = s.PlaySong(bot)
						if err != nil {
							fmt.Println("Error playing song:", err)
						}
						return
					}()

					return
				} else {
					fmt.Println("No more songs in the list")
					err := vc.Disconnect()

					if err != nil {
						fmt.Println("Error disconnecting:", err)
					}
					return
				}
			default:
				time.Sleep(5 * time.Second)
			}
		}
	}()

	return nil
}

func (s *SongList) AddSong(title string, url string) (*Song, error) {
	newUuid, err := uuid.NewUUID()

	if err != nil {
		fmt.Println("Error generating UUID:", err)
		return nil, errors.New("error generating UUID")
	}
	songToAdd := Song{
		Title: title,
		Id:    newUuid.String(),
		Url:   url,
	}
	s.Mu.Lock()
	s.Songs = append(s.Songs, songToAdd)
	s.Mu.Unlock()

	return &songToAdd, nil
}

func (s *SongList) RemoveSong(uuid string) error {
	s.Mu.Lock()
	for i, song := range s.Songs {
		if song.Id == uuid {
			s.Songs = append(s.Songs[:i], s.Songs[i+1:]...)
			return nil
		}
	}
	s.Mu.Unlock()

	return errors.New("song not found")
}

func (s *SongList) StopSong() error {
	if s.IsPlaying {

		s.Mu.Lock()
		s.StopSig <- true
		s.IsPlaying = false
		err := s.Vc.Disconnect()
		s.Mu.Unlock()

		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

func (s *SongList) PauseSong() error {
	s.Mu.Lock()
	if s.IsPlaying {
		vc := s.Vc
		if vc != nil {
			s.StopSig <- true
			s.IsPlaying = false
			time.Sleep(1 * time.Second) // wait for the song to stop
		} else {
			log.Printf("Error: No active voice connection")
			return fmt.Errorf("no active voice connection")
		}
	}
	s.Mu.Unlock()
	return nil
}

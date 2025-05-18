package music

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/kkdai/youtube/v2"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
	"untitled/internal/bot"
	"untitled/internal/storage"
)

type SongList struct {
	Songs     []Song                     `json:"songs"`
	IsPlaying bool                       `json:"is_playing"`
	Mu        sync.Mutex                 `json:"-"`
	Vc        *discordgo.VoiceConnection `json:"-"`
	StopSig   chan bool                  `json:"-"`
}
type Song struct {
	Title string `json:"title"`
	Id    string `json:"id"`
	Url   string `json:"url"`
}

type videoInfo struct {
	ID string `json:"id"`
}

var (
	ytdlp string = "yt-dlp"
)

func getPlatform() string {
	platform := os.Getenv("PLATFORM")
	if platform == "" {
		return "windows"
	}
	return platform
}

// checkYtdlp checks if the platform is Windows and sets the ytdlp variable accordingly.
func checkYtdlp() {
	if getPlatform() == "windows" {
		ytdlp = "yt-dlp.exe"
	}
}

func IsYtdlpInstalled() bool {
	if getPlatform() == "windows" {
		ytdlp = "yt-dlp.exe"
	} else {
		ytdlp = "yt-dlp"
	}

	_, err := exec.LookPath(ytdlp)
	if err != nil {
		log.Printf("yt-dlp not found: %v", err)
		return false
	}
	return true
}

func getVideoInfo(url string) (*videoInfo, error) {
	if !IsYtdlpInstalled() {
		return nil, errors.New("yt-dlp is not installed")
	}

	checkYtdlp()

	cmd := exec.Command(ytdlp, "--skip-download", "--dump-json", url)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error executing yt-dlp: %w", err)
	}

	var video videoInfo
	err = json.Unmarshal(output, &video)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling video info: %w", err)
	}

	return &video, nil
}

func ytbClientDownload(filepathToStore string, url string) (filePath string, err error) {
	if !IsYtdlpInstalled() {
		return "", errors.New("yt-dlp is not installed")
	}

	checkYtdlp()

	outputTemplate := filepath.Join(filepathToStore, "%(id)s.%(ext)s")

	cmd := exec.Command(ytdlp, "-x", "--audio-format", "mp3", "--audio-quality", "0", "-o", outputTemplate, url)

	id, err := youtube.ExtractVideoID(url)
	if err != nil {
		return "", err
	}

	_, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error executing yt-dlp: %w", err)
	}

	return filepath.Join(filepathToStore, id+".mp3"), nil
}

// DownloadSong downloads the song from the given URL and saves it to a file. returns the file path to the downloaded song.
func DownloadSong(url string, ytbCookie string) (filePath string, err error) {
	err = os.MkdirAll("./songCache", os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("error creating directory: %w", err)
	}

	if !IsYtdlpInstalled() {
		return "", errors.New("yt-dlp is not installed")
	}

	video, err := getVideoInfo(url)
	if err != nil {
		return "", fmt.Errorf("error getting video info: %w", err)
	}

	filePath = filepath.Join("./songCache", video.ID+".mp3")
	// check if song already exists
	if storage.CheckFileExistence(filePath) {
		return filePath, nil
	}

	downloadedFilepath, err := ytbClientDownload("./songCache", url)
	if err != nil {
		return "", err
	}

	if downloadedFilepath == "" {
		return "", fmt.Errorf("error downloading song: %w", err)
	}

	// Return the file path of the downloaded song
	return downloadedFilepath, nil
}

// PlaySong plays the audio file using the voice connection. gid and cid are the guild and channel IDs.
func (s *SongList) PlaySong(gid string, cid string, bot *bot.Bot, ytbCookie string) error {
	// check if the song list is empty
	if len(s.Songs) <= 0 {
		return fmt.Errorf("no songs in the list")
	}

	// check if the bot is already in a voice channel, different from the one we want to join
	voiceChats := bot.Session.VoiceConnections
	if len(voiceChats) > 0 && voiceChats[gid] == nil {
		return errors.New("bot already in a voice channel")
	}

	currSong := s.Songs[0]
	filePath, err := DownloadSong(currSong.Url, ytbCookie)
	if err != nil {
		return fmt.Errorf("error downloading song: %w", err)
	}

	vc, err := bot.JoinVC(gid, cid)

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
						err = s.PlaySong(gid, cid, bot, ytbCookie)
						if err != nil {
							fmt.Println("Error playing song:", err)
						}
						return
					}()

					return
				} else {
					fmt.Println("No more songs in the list")

					s.Mu.Lock()
					s.IsPlaying = false
					s.Mu.Unlock()

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

func NewSongList() *SongList {
	ret := SongList{
		Songs:     make([]Song, 0),
		IsPlaying: false,
		Mu:        sync.Mutex{},
		Vc:        nil,
		StopSig:   make(chan bool),
	}

	return &ret
}

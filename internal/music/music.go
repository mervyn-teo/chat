package music

import (
	"bytes"
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
	"runtime"
	"strings"
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
	return runtime.GOOS
}

// executeCommand is a synchronous version of exec.Command.Output().
func executeCommand(cmd *exec.Cmd) (output []byte, err error) {
	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Start()

	if err != nil {
		return nil, fmt.Errorf("error starting yt-dlp: %w", err)
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Println("error log: ", stderr.String())
		return nil, fmt.Errorf("error waiting for yt-dlp: %w", err)
	}

	output = stdout.Bytes()

	if stderr.Len() > 0 {
		err = errors.New(stderr.String())
	} else {
		err = nil
	}

	return output, err
}

// checkYtdlp checks if the platform is Windows and sets the ytdlp variable accordingly.
func checkYtdlp() {
	if getPlatform() == "windows" {
		ytdlp = "yt-dlp.exe"
	}
}

func IsYtdlpInstalled() bool {
	checkYtdlp()

	_, err := exec.LookPath(ytdlp)
	if err != nil {
		log.Printf("yt-dlp not found: %v", err)
		return false
	}
	return true
}

func getVideoInfo(url string) (*videoInfo, error) {

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Use absolute path to cookies file
	cookiesPath := filepath.Join(cwd, "cookies.txt")
	if !IsYtdlpInstalled() {
		return nil, errors.New("yt-dlp is not installed")
	}

	checkYtdlp()

	cmd := exec.Command(ytdlp, "--skip-download", "--dump-json", "--cookies", cookiesPath, url)

	output, err := executeCommand(cmd)

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
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Use absolute path to cookies file
	cookiesPath := filepath.Join(cwd, "cookies.txt")

	if !IsYtdlpInstalled() {
		return "", errors.New("yt-dlp is not installed")
	}

	checkYtdlp()

	outputTemplate := filepath.Join(filepathToStore, "%(id)s.%(ext)s")

	cmd := exec.Command(ytdlp, "-x", "--audio-format", "mp3", "--audio-quality", "0", "-o", outputTemplate, "--cookies", cookiesPath, url)

	cmdStr := fmt.Sprintf("%s %s", ytdlp, strings.Join(cmd.Args, " "))
	log.Printf("Executing command: %s", cmdStr)

	id, err := youtube.ExtractVideoID(url)
	if err != nil {
		return "", err
	}

	_, err = executeCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error executing yt-dlp: %w", err)
	}

	return filepath.Join(filepathToStore, id+".mp3"), nil
}

// DownloadSong downloads the song from the given URL and saves it to a file. returns the file path to the downloaded song.
func DownloadSong(url string) (filePath string, err error) {
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
func (s *SongList) PlaySong(gid string, cid string, myBot *bot.Bot, ytbCookie string) error {
	// Check if the song list is empty
	if len(s.Songs) <= 0 {
		return fmt.Errorf("no songs in the list")
	}

	// Check if the bot is already in this guild
	for _, vc := range myBot.Session.VoiceConnections {
		if vc.GuildID == gid && s.IsPlaying {
			return fmt.Errorf("play song failed, already in a voice channel in this guild")
		}
	}

	// Download the current song
	currSong := s.Songs[0]
	filePath, err := DownloadSong(currSong.Url)
	if err != nil {
		return fmt.Errorf("error downloading song: %w", err)
	}

	// Join the voice channel
	vc, err := myBot.JoinVC(gid, cid)
	if err != nil {
		return fmt.Errorf("error joining voice channel: %w", err)
	}

	// Set up channels for playback control
	donePlaying := make(chan bool)
	stopper := make(chan bool)

	// Update SongList state
	s.Mu.Lock()
	s.Vc = vc
	s.IsPlaying = true
	s.Mu.Unlock()

	// Start playing the audio file
	go PlayAudioFile(vc, filePath, stopper, donePlaying)

	// Monitor playback
	go s.monitorPlayback(gid, cid, myBot, ytbCookie, vc, stopper, donePlaying)

	return nil
}

// monitorPlayback handles the song completion and queue management
func (s *SongList) monitorPlayback(
	gid, cid string,
	myBot *bot.Bot,
	ytbCookie string,
	vc *discordgo.VoiceConnection,
	stopper, donePlaying chan bool,
) {
	for {
		select {
		case <-s.StopSig:
			stopper <- true
			fmt.Println("Stopping song")
			return

		case <-donePlaying:
			fmt.Println("Finished playing audio")

			if s.handleSongCompletion(gid, cid, myBot, ytbCookie, vc) {
				return
			}

		default:
			time.Sleep(5 * time.Second)
		}
	}
}

// handleSongCompletion processes the next song or cleans up if queue is empty
// Returns true if monitoring should stop, false to continue
func (s *SongList) handleSongCompletion(
	gid, cid string,
	myBot *bot.Bot,
	ytbCookie string,
	vc *discordgo.VoiceConnection,
) bool {
	// Check if there are more songs in the queue
	if len(s.Songs) > 1 {
		// Remove the first song and play the next one
		s.Mu.Lock()
		s.Songs = s.Songs[1:]
		nextSong := s.Songs[0]
		s.IsPlaying = false
		s.Mu.Unlock()

		fmt.Println("Playing next song:", nextSong)

		go func() {
			if err := s.PlaySong(gid, cid, myBot, ytbCookie); err != nil {
				fmt.Println("Error playing song:", err)
			}
		}()

		return true
	}

	// No more songs, clean up
	fmt.Println("No more songs in the list")

	s.Mu.Lock()
	s.Songs = []Song{} // clear the song list
	s.IsPlaying = false
	s.Mu.Unlock()

	if err := vc.Disconnect(); err != nil {
		fmt.Println("Error disconnecting:", err)
	}

	return true
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
			s.Mu.Unlock()
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

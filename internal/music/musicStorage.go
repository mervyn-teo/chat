package music

import (
	"encoding/json"
	"log"
	"sync"
	"untitled/internal/storage"
)

func SaveSongMapToFile(s *map[string]map[string]*SongList) error {
	if !storage.CheckFileExistence("songMap.json") {
		log.Println("Song map file does not exist. Creating a new one.")
		err := storage.CreateFile("songMap.json")

		if err != nil {
			log.Println("Error creating song map file:", err)
			return err
		}

		byteFile, err := json.MarshalIndent(s, "", "  ")

		if err != nil {
			log.Println("Error marshalling empty song map:", err)
			return err
		}

		err = storage.WriteToFile("songMap.json", byteFile)
		if err != nil {
			log.Println("Error writing empty song map to file:", err)
			return err
		}

		return nil
	}

	byteFile, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		log.Println("Error marshalling song map:", err)
		return err
	}

	err = storage.WriteToFile("songMap.json", byteFile)
	if err != nil {
		log.Println("Error writing song map to file:", err)
		return err
	}

	log.Println("Saved song map to file:", s)
	return nil
}

func LoadSongMapFromFile(s *map[string]map[string]*SongList) error {
	if !storage.CheckFileExistence("songMap.json") {
		log.Println("Song map file does not exist. Creating a new one.")
		err := storage.CreateFile("songMap.json")

		if err != nil {
			log.Println("Error creating song map file:", err)
			return err
		}
		tempSongMap := make(map[string]map[string]*SongList)
		byteFile, err := json.MarshalIndent(tempSongMap, "", "  ")

		if err != nil {
			log.Println("Error marshalling empty song map:", err)
			return err
		}

		err = storage.WriteToFile("songMap.json", byteFile)
		if err != nil {
			log.Println("Error writing empty song map to file:", err)
			return err
		}
		return nil
	}

	byteFile, err := storage.ReadFromFile("songMap.json")
	if err != nil {
		log.Println("Error reading song map file:", err)
		return err
	}

	err = json.Unmarshal(byteFile, s)
	if err != nil {
		log.Println("Error unmarshalling song map:", err)
		return err
	}

	// json cannot store channels, so we need to
	// populate the song map with missing channels
	for _, songList := range *s {
		for _, song := range songList {
			song.Mu = sync.Mutex{}
			song.IsPlaying = false // set to false every time we load
			song.StopSig = make(chan bool)
		}
	}

	log.Println("Loaded song map from file:", s)
	return nil
}

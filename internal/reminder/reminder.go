package reminder

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"log"
	"time"
	"untitled/internal/storage"
)

type Reminder struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Time        time.Time `json:"time"`
	UserID      string    `json:"user_id"`
	ChannelID   string    `json:"channel_id"`
	UUID        string    `json:"uuid"`
}

type ReminderList struct {
	Reminders []Reminder `json:"reminders"`
}

// NewReminder creates a new reminder, parsing the time from a string, the format is 2006-01-02T15:04:05
func NewReminder(title, description, reminderTime, userID, channelID string) (*Reminder, error) {
	parsedTime, err := time.Parse(time.RFC3339, reminderTime+"+08:00") // Parse the time string

	log.Println("Parsed time is: ", parsedTime)

	if err != nil {
		log.Println("Error parsing time:", err)
		return nil, err
	}

	// Check if the parsed time is in the past
	if parsedTime.Before(time.Now()) {
		log.Println("Error: Reminder time is in the past")
		return nil, errors.New("reminder time is in the past")
	}

	ret := &Reminder{
		Title:       title,
		Description: description,
		Time:        parsedTime,
		UserID:      userID,
		ChannelID:   channelID,
		UUID:        uuid.New().String(),
	}

	return ret, nil
}

func (r *ReminderList) GetReminders() (string, error) {
	if len(r.Reminders) == 0 {
		return "No reminders set.", nil
	}

	marshaled, err := json.Marshal(r.Reminders)

	if err != nil {
		return "", err
	}

	return string(marshaled), nil
}

func (r *ReminderList) AddReminder(reminder Reminder) {
	r.Reminders = append(r.Reminders, reminder)

	err := r.SaveRemindersToFile()
	if err != nil {
		log.Println("Error saving reminders to file:", err)
		return
	}

	log.Printf("Added reminder: %s", reminder.Title)
}

func (r *ReminderList) RemoveReminder(uuid string) {
	for i, reminder := range r.Reminders {
		if reminder.UUID == uuid {
			r.Reminders = append(r.Reminders[:i], r.Reminders[i+1:]...)
			log.Printf("Removed reminder: %s", reminder.Title)

			err := r.SaveRemindersToFile()
			if err != nil {
				log.Println("Error saving reminders to file:", err)
				return
			}

			return
		}
	}

	log.Printf("No reminder found with UUID: %s", uuid)
}

func (r *ReminderList) SaveRemindersToFile() error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		log.Println("Error marshalling reminders:", err)
		return err
	}

	err = storage.WriteToFile("reminders.json", data)
	if err != nil {
		log.Println("Error writing reminders to file:", err)
		return err
	}

	log.Println("Saved reminders to file:", r.Reminders)
	return nil
}

func LoadRemindersFromFile(r *ReminderList) error {
	if !storage.CheckFileExistence("reminders.json") {
		log.Println("Reminders file does not exist. Creating a new one.")
		err := storage.CreateFile("reminders.json")

		if err != nil {
			log.Println("Error creating reminders file:", err)
			return err
		}
		tempReminder := ReminderList{}
		byteFile, err := json.Marshal(tempReminder)

		if err != nil {
			log.Println("Error marshalling empty reminders:", err)
			return err
		}

		err = storage.WriteToFile("reminders.json", byteFile)
		if err != nil {
			log.Println("Error writing empty reminders to file:", err)
			return err
		}

		return nil
	}

	byteFile, err := storage.ReadFromFile("reminders.json")
	if err != nil {
		log.Println("Error reading reminders file:", err)
		return err
	}

	err = json.Unmarshal(byteFile, r)
	if err != nil {
		log.Println("Error unmarshalling reminders:", err)
		return err
	}

	log.Println("Loaded reminders from file:", r.Reminders)
	return nil
}

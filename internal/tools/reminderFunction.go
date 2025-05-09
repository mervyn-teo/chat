package tools

import (
	"encoding/json"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"log"
	"time"
	"untitled/internal/bot"
	"untitled/internal/reminder"
)

type newReminderArgs struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Time        string `json:"time"` // Time in 2006-01-02T15:04:05 format
	UserID      string `json:"user_id"`
	ChannelID   string `json:"channel_id"`
}

// HandleReminderCall processes the tool call for reminders
func HandleReminderCall(call openai.ToolCall, i *reminder.ReminderList, bot *bot.Bot) (string, error) {
	if call.Type != openai.ToolTypeFunction {
		log.Printf("Received unexpected tool type: %s", call.Type)
		return fmt.Sprintf(`{"error": "Unsupported tool type: %s"}`, call.Type), fmt.Errorf("unsupported tool type: %s", call.Type)
	}

	switch call.Function.Name {
	case "create_reminder":
		log.Printf("Received call for create_reminder. Parsing arguments.")
		var args newReminderArgs
		err := json.Unmarshal([]byte(call.Function.Arguments), &args)

		if err != nil {
			log.Printf("Error parsing arguments for function '%s': %v", call.Function.Name, err)
			return fmt.Sprintf(`{"error": "Failed to parse arguments for function '%s': %v"}`, call.Function.Name, err), fmt.Errorf("argument parsing failed: %w", err)
		}

		newReminder, err := reminder.NewReminder(args.Title, args.Description, args.Time, args.UserID, args.ChannelID)
		if err != nil {
			return "", err
		}

		log.Printf("Reminder created: %s, time: %s\n", newReminder.Title, newReminder.Time)
		i.AddReminder(*newReminder)

		// Schedule the reminder
		go func() {
			time.Sleep(time.Until(newReminder.Time))
			// Send the reminder message to the user
			fmt.Println("Sending reminder message...")
			bot.SendMessageToChannel(newReminder.ChannelID, fmt.Sprintf("<@%s> \n Reminder: %s - %s", newReminder.UserID, newReminder.Title, newReminder.Description))
		}()

		return fmt.Sprintf(`Reminder created, UUID: %s`, newReminder.UUID), nil
	case "list_reminders":
		log.Printf("Received call for list_reminders. No arguments expected/parsed.")
		result, funcErr := i.GetReminders()
		if funcErr != nil {
			log.Printf("Error executing function '%s': %v", call.Function.Name, funcErr)
			return fmt.Sprintf(`{"error": "Execution of function '%s' failed: %v"}`, call.Function.Name, funcErr), fmt.Errorf("function execution failed: %w", funcErr)
		}

		return result, nil
	default:
		log.Printf("Received call for unknown function: %s", call.Function.Name)
		return fmt.Sprintf(`{"error": "Unknown function: %s"}`, call.Function.Name), fmt.Errorf("unknown function: %s", call.Function.Name)
	}
}

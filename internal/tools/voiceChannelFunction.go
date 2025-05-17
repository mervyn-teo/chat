package tools

import (
	"encoding/json"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"log"
	"untitled/internal/bot"
	"untitled/internal/voiceChatUtils"
)

type voiceChannelArgs struct {
	GID    string `json:"gid"`
	UserID string `json:"userid"`
}

func HandleVoiceChannel(call openai.ToolCall, myBot *bot.Bot) (string, error) {
	if call.Type != openai.ToolTypeFunction {
		log.Printf("Received unexpected tool type: %s", call.Type)
		return fmt.Sprintf(`Unsupported tool type: %s"}`, call.Type), fmt.Errorf("unsupported tool type: %s", call.Type)
	}

	switch call.Function.Name {
	case "find_voice_channel":
		var args voiceChannelArgs
		err := json.Unmarshal([]byte(call.Function.Arguments), &args)
		if err != nil {
			log.Printf("Error parsing arguments for function '%s': %v", call.Function.Name, err)
			return fmt.Sprintf(`error: Failed to parse arguments for function '%s': %v`, call.Function.Name, err), fmt.Errorf("argument parsing failed: %w", err)
		}

		channel, err := voiceChatUtils.FindVoiceChannel(myBot.Session, args.GID, args.UserID)
		if err != nil {
			return "", err
		}

		return channel, nil
	default:
		return "", fmt.Errorf("function %s not found", call.Function.Name)
	}

}

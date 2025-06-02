package voiceChatUtils

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
)

// GetVoiceChannel retrieves the list of voice channel ID for a given guild ID.
func GetVoiceChannel(s *discordgo.Session, guildID string) (voiceChatIds []string, err error) {
	channels, err := s.GuildChannels(guildID)

	if err != nil {
		return nil, err
	}

	var voiceChannelIDs []string
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildVoice {
			voiceChannelIDs = append(voiceChannelIDs, channel.ID)
		}
	}

	return voiceChannelIDs, nil
}

func CheckJoinPermission(s *discordgo.Session, guildID, channelID string) (allowed bool, err error) {
	botUser, err := s.User("@me")

	if err != nil {
		return false, err
	}

	channel, err := s.Channel(channelID)

	if err != nil {
		return false, err
	}

	permissions, err := s.State.UserChannelPermissions(botUser.ID, channel.ID)
	if err != nil {
		return false, fmt.Errorf("error calculating bot permissions: %w", err)
	}

	// 4. Check the PermissionConnect Flag
	if permissions&discordgo.PermissionVoiceConnect != 0 {
		return true, nil // Bot has PermissionConnect
	}

	return false, nil
}

// CheckVoicePermission checks if the bot has permission to speak in a voice channel.
func CheckVoicePermission(s *discordgo.Session, guildID, channelID string) (allowed bool, err error) {
	botUser, err := s.User("@me")

	if err != nil {
		return false, err
	}

	channel, err := s.Channel(channelID)

	if err != nil {
		return false, err
	}

	permissions, err := s.State.UserChannelPermissions(botUser.ID, channel.ID)
	if err != nil {
		return false, fmt.Errorf("error calculating bot permissions: %w", err)
	}

	// 4. Check the PermissionConnect Flag
	if permissions&discordgo.PermissionVoiceSpeak != 0 {
		return true, nil // Bot has PermissionConnect
	}

	return false, nil
}

// CheckUserVoiceChannel checks user's voice channel. returns the channelID if user is in a voice channel
func CheckUserVoiceChannel(s *discordgo.Session, guildID, userID string) (isInVoiceChannel bool, channelID string, err error) {
	guild, err := s.State.Guild(guildID)
	if err != nil {
		return false, "", err
	}

	for _, channel := range guild.VoiceStates {
		if channel.UserID == userID {
			return true, channel.ChannelID, nil // User is in a voice channel
		}
	}

	return false, "", nil
}

func CheckMusicPerm(s *discordgo.Session, guildID, channelID string) (allowed bool, err error) {
	allowedJoin, err := CheckJoinPermission(s, guildID, channelID)
	if err != nil {
		return false, err
	}

	allowedSpeak, err := CheckVoicePermission(s, guildID, channelID)
	if err != nil {
		return false, err
	}

	return allowedJoin && allowedSpeak, nil
}

// FindVoiceChannel gets the appropriate voice channel for bot to join, returns the channelID if found.
func FindVoiceChannel(s *discordgo.Session, guildID, userID string) (channelID string, err error) {
	isInVoice, channelID, err := CheckUserVoiceChannel(s, guildID, userID)
	if err != nil {
		return "", err
	}

	if isInVoice {
		allowed, err := CheckMusicPerm(s, guildID, channelID)
		if err != nil {
			return "", err
		}

		// Can join user's voice channel
		if allowed {
			return channelID, nil
		}
	}

	channels, err := GetVoiceChannel(s, guildID)

	for _, channel := range channels {
		allowed, err := CheckMusicPerm(s, guildID, channel)
		if err != nil {
			return "", err
		}

		if allowed {
			return channel, nil
		}
	}

	return "", errors.New("no voice channel found")
}

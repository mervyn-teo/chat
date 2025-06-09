![workflow status](https://github.com/mervyn-teo/chat/actions/workflows/go.yml/badge.svg)

# About
Discord chatbot with AI integration using OpenRouter API.

This bot is designed to provide a seamless chat experience by leveraging the power of AI to generate responses to user queries.
This bot can execute functions that users have declared.  

# Prerequisites
## Essential
- Go 1.20 or later
- A Discord app token (bot token)
- OpenRouter API key

## Optional
- News API key (for fetching current news)
- YouTube API key (for fetching YouTube videos and playing music)
- [FFmpeg](https://ffmpeg.org/) (for playing music in voice channels)
- [Yt-dlp](https://github.com/yt-dlp/yt-dlp) (for downloading YouTube videos to play in voice channels)
- [YouTube cookies](https://www.reddit.com/r/youtubedl/wiki/cookies/) (for downloading YouTube videos that require age verification or are region-locked, if you plan to run this bot on AWS or any other mainstream cloud provider, you will need to use a cookie to access YouTube videos that require age verification or are region-locked)

# Installation
**1. Clone the repository**
```bash
$ git clone https://github.com/mervyn-teo/chat
```

**2. Change directory to the cloned repository**
```bash
$ cd chat
```

**3. build with go**
```bash
$ go build -o chat ./cmd/api
```

**4. Set up environment variables**
- rename `settings.json.example` to `settings.json`
- fill in the json with your own API keys
  - you can get your openRouter API key from [here](https://openrouter.ai/settings/keys)
  - you can get your Discord bot token from [here](https://discord.com/developers/applications)
  - you can get your news API token from [here](https://newsapi.org/)
  - follow this page to get your [Youtube API key](https://developers.google.com/youtube/v3/getting-started)

**5. Run the bot**
```bash
$ ./chat
```

**6. Invite the bot to your server**
- Go to the [Discord Developer Portal](https://discord.com/developers/applications)
- Select your application
- Go to the "OAuth2" tab
- Under "Scopes", select "bot"
- Under "Bot Permissions", select the permissions you want to grant the bot
  - Selected permissions:
    - Read Messages
    - Send Messages
- Under "Bot" tab, select "Privileged Gateway Intents" and enable "Message Content Intent"
- Copy the generated URL and paste it into your browser
- Select the server you want to invite the bot to and click "Authorize"

# Personalization
You can customize the bot's behavior by modifying the `settings.json` file.
- `instructions`: The instructions that will be sent to the AI model to set it up. This is the prompt that will be used as a basis to generate responses.

# List of commands
- `!ping` - the bot will respond with a `pong` and tell you the latency
- `@[bot] [text]` - initiate a chat with the bot, replying to the bot works too.
- `!forget` - clears the memory of current user's chat with the bot

# Abilities
You can ask the chatbot to do the following for you:
- Scheduling reminders
- Get current news
- Find YouTube videos
- Play music from YouTube in the voice channel (requires the bot to have permission to join and speak in the voice channel)
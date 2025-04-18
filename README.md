![workflow status](https://github.com/mervyn-teo/chat/actions/workflows/go.yml/badge.svg)

# About
Discord chatbot with AI integration using OpenRouter API.

This bot is designed to provide a seamless chat experience by leveraging the power of AI to generate responses to user queries.

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
  - you can get your news API token from [here](https://currentsapi.services/)

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
- Copy the generated URL and paste it into your browser
- Select the server you want to invite the bot to and click "Authorize"

# Personalization
You can customize the bot's behavior by modifying the `settings.json` file.
- `instructions`: The instructions that will be sent to the AI model to set it up. This is the prompt that will be used as a basis to generate responses.

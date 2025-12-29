# LinkedIn Ghostwriter Bot

A Slack bot that helps you generate LinkedIn posts from your thoughts and completed Linear issues. It uses AI to categorize ideas, generate content, and schedule posts for you.

## What it does

- Listens to messages in Slack and saves them as "thoughts"
- Connects to Linear and creates thoughts when you complete issues
- Uses AI to categorize and generate LinkedIn post content
- Lets you schedule posts and get approval through Slack reactions

## Getting Started

### 1. Set up PostgreSQL with Docker

Run this command to start a PostgreSQL database:

```bash
docker run --name linkedin-ghostwriter-db -e POSTGRES_PASSWORD=yourpassword -e POSTGRES_DB=ghostwriter -p 5432:5432 -d postgres:latest
```

Replace `yourpassword` with a secure password. The database will be available at `localhost:5432`.

### 2. Create a Slack App

1. Go to https://api.slack.com/apps
2. Click "Create New App" → "From scratch"
3. Give it a name like "LinkedIn Ghostwriter" and select your workspace
4. Go to "OAuth & Permissions" in the sidebar
5. Add these Bot Token Scopes:
   - `app_mentions:read`
   - `channels:history`
   - `chat:write`
   - `reactions:read`
   - `users:read`
6. Scroll up and click "Install to Workspace"
7. Copy the "Bot User OAuth Token" (starts with `xoxb-`)
8. Go to "Basic Information" → "App Credentials"
9. Copy the "Signing Secret"

### 3. Set up Linear API Key (Optional)

1. Go to https://linear.app/settings/api
2. Create a new Personal API Key
3. Copy the API key

### 4. Get an Anthropic API Key

1. Go to https://console.anthropic.com/
2. Sign up or log in
3. Go to API Keys section
4. Create a new API key and copy it

### 5. Configure Environment Variables

Create a `.env` file in the project root:

```env
DATABASE_URL=postgres://postgres:yourpassword@localhost:5432/ghostwriter?sslmode=disable
SLACK_BOT_TOKEN=xoxb-your-bot-token-here
SLACK_SIGNING_SECRET=your-signing-secret-here
LINEAR_API_KEY=your-linear-api-key-here
ANTHROPIC_API_KEY=your-anthropic-api-key-here
```

Replace the values with your actual credentials.

### 6. Run the Bot

```bash
go run cmd/bot/main.go
```

The bot will start on port 3000. Make sure to configure your Slack app's Event Subscriptions to point to your server URL (you'll need to expose it publicly, like with ngrok for local development).

## Slack Commands

Once the bot is running, you can use these commands in Slack by mentioning the bot:

- `@LinkedIn Ghostwriter generate` - Generate LinkedIn post drafts from your recent thoughts
- `@LinkedIn Ghostwriter generate [topic]` - Generate posts from thoughts in a specific category
- `@LinkedIn Ghostwriter brainstorm [topic]` - Brainstorm ideas on a topic
- `@LinkedIn Ghostwriter drafts` - View all pending draft posts
- `@LinkedIn Ghostwriter schedule [1-4]` - Schedule approved posts (1-4 posts per day)
- `@LinkedIn Ghostwriter view schedule` - See your upcoming scheduled posts
- `@LinkedIn Ghostwriter stats` - Show statistics about your thoughts
- `@LinkedIn Ghostwriter help` - Show help message
- `@LinkedIn Ghostwriter sync linear` - Sync completed Linear issues as thoughts

**Workflow:**
1. Just send regular messages in Slack - they'll be saved as thoughts automatically
2. Generate posts: `@LinkedIn Ghostwriter generate`
3. React with 1️⃣, 2️⃣, 3️⃣, or ✅ to approve drafts
4. Schedule approved posts: `@LinkedIn Ghostwriter schedule 2` (for 2 posts per day)
5. Posts will be published automatically at scheduled times!

## Notes

- The Linear integration is optional - if you don't provide `LINEAR_API_KEY`, the bot will work fine without it
- Make sure your PostgreSQL container is running before starting the bot
- The bot creates all necessary database tables automatically on startup


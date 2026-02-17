# Google OAuth Authentication with Gin

A Golang web application using Gin framework with Google OAuth 2.0 authentication.

## Features

- Google OAuth 2.0 authentication flow
- Session management with secure cookies
- User profile retrieval
- CSRF protection with state tokens
- RESTful API endpoints

## Prerequisites

- Go 1.23 or higher
- Google Cloud Console project with OAuth 2.0 credentials

## Setup

### 1. Google Cloud Console Setup

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable Google+ API
4. Go to "Credentials" → "Create Credentials" → "OAuth client ID"
5. Configure OAuth consent screen if prompted
6. Create OAuth 2.0 Client ID:
   - Application type: Web application
   - Authorized redirect URIs: `http://localhost:8080/auth/google/callback`
   - Copy the Client ID and Client Secret

### 2. Environment Configuration

1. Copy `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```

2. Update `.env` with your credentials:
   ```
   GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
   GOOGLE_CLIENT_SECRET=your-client-secret
   SESSION_SECRET=your-random-secret-key
   ```

### 3. Install Dependencies

```bash
go mod download
```

### 4. Run the Application

```bash
go run cmd/main.go
```

The server will start on `http://localhost:8080`

## API Endpoints

- `GET /` - API information and available endpoints
- `GET /auth/google` - Initiate Google OAuth login
- `GET /auth/google/callback` - OAuth callback handler
- `GET /auth/profile` - Get current user profile (requires authentication)
- `GET /auth/logout` - Logout and clear session

## Usage Example

1. Start the server
2. Navigate to `http://localhost:8080/auth/google`
3. You'll be redirected to Google's consent screen
4. After authentication, you'll be redirected to your frontend URL
5. Use `/auth/profile` to get user information
6. Use `/auth/logout` to clear the session

## Docker

Build and run with Docker:

```bash
docker build -t googleauth .
docker run -p 8080:8080 --env-file .env googleauth
```

## Project Structure

```
.
├── cmd/
│   └── main.go          # Application entry point
├── config/
│   └── config.go        # Configuration management
├── handlers/
│   └── auth.go          # OAuth handlers
├── .env.example         # Environment variables template
├── Dockerfile           # Docker configuration
└── go.mod              # Go dependencies
```

## Security Notes

- Always use HTTPS in production
- Set `ENVIRONMENT=production` in production
- Use a strong `SESSION_SECRET` (at least 32 random characters)
- Configure proper CORS settings for your frontend domain
- Update `GOOGLE_REDIRECT_URL` to match your production domain

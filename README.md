# Whisper Receiver (Go)

Go-based microservice that accepts audio files and transcribes them using a remote go-whisper server.

## Architecture

```
Audio Upload → whisper-receiver → go-whisper server → webhook
```

- **whisper-receiver**: HTTP server accepting audio uploads (this project)
- **go-whisper**: Remote transcription service running the Whisper model
- **webhook**: Destination URL for transcription results

## Quick Start

### 1. Configure Environment

Copy `.env.example` to `.env` and set your webhook URL:

```bash
cp .env.example .env
# Edit .env with your WEBHOOK_URL
```

### 2. Start Services

```bash
docker-compose up -d
```

This starts:
- `whisper-server` - go-whisper API server on port 8081
- `whisper-receiver` - this service on port 8000

### 3. Download Model

The model needs to be downloaded once. After containers are running:

```bash
docker exec whisper-server gowhisper download-model ggml-medium-q5_0
```

Or add this to the docker-compose to auto-download on startup.

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/transcribe/` | POST | Accept audio file (multipart/form-data) |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `WEBHOOK_URL` | Yes | - | Destination URL for transcription results |
| `WHISPER_URL` | No | http://localhost:8081 | URL of go-whisper server |
| `WHISPER_MODEL` | No | ggml-medium-q5_0 | Whisper model to use |
| `PORT` | No | 8000 | Port for this service |

## Webhook Payload

The webhook receives a JSON payload:

```json
{
  "transcription": "The transcribed text..."
}
```

## Usage

```bash
# Upload audio for transcription
curl -X POST http://localhost:8000/transcribe/ \
  -F "file=@audio.m4a"

# Check health
curl http://localhost:8000/health
```

## Development

```bash
# Build locally
go build -o whisper-receiver .

# Run locally (requires go-whisper running separately)
go run . -whisper-url http://localhost:8081
```
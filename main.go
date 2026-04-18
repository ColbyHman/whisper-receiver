package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	webhookURL = os.Getenv("WEBHOOK_URL")
	port       = getEnv("PORT", "8000")
	whisperURL = getEnv("WHISPER_URL", "http://localhost:8081")
	modelID    = getEnv("WHISPER_MODEL", "ggml-medium-q5_0")
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	if webhookURL == "" {
		log.Fatal("WEBHOOK_URL environment variable is required")
	}

	log.Printf("Whisper receiver starting...")
	log.Printf("Whisper server URL: %s", whisperURL)
	log.Printf("Model: %s", modelID)

	router := setupRouter()

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		log.Printf("Starting server on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func setupRouter() *gin.Engine {
	router := gin.Default()

	router.Use(func(c *gin.Context) {
		log.Printf("%s %s %s", c.Request.Method, c.Request.URL.Path, c.ContentType())
		c.Next()
	})

	router.GET("/health", healthCheck)
	router.POST("/transcribe/", transcribeAudioHandler)

	return router
}

func healthCheck(c *gin.Context) {
	whisperHealthy := checkWhisperServer()
	c.JSON(http.StatusOK, gin.H{
		"status":            "healthy",
		"service":           "audio-upload-service",
		"whisper_connected": whisperHealthy,
	})
}

func checkWhisperServer() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", whisperURL+"/api/whisper/models", nil)
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func transcribeAudioHandler(c *gin.Context) {
	contentType := c.ContentType()
	log.Printf("Content-Type: %s", contentType)

	var file *os.File
	var filename string

	if strings.HasPrefix(contentType, "audio/") || contentType == "application/octet-stream" {
		// Direct audio upload - read from body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("Failed to read body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read audio data"})
			return
		}

		// Create temp directory if it doesn't exist
		if err := os.MkdirAll("/tmp", 0755); err != nil {
			log.Printf("Failed to create /tmp directory: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp directory"})
			return
		}

		tmpFile, err := os.CreateTemp("", "whisper-*.m4a")
		if err != nil {
			log.Printf("Failed to create temp file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file"})
			return
		}
		defer tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.Write(body); err != nil {
			log.Printf("Failed to write audio to temp file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save audio"})
			return
		}
		tmpFile.Close()

		// Extract extension from content-type
		ext := ".m4a"
		if contentType == "audio/mpeg" || contentType == "audio/mp3" {
			ext = ".mp3"
		} else if contentType == "audio/wav" || contentType == "audio/x-wav" {
			ext = ".wav"
		} else if contentType == "audio/ogg" {
			ext = ".ogg"
		}

		filename = "audio" + ext
		file = tmpFile

		log.Printf("Received direct audio upload: %s, size: %d bytes", filename, len(body))
	} else {
		// Multipart form upload
		uploadedFile, err := c.FormFile("file")
		if err != nil {
			log.Printf("FormFile error: %v", err)
			log.Printf("Content-Type was: %s", contentType)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get file from request", "received_type": contentType})
			return
		}

		src, err := uploadedFile.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file"})
			return
		}
		defer src.Close()

		tmpFile, err := os.CreateTemp("", "whisper-*.m4a")
		if err != nil {
			// Try creating temp directory first
			if mkerr := os.MkdirAll("/tmp", 0755); mkerr != nil {
				log.Printf("Failed to create /tmp directory: %v", mkerr)
			}
			tmpFile, err = os.CreateTemp("", "whisper-*.m4a")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file"})
				return
			}
		}
		defer tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		if _, err := io.Copy(tmpFile, src); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
			return
		}
		tmpFile.Close()

		filename = uploadedFile.Filename
		file = tmpFile
	}

	go processTranscription(file.Name(), filename)

	c.JSON(http.StatusCreated, gin.H{"detail": "File received"})
}

func processTranscription(filePath, filename string) {
	log.Printf("Starting background transcription for %s\n", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Failed to open temp file: %v\n", err)
		return
	}
	defer file.Close()

	transcription, err := transcribeWithWhisper(file, filename)
	if err != nil {
		log.Printf("Transcription error: %v\n", err)
		return
	}

	log.Println("Transcription complete, sending to webhook...")

	if err := sendToWebhook(transcription); err != nil {
		log.Printf("Failed to send to webhook: %v\n", err)
	} else {
		log.Println("Transcription completed successfully")
	}

	os.Remove(filePath)
}

func transcribeWithWhisper(file *os.File, filename string) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("audio", filename)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("failed to copy file to form: %w", err)
	}

	if err := writer.WriteField("model", modelID); err != nil {
		return "", fmt.Errorf("failed to write model field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	log.Printf("Sending transcription request to %s/api/whisper/transcribe\n", whisperURL)

	client := &http.Client{Timeout: 300 * time.Second}
	req, err := http.NewRequest("POST", whisperURL+"/api/whisper/transcribe", &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper server returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Text string `json:"text"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Text, nil
}

func sendToWebhook(text string) error {
	payload := map[string]string{"transcription": text}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	log.Printf("Sending transcription to %s\n", webhookURL)

	log.Printf("Payload: %s\n", string(body))

	// client := &http.Client{Timeout: 30 * time.Second}
	// resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(body))
	// if err != nil {
	// 	log.Printf("Webhook POST failed: %v\n", err)
	// 	return err
	// }
	// defer resp.Body.Close()

	// if resp.StatusCode >= 400 {
	// 	return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	// }

	log.Println("Successfully sent transcription to webhook")
	return nil
}

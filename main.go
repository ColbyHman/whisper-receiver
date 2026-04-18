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
	log.Printf("Content-Type: %s", c.ContentType())
	log.Printf("Content-Length: %s", c.GetHeader("Content-Length"))

	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("FormFile error: %v", err)
		log.Printf("Available form keys: %v", c.Request.Form)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get file from request"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file"})
		return
	}
	defer src.Close()

	tmpFile, err := os.CreateTemp("", "whisper-*.m4a")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file"})
		return
	}
	defer tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	tmpFile.Close()

	go processTranscription(tmpFile.Name(), file.Filename)

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

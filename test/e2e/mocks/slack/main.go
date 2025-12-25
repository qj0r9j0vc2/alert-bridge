package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "9091"
	}

	// Create mock handler
	handler := NewMockSlackHandler()

	// Setup HTTP server
	server := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down mock Slack server...")
		if err := server.Close(); err != nil {
			log.Printf("Error closing server: %v", err)
		}
	}()

	// Start server
	log.Printf("Mock Slack API server listening on port %s", port)
	log.Println("Endpoints:")
	log.Println("  POST   /api/chat.postMessage  - Post a message")
	log.Println("  POST   /api/chat.update        - Update a message")
	log.Println("  GET    /api/test/messages      - Query messages (test helper)")
	log.Println("  POST   /api/test/reset         - Reset mock state (test helper)")
	log.Println("  GET    /health                 - Health check")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	fmt.Println("Server stopped")
}

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
		port = "9092"
	}

	// Create mock handler
	handler := NewMockPagerDutyHandler()

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

		log.Println("Shutting down mock PagerDuty server...")
		if err := server.Close(); err != nil {
			log.Printf("Error closing server: %v", err)
		}
	}()

	// Start server
	log.Printf("Mock PagerDuty Events API v2 server listening on port %s", port)
	log.Println("Endpoints:")
	log.Println("  POST   /v2/enqueue            - Send an event")
	log.Println("  GET    /api/test/events       - Query events (test helper)")
	log.Println("  POST   /api/test/reset        - Reset mock state (test helper)")
	log.Println("  GET    /health                - Health check")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	fmt.Println("Server stopped")
}

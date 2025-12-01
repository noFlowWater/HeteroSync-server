package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"time-sync-server/config"
	"time-sync-server/internal/api"
	"time-sync-server/internal/repository"
	"time-sync-server/internal/service"
	"time-sync-server/internal/websocket"
)

func main() {
	// Load configuration
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.Printf("Starting Time Sync Server on port %s", cfg.ServerPort)
	log.Printf("Database path: %s", cfg.DBPath)

	// Initialize SQLite repository
	repo, err := repository.NewSQLiteRepository(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}
	defer repo.Close()

	// Initialize WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Initialize service
	syncService := service.NewSyncService(hub, repo)

	// Initialize AutoSyncMonitor
	autoSyncMonitor := service.NewAutoSyncMonitor(syncService)

	// Initialize Pairing Operator (for automatic pairing restoration)
	pairingOperator := service.NewPairingOperator(hub, repo, autoSyncMonitor)
	hub.SetPairingOperator(pairingOperator)
	log.Println("Pairing Operator initialized - automatic pairing restoration enabled")

	// Initialize HTTP handler
	handler := api.NewHandler(syncService, autoSyncMonitor, hub, cfg, repo)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Setup routes
	api.SetupRoutes(router, handler)

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server is running on http://localhost:%s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Shutdown auto-sync monitor
	autoSyncMonitor.Shutdown()

	// Gracefully shutdown the server with a timeout of 5 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

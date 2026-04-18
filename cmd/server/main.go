package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lusiker/filetransmitter/internal/config"
	"github.com/lusiker/filetransmitter/internal/handler"
	"github.com/lusiker/filetransmitter/internal/service"
	"github.com/lusiker/filetransmitter/internal/ws"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("FileTransmitter %s (built: %s)", Version, BuildTime)
	log.Printf("Device: %s (%s)", cfg.Device.Name, cfg.Device.ID)

	// Ensure temp directory exists
	if err := os.MkdirAll(cfg.Transfer.TempDir, 0755); err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}

	// Initialize services
	hub := ws.NewHub()
	go hub.Run()

	sessionService := service.NewSessionService(hub)
	cleanupService := service.NewCleanupService(cfg.Transfer.TempDir)
	transferService := service.NewTransferService(sessionService, cleanupService, &cfg.Transfer, hub)

	// Initialize RelayService for streaming transfer
	relayService := service.NewRelayService(hub, sessionService, &cfg.Transfer)
	transferService.SetRelayService(relayService)
	sessionService.SetRelayService(relayService)

	// Client device service for browser clients
	clientDeviceService := service.NewClientDeviceService(hub)

	// Set up Hub callbacks for device registration
	hub.SetOnDeviceRegister(func(deviceID, clientID string) {
		// We'll handle registration via WebSocket message from client
		// This callback just logs the connection
		log.Printf("[Main] Device connected via WebSocket: %s", deviceID)
	})

	hub.SetOnDeviceUnregister(func(deviceID string) {
		// Cancel any pending sessions from this sender
		clientDeviceService.CancelPendingSessions(deviceID, sessionService)
		clientDeviceService.Unregister(deviceID)
	})

	// Initialize handlers
	deviceHandler := handler.NewDeviceHandler(clientDeviceService, hub, relayService)
	sessionHandler := handler.NewSessionHandler(sessionService)
	transferHandler := handler.NewTransferHandler(transferService, sessionService)
	adminHandler := handler.NewAdminHandler(clientDeviceService, sessionService, transferService, relayService, hub, &cfg.Transfer)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Increase multipart form limits for large file uploads
	r.MaxMultipartMemory = 64 << 20 // 64 MB (adjust as needed)

	// CORS middleware for browser clients (直连后端需要完整CORS配置)
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		} else {
			c.Header("Access-Control-Allow-Origin", "*")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS, PUT")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept, Origin, Cache-Control, X-File-Id, X-Session-Id")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, Content-Disposition")
		c.Header("Access-Control-Max-Age", "86400") // 24 hours
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Health check under API group
	api := r.Group("/api/v1")
	api.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"version": Version,
			"device":  cfg.Device.Name,
		})
	})

	handler.RegisterDeviceRoutes(api, deviceHandler)
	handler.RegisterSessionRoutes(api, sessionHandler)
	handler.RegisterTransferRoutes(api, transferHandler)
	handler.RegisterAdminRoutes(api, adminHandler)

	// Static files (frontend) - for production build
	r.Static("/assets", "./web/dist/assets")
	r.NoRoute(func(c *gin.Context) {
		c.File("./web/dist/index.html")
	})

	// Start HTTP server with timeouts for large file uploads
	go func() {
		addr := ":" + strconv.Itoa(cfg.Server.HTTPPort)
		log.Printf("HTTP server starting on %s", addr)

		// Use http.Server with custom timeouts
		server := &http.Server{
			Addr:              addr,
			Handler:           r,
			ReadHeaderTimeout: 30 * time.Second, // Timeout for reading request headers
			WriteTimeout:      0,                 // No write timeout (large downloads may take time)
			ReadTimeout:       0,                 // No read timeout (large uploads may take time)
			MaxHeaderBytes:    1 << 20,           // 1MB max header size
		}

		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	log.Println("FileTransmitter started successfully")
	log.Println("Open http://localhost:8080 in browser to use")

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	cleanupService.CleanupAll()
	log.Println("FileTransmitter stopped")
}
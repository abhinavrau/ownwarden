package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ownwarden/funnel-access/backend/internal/api"
	"github.com/ownwarden/funnel-access/backend/internal/database"
	"github.com/gorilla/csrf"
	"github.com/ownwarden/funnel-access/backend/internal/scheduler"
	"github.com/ownwarden/funnel-access/backend/internal/service"
	// token package import removed
)

const (
	defaultServerAddr         = ":8080"
	defaultDBPath             = "./data/funnel_controller.db" // Relative to where the binary is run
	defaultSchedulerInterval  = 1 * time.Minute
	defaultCSRFAuthKeyMinLen = 32
)

type AppConfig struct {
	ServerAddr        string
	DBPath            string
	CSRFAuthKey       string // New field for CSRF key
	SchedulerInterval time.Duration
}

func loadConfig() AppConfig {
	cfg := AppConfig{
		ServerAddr:        getEnv("APP_SERVER_ADDR", defaultServerAddr),
		DBPath:            getEnv("APP_DB_PATH", defaultDBPath),
		CSRFAuthKey:       os.Getenv("APP_CSRF_AUTH_KEY"), // Required for CSRF protection
		SchedulerInterval: getEnvDuration("APP_SCHEDULER_INTERVAL", defaultSchedulerInterval),
	}

	if cfg.CSRFAuthKey == "" {
		log.Fatal("APP_CSRF_AUTH_KEY environment variable is required.")
	}
	if len(cfg.CSRFAuthKey) < defaultCSRFAuthKeyMinLen {
		log.Fatalf("APP_CSRF_AUTH_KEY must be at least %d characters long.", defaultCSRFAuthKeyMinLen)
	}

	if cfg.SchedulerInterval <= 0 {
		log.Printf("Warning: Invalid APP_SCHEDULER_INTERVAL, using default %v", defaultSchedulerInterval)
		cfg.SchedulerInterval = defaultSchedulerInterval
	}

	log.Printf("Configuration loaded: ServerAddr=%s, DBPath=%s, SchedulerInterval=%v, CSRFAuthKeyIsSet=%t",
		cfg.ServerAddr, cfg.DBPath, cfg.SchedulerInterval, cfg.CSRFAuthKey != "")
	return cfg
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	valStr := os.Getenv(key)
	if valStr == "" {
		return fallback
	}
	d, err := time.ParseDuration(valStr)
	if err != nil {
		log.Printf("Warning: Invalid duration format for %s ('%s'), using default %v. Error: %v", key, valStr, fallback, err)
		return fallback
	}
	return d
}

func main() {
	log.Println("Starting Funnel Access Controller...")
	cfg := loadConfig()

	// Initialize database
	dbStore, err := database.NewStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dbStore.Close()
	log.Println("Database initialized.")

	// Token manager initialization removed

	// Initialize Tailscale commander
	// This will modify the serve-config.json file.
	// The SERVE_CONFIG_PATH and FUNNEL_KEY environment variables can be used to configure its behavior.
	tsCommander := service.NewFileTailscaleCommander("", "") // Pass empty strings to use env vars or defaults
	log.Println("File-based Tailscale commander initialized.")

	// Initialize funnel service
	funnelService := service.NewFunnelService(dbStore, tsCommander)
	log.Println("Funnel service initialized.")

	// Initialize API handler
	apiHandler := api.NewAPIHandler(funnelService) // tokenManager removed
	log.Println("API handler initialized.")

	// Initialize and start scheduler
	appScheduler := scheduler.NewScheduler(funnelService, cfg.SchedulerInterval)
	go appScheduler.Start()
	defer appScheduler.Stop()
	log.Println("Scheduler started.")

	// Setup HTTP server
	router := api.NewRouter(apiHandler)

	// Initialize CSRF protection
	// TODO: Make csrf.Secure(true) the default and only set to false if APP_ENV=development
	csrfMiddleware := csrf.Protect(
		[]byte(cfg.CSRFAuthKey), // Use new CSRF auth key
		csrf.Secure(false),      // Set to true in production (requires HTTPS)
		csrf.Path("/"),           // Apply CSRF protection to all paths
		// Consider adding csrf.SameSite(csrf.SameSiteStrictMode)
	)

	server := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: csrfMiddleware(router), // Wrap the router with CSRF protection
		// Good practice: add timeouts
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown handling
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("HTTP server starting on %s", cfg.ServerAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe error: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-stopChan
	log.Println("Shutting down server...")

	// Shutdown server with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("HTTP server Shutdown error: %v", err)
	}

	log.Println("Server gracefully stopped.")
}

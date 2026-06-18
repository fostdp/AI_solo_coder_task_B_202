package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"bianzhong-acoustic-system/config"
	"bianzhong-acoustic-system/database"
	"bianzhong-acoustic-system/handlers"
	"bianzhong-acoustic-system/metrics"
	"bianzhong-acoustic-system/middleware"
	"bianzhong-acoustic-system/mqtt"
)

var (
	buildVersion = "dev"
	buildTime    = "unknown"
)

func main() {
	log.Println("========================================")
	log.Println("  古代编钟调音磨锉声学仿真与音高修正系统")
	log.Println("  Bianzhong Acoustic Simulation System")
	log.Printf("  Version: %s  Build: %s", buildVersion, buildTime)
	log.Println("========================================")

	configDir := getEnv("CONFIG_DIR", "./config")
	cfg, err := config.Load(configDir)
	if err != nil {
		log.Printf("WARN: Failed to load configuration files: %v (using embedded defaults)", err)
	} else {
		log.Printf("Loaded acoustic config: %d harmonic ratios", len(cfg.Acoustic.HarmonicRatios))
		log.Printf("Loaded constraint config: algorithm=%s max_outer_iter=%d",
			cfg.Constraint.Optimization.Algorithm,
			cfg.Constraint.Optimization.MaxOuterIterations)
	}

	database.Init()
	defer database.Close()

	mqtt.InitAlertManager()
	defer mqtt.Close()

	r := mux.NewRouter()
	r.Use(handlers.CORS)
	r.Use(middleware.GzipCompression)
	r.Use(metrics.PrometheusMiddleware)

	metrics.RegisterHandlers(r)

	api := r.PathPrefix("/api").Subrouter()

	api.HandleFunc("/bells", handlers.GetBells).Methods("GET")
	api.HandleFunc("/bells/{id}", handlers.GetBell).Methods("GET")

	api.HandleFunc("/measurements", handlers.PostAcousticMeasurement).Methods("POST")
	api.HandleFunc("/bells/{id}/measurements", handlers.GetAcousticMeasurements).Methods("GET")

	api.HandleFunc("/grinding", handlers.PostGrindingOperation).Methods("POST")
	api.HandleFunc("/bells/{id}/grinding", handlers.GetGrindingOperations).Methods("GET")

	api.HandleFunc("/bells/{id}/simulation", handlers.RunSimulation).Methods("POST")
	api.HandleFunc("/bells/{id}/correction", handlers.GetPitchCorrection).Methods("GET")

	api.HandleFunc("/alerts", handlers.GetAlerts).Methods("GET")
	api.HandleFunc("/dashboard/stats", handlers.GetDashboardStats).Methods("GET")
	api.HandleFunc("/healthz", healthzHandler).Methods("GET")
	api.HandleFunc("/version", versionHandler).Methods("GET")

	api.HandleFunc("/ws", handlers.WebSocketHandler)

	api.HandleFunc("/processes", handlers.GetTuningProcesses).Methods("GET")
	api.HandleFunc("/processes/compare", handlers.CompareTuningProcesses).Methods("POST")
	api.HandleFunc("/processes/stats", handlers.GetComparisonStats).Methods("GET")

	api.HandleFunc("/rules", handlers.GetEmpiricalRules).Methods("GET")
	api.HandleFunc("/rules/validate", handlers.ValidateEmpiricalRule).Methods("POST")

	api.HandleFunc("/comparisons", handlers.GetComparisonArticles).Methods("GET")

	api.HandleFunc("/virtual/start", handlers.StartVirtualTuning).Methods("POST")
	api.HandleFunc("/virtual/{session_id}", handlers.GetVirtualTuningSession).Methods("GET")
	api.HandleFunc("/virtual/{session_id}/grind", handlers.VirtualTuningGrind).Methods("POST")
	api.HandleFunc("/virtual/{session_id}/play", handlers.VirtualTuningPlay).Methods("POST")
	api.HandleFunc("/virtual/{session_id}/reset", handlers.VirtualTuningReset).Methods("POST")

	port := getEnv("PORT", "8080")

	frontendDir := getEnv("FRONTEND_DIR", "./frontend")
	if _, err := os.Stat(frontendDir); err == nil {
		fs := http.FileServer(http.Dir(frontendDir))
		r.PathPrefix("/").Handler(http.StripPrefix("/", fs))
		log.Printf("Serving frontend from: %s (gzip-enabled)", frontendDir)
	} else {
		log.Printf("WARN: Frontend dir %s not found: %v", frontendDir, err)
	}

	pprofAddr := getEnv("PPROF_ADDR", "")
	if pprofAddr != "" {
		go func() {
			log.Printf("pprof endpoint listening on %s/debug/pprof/", pprofAddr)
			if err := http.ListenAndServe(pprofAddr, nil); err != nil {
				log.Printf("pprof server stopped: %v", err)
			}
		}()
	}

	readTimeout := parseDuration(getEnv("HTTP_READ_TIMEOUT", "30s"), 30*time.Second)
	writeTimeout := parseDuration(getEnv("HTTP_WRITE_TIMEOUT", "30s"), 30*time.Second)
	idleTimeout := parseDuration(getEnv("HTTP_IDLE_TIMEOUT", "120s"), 120*time.Second)

	srv := &http.Server{
		Handler:      r,
		Addr:         ":" + port,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	go func() {
		log.Printf("Server starting on port %s", port)
		log.Printf("API endpoint:     http://localhost:%s/api", port)
		log.Printf("Health check:     http://localhost:%s/api/healthz", port)
		log.Printf("Metrics:          http://localhost:%s/metrics", port)
		log.Printf("WebSocket:        ws://localhost:%s/api/ws", port)
		if pprofAddr != "" {
			log.Printf("pprof:            http://%s/debug/pprof/", pprofAddr)
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	shutdownTimeout := parseDuration(getEnv("GRACEFUL_SHUTDOWN_TIMEOUT", "30s"), 30*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","version":"%s","build_time":"%s"}`, buildVersion, buildTime)
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"version":"%s","build_time":"%s","go_version":"%s"}`, buildVersion, buildTime, "1.22")
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func parseDuration(s string, defaultDur time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultDur
	}
	return d
}

package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"

	"bianzhong-acoustic-system/database"
	"bianzhong-acoustic-system/handlers"
	"bianzhong-acoustic-system/mqtt"
)

func main() {
	log.Println("========================================")
	log.Println("  古代编钟调音磨锉声学仿真与音高修正系统")
	log.Println("  Bianzhong Acoustic Simulation System")
	log.Println("========================================")

	database.Init()
	defer database.Close()

	mqtt.InitAlertManager()
	defer mqtt.Close()

	r := mux.NewRouter()
	r.Use(handlers.CORS)

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

	api.HandleFunc("/ws", handlers.WebSocketHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	frontendDir := "./frontend"
	if _, err := os.Stat(frontendDir); err == nil {
		r.PathPrefix("/").Handler(http.FileServer(http.Dir(frontendDir)))
		log.Printf("Serving frontend from: %s", frontendDir)
	}

	srv := &http.Server{
		Handler: r,
		Addr:    ":" + port,
	}

	go func() {
		log.Printf("Server starting on port %s", port)
		log.Printf("API endpoint: http://localhost:%s/api", port)
		log.Printf("WebSocket:    ws://localhost:%s/api/ws", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")
}

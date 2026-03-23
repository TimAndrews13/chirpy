package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

// Struct to Track Stateful, In-Memory data
type apiConfig struct {
	fileserverHits atomic.Int32
}

// Middleware Method for Incrementing fileserverHits
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// Create handler that writes the number of requests so far
func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	hits := cfg.fileserverHits.Load()
	writeString := fmt.Sprintf("Hits: %d", hits)
	w.Write([]byte(writeString))
}

// Create handler that resets the number of requests
func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	cfg.fileserverHits.Store(0)
	hits := cfg.fileserverHits.Load()
	writeString := fmt.Sprintf("Hits: %d", hits)
	w.Write([]byte(writeString))
}

// Add Readiness Endpoint
func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	//Create ServeMux
	mux := http.NewServeMux()

	//Define handler
	handler := http.FileServer(http.Dir("."))

	//Define Tracker for in-memory data
	apiCFG := apiConfig{}

	//Register Readiness Endpoint
	mux.HandleFunc("/healthz", handlerReadiness)
	//Register Metrics Endpoint
	mux.HandleFunc("/metrics", apiCFG.metricsHandler)
	//Register Reset Endpoint
	mux.HandleFunc("/reset", apiCFG.resetHandler)

	//Register FileServer for /app/
	mux.Handle("/app/", http.StripPrefix("/app", apiCFG.middlewareMetricsInc(handler)))

	//Define Server Params
	server := &http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	log.Printf("Serving on port: 8080\n")
	log.Fatal(server.ListenAndServe())
}

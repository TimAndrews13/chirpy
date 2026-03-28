package main

import (
	"fmt"
	"net/http"
)

// Middleware Method for Incrementing fileserverHits
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// Create handler that writes the number of requests so far
func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	hits := cfg.fileserverHits.Load()
	html := fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", hits)
	w.Write([]byte(html))
}

// Create handler that resets the number of requests
func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	//check platform before running resetHandler logic
	if cfg.platform != "dev" {
		respondWithError(w, http.StatusForbidden, "action forbidden")
		return
	}
	//Set Reesponse Headers
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	//Reset fileserverHits to 0
	cfg.fileserverHits.Store(0)
	hits := cfg.fileserverHits.Load()
	//Reset User table by Deleting It
	err := cfg.db.DeleteUsers(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeString := fmt.Sprintf("Hits: %d\n", hits)
	w.Write([]byte(writeString))
}

// Add Readiness Endpoint
func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

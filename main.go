package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
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
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	hits := cfg.fileserverHits.Load()
	html := fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", hits)
	w.Write([]byte(html))
}

// Create handler that resets the number of requests
func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	cfg.fileserverHits.Store(0)
	hits := cfg.fileserverHits.Load()
	writeString := fmt.Sprintf("Hits: %d\n", hits)
	w.Write([]byte(writeString))
}

// Helper Function for respond with Error
func respondWithError(w http.ResponseWriter, code int, msg string) {
	//Set struct that is shape of JSON
	type returnVals struct {
		Error string `json:"error"`
	}

	//Set respBody and its value
	respBody := returnVals{
		Error: msg,
	}
	//Marshall respBody to JSON
	dat, err := json.Marshal(respBody)
	if err != nil {
		log.Printf("Error marsalling JSON: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	//Set Headers and Write data
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

// Helper Function for respond with JSON
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	//Marshall payload to JSON
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	//Set Headers and Write data
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

// Helper Function to Remove Bad Words
func helperCleanText(msg string, badWords map[string]struct{}) string {

	words := strings.Split(msg, " ")
	for i, word := range words {
		if _, ok := badWords[strings.ToLower(word)]; ok {
			words[i] = "****"
		}
	}
	return strings.Join(words, " ")
}

// Create handler that accepts POST requests at /api/validate_chirp
func handlerValidateChirp(w http.ResponseWriter, r *http.Request) {
	//Set struct to receive JSON
	type parameters struct {
		Body string `json:"body"`
	}

	//Decode JSON Request Body
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	//Check length of parms.body
	if len(params.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}

	//Pass params.Body into Helper Funciton to Clean Bad Words
	badWords := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}
	params.Body = helperCleanText(params.Body, badWords)

	//Set struct to respond with JSON
	type respJSON struct {
		CleanedBody string `json:"cleaned_body"`
	}

	resBody := respJSON{CleanedBody: params.Body}

	respondWithJSON(w, http.StatusOK, resBody)
}

// Add Readiness Endpoint
func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func main() {
	//Create ServeMux
	mux := http.NewServeMux()

	//Define handler
	handler := http.FileServer(http.Dir("."))

	//Define Tracker for in-memory data
	apiCFG := apiConfig{}

	//Register Readiness Endpoint
	mux.HandleFunc("GET /api/healthz", handlerReadiness)
	//Register Metrics Endpoint
	mux.HandleFunc("GET /admin/metrics", apiCFG.metricsHandler)
	//Register Reset Endpoint
	mux.HandleFunc("POST /admin/reset", apiCFG.resetHandler)
	//Register Validat Chirp Endpoint
	mux.HandleFunc("POST /api/validate_chirp", handlerValidateChirp)

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

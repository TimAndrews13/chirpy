package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/TimAndrews13/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Struct to Track Stateful, In-Memory data
type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	secretKey      string
}

func main() {
	//Load .env file and get connection
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL must be set")
	}
	//Get Platform from .env file
	platform := os.Getenv("PLATFORM")
	if platform == "" {
		log.Fatal("PLATFORM must be set")
	}
	//Get Secret Key from .env fil
	secretKey := os.Getenv("SECRET")
	if secretKey == "" {
		log.Fatal("SECRET must be set")
	}
	//Open DB connection
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("error connecting to postgres database: %v\n", err)
	}
	//Create new database queries
	dbQueries := database.New(db)

	//Create ServeMux
	mux := http.NewServeMux()

	//Define handler
	handler := http.FileServer(http.Dir("."))

	//Define Tracker for in-memory data
	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
		db:             dbQueries,
		platform:       platform,
		secretKey:      secretKey,
	}

	//Register Readiness Endpoint
	mux.HandleFunc("GET /api/healthz", handlerReadiness)
	//Register Metrics Endpoint
	mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	//Register Reset Endpoint
	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)
	//Register API User Endpoint
	mux.HandleFunc("POST /api/users", apiCfg.handlerNewUser)
	//Register API Login Endpoint
	mux.HandleFunc("POST /api/login", apiCfg.handlerLoginUser)
	//Register PUT /api/users Endpoint
	mux.HandleFunc("PUT /api/users", apiCfg.handlerUserUpdate)
	//Register API Chirps POST Endpoint
	mux.HandleFunc("POST /api/chirps", apiCfg.handlerNewChirp)
	//Register API Chirps GET Endpoint
	mux.HandleFunc("GET /api/chirps", apiCfg.handlerGetAllChirps)
	//Register API Chirps GET Endpoint for Single Chirp
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.handlerGetChirp)
	//Register DELETE /api/chrips Endpoint for Single Chirp
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.handlerDeleteChirp)
	//Register Post /api/refresh Endpoint
	mux.HandleFunc("POST /api/refresh", apiCfg.handlerRefresh)
	//Register Post /api/revoke Endpoint
	mux.HandleFunc("POST /api/revoke", apiCfg.handlerRevokeRefreshToken)
	//Register POST /api/polka/webhooks Endpoint
	mux.HandleFunc("POST /api/polka/webhooks", apiCfg.handlerUpdateUserToRed)

	//Register FileServer for /app/
	mux.Handle("/app/", http.StripPrefix("/app", apiCfg.middlewareMetricsInc(handler)))

	//Define Server Params
	server := &http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	log.Printf("Serving on port: 8080\n")
	log.Fatal(server.ListenAndServe())
}

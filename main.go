package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/TimAndrews13/chirpy/internal/auth"
	"github.com/TimAndrews13/chirpy/internal/database"
	"github.com/google/uuid"
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

// Middleware Method for Incrementing fileserverHits
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
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

// Create Chirp struct
type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

// Create handler that accepts POST new chirps at api/chirps
func (cfg *apiConfig) handlerNewChirp(w http.ResponseWriter, r *http.Request) {
	//Set struct to receive JSON
	type parameters struct {
		Body string `json:"body"`
	}

	//Get Bearer Token
	tokenString, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	//Validate token
	userID, err := auth.ValidateJWT(tokenString, cfg.secretKey)
	if err != nil {
		log.Printf("error validating token: %s", err)
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	//Decode JSON Request Body
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
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

	//Use CreateChrip SQL Query to create the new chrip and return the new chirp
	chirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
		Body:   params.Body,
		UserID: userID,
	})
	if err != nil {
		log.Printf("error creating new chirp: %s\n", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	//Map the database package User Sruct to main package Chirp Struct
	resChirp := Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}

	//Pass new main package Chrip Struct as payload for respondWithJSON helper function
	respondWithJSON(w, http.StatusCreated, resChirp)
}

// Add Handler to Get All Chirps
func (cfg *apiConfig) handlerGetAllChirps(w http.ResponseWriter, r *http.Request) {
	//Use GetAllChirps function to pull all Chirps from postgres
	chirps, err := cfg.db.GetAllChirps(r.Context())
	if err != nil {
		log.Printf("error retrieving all chirps: %s", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	//Set Array of main package Chirps Struct
	resChirps := make([]Chirp, len(chirps))

	for i := range chirps {
		resChirps[i].ID = chirps[i].ID
		resChirps[i].CreatedAt = chirps[i].CreatedAt
		resChirps[i].UpdatedAt = chirps[i].UpdatedAt
		resChirps[i].Body = chirps[i].Body
		resChirps[i].UserID = chirps[i].UserID
	}

	//Pass new Array of main package Chirps Struct as payload for respondWithJSON helper function
	respondWithJSON(w, http.StatusOK, resChirps)
}

// Add Handler to Get Single Chirp
func (cfg *apiConfig) handlerGetChirp(w http.ResponseWriter, r *http.Request) {
	//Check if Chirp ID exists
	chirpIDString := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(chirpIDString)
	if err != nil {
		log.Printf("chirp not found: %s", err)
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	//Use GetChirp to retrieve chirp from postgres database
	chirp, err := cfg.db.GetChirp(r.Context(), chirpID)
	if err != nil {
		log.Printf("chirp not found: %s", err)
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	//Set Response Chirp struct using struct in main package
	resChirp := Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}

	//Pass Response Chirp as payload for respondWithJSON helper function
	respondWithJSON(w, http.StatusOK, resChirp)
}

// Add Handler to Get Delete Chirp
func (cfg *apiConfig) handlerDeleteChirp(w http.ResponseWriter, r *http.Request) {
	//Get Bearer Token
	tokenString, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	//Validate token
	userID, err := auth.ValidateJWT(tokenString, cfg.secretKey)
	if err != nil {
		log.Printf("error validating token: %s", err)
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	//Check if Chirp ID exists
	chirpIDString := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(chirpIDString)
	if err != nil {
		log.Printf("chirp not found: %s", err)
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	//Use GetChirp to retrieve chirp from postgres database
	chirp, err := cfg.db.GetChirp(r.Context(), chirpID)
	if err != nil {
		log.Printf("chirp not found: %s", err)
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	//Check userID from header against userID from returned chirp
	if userID != chirp.UserID {
		respondWithError(w, http.StatusForbidden, "user not creator of chirp")
		return
	}

	//Use GetChirp to retrieve chirp from postgres database
	err = cfg.db.DeleteChirp(r.Context(), chirpID)
	if err != nil {
		log.Printf("chirp not found: %s", err)
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	//Pass Response Chirp as payload for respondWithJSON helper function
	w.WriteHeader(http.StatusNoContent)
}

// Add Readiness Endpoint
func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

// Create User struct
type User struct {
	ID          uuid.UUID `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Email       string    `json:"email"`
	IsChirpyRed bool      `json:"is_chirpy_red"`
}

// Add Handler for Creating New Users
func (cfg *apiConfig) handlerNewUser(w http.ResponseWriter, r *http.Request) {
	//Set struct to receive JSON
	type parameters struct {
		Password string `json:"password"`
		Email    string `json:"email"`
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

	//Hash Password
	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("error hashing password: %s", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	//Use CreateUser SQL Query to create the new user and return the new user
	user, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		log.Printf("error creating new user: %s", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	//Map the database package User Sruct to main package User Struct
	returnUser := User{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	}

	//User Respong with JSON Helper Function to Marhsal return User
	respondWithJSON(w, http.StatusCreated, returnUser)
}

// Add handler for Logging in User
func (cfg *apiConfig) handlerLoginUser(w http.ResponseWriter, r *http.Request) {
	//Set struct to receive JSON
	type parameters struct {
		Password string `json:"password"`
		Email    string `json:"email"`
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

	//Use GetUser SQL Query to pull user by email
	user, err := cfg.db.GetUser(r.Context(), params.Email)
	if err != nil {
		log.Printf("error geting user: %s", err)
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	//Check Entered Password in Params against Returned Users Hashed Password
	ok, _ := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if !ok {
		log.Printf("Incorrect email or password: %s", err)
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	//Generate JWT Token
	token, err := auth.MakeJWT(user.ID, cfg.secretKey, time.Hour)
	if err != nil {
		log.Printf("Error creating token: %s", err)
		respondWithError(w, http.StatusInternalServerError, "error creating token")
		return
	}

	//Generate Refresh Token
	refreshToken := auth.MakeRefreshToken()
	//Add Refresh Token to the Database
	rToken, err := cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(time.Hour * 24 * 60),
	})
	if err != nil {
		log.Printf("error creating refresh token: %s", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	//Set internal response struct for hanlderLoginUser
	type response struct {
		User
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}

	//Map the database package User Sruct to main package User Struct
	returnUser := User{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	}

	//Map response struct
	responseStruct := response{
		User:         returnUser,
		Token:        token,
		RefreshToken: rToken.Token,
	}

	//User Respong with JSON Helper Function to Marhsal return User
	respondWithJSON(w, http.StatusOK, responseStruct)
}

func (cfg *apiConfig) handlerUserUpdate(w http.ResponseWriter, r *http.Request) {
	//Set struct to receive JSON
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	//Get Bearer Token
	tokenString, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	//Validate token
	userID, err := auth.ValidateJWT(tokenString, cfg.secretKey)
	if err != nil {
		log.Printf("error validating token: %s", err)
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	//Decode JSON Request Body
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	//Hash New password
	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("error hashing password: %s", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	//Update User
	user, err := cfg.db.UpdateUser(r.Context(), database.UpdateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPassword,
		ID:             userID,
	})
	if err != nil {
		log.Printf("error updating user: %s", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	//Map the database package User Sruct to main package User Struct
	returnUser := User{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	}

	//User Respong with JSON Helper Function to Marhsal return User
	respondWithJSON(w, http.StatusOK, returnUser)
}

// Handler Function to Refresh
func (cfg *apiConfig) handlerRefresh(w http.ResponseWriter, r *http.Request) {
	//Get Bearer Token
	tokenString, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	//Get Refresh Token from the Database
	refreshToken, err := cfg.db.GetRefreshToken(r.Context(), tokenString)
	if err != nil {
		log.Printf("error retrieving refresh token: %s", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	//Validate RefreshToken
	if time.Now().After(refreshToken.ExpiresAt) {
		log.Print("refresh token expired")
		respondWithError(w, http.StatusUnauthorized, "refresh token expired")
		return
	}
	if !refreshToken.RevokedAt.Time.IsZero() {
		log.Print("refresh token revoked")
		respondWithError(w, http.StatusUnauthorized, "refresh token revoked")
		return
	}
	//Generate New JWT
	token, err := auth.MakeJWT(refreshToken.UserID, cfg.secretKey, time.Hour)
	if err != nil {
		log.Printf("Error creating token: %s", err)
		respondWithError(w, http.StatusInternalServerError, "error creating token")
		return
	}
	// Create Response struct
	type response struct {
		Token string `json:"token"`
	}
	//Set responseStruct
	responseStruct := response{
		Token: token,
	}
	//Respond with JSON Helper Function to Marhsal return responseStruct
	respondWithJSON(w, http.StatusOK, responseStruct)
}

func (cfg *apiConfig) handlerRevokeRefreshToken(w http.ResponseWriter, r *http.Request) {
	//Get Bearer Token
	tokenString, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	//Get Refresh Token from the Database
	err = cfg.db.RevokeRefreshToken(r.Context(), tokenString)
	if err != nil {
		log.Printf("error revoking refresh token: %s", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) handlerUpdateUserToRed(w http.ResponseWriter, r *http.Request) {
	//Set struct to receive JSON
	type parameters struct {
		Event string `json:"event"`
		Data  struct {
			UserID uuid.UUID `json:"user_id"`
		} `json:"data"`
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
	//Check Event type
	if params.Event != "user.upgraded" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	//Update User to Chripy Red
	_, err = cfg.db.UpdateUserToRed(r.Context(), params.Data.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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

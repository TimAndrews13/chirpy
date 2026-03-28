package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/TimAndrews13/chirpy/internal/auth"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

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
	//Load .env file and get connection
	godotenv.Load()
	polkaKey := os.Getenv("POLKA_KEY")

	//Get API Key
	apiKey, err := auth.GetAPIKey(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if apiKey != polkaKey {
		respondWithError(w, http.StatusUnauthorized, "api key does not match polka api key")
		return
	}

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
	err = decoder.Decode(&params)
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

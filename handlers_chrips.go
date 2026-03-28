package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/TimAndrews13/chirpy/internal/auth"
	"github.com/TimAndrews13/chirpy/internal/database"
	"github.com/google/uuid"
)

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
	//Check for sort method
	srt := r.URL.Query().Get("sort")
	if srt != "" && srt != "asc" && srt != "desc" {
		log.Printf("ivalid sort parameter")
		respondWithError(w, http.StatusBadRequest, "invalid sort parameter")
		return
	}
	if srt == "" {
		srt = "asc"
	}
	//Check for author_id
	s := r.URL.Query().Get("author_id")
	//if author_id is present get chirps for user_id
	if s != "" {
		userID, err := uuid.Parse(s)
		if err != nil {
			log.Printf("userID not found: %s", err)
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		//GetAllChirpsByUser
		chirpsByUser, err := cfg.db.GetChirpsByAuthor(r.Context(), userID)
		if err != nil {
			log.Printf("error retrieving chirps for user %v: %v", userID, err)
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		//Sort chirpsByUser Slice
		if srt == "asc" {
			sort.Slice(chirpsByUser, func(i, j int) bool {
				return chirpsByUser[i].CreatedAt.Before(chirpsByUser[j].CreatedAt)
			})
		} else if srt == "desc" {
			sort.Slice(chirpsByUser, func(i, j int) bool {
				return chirpsByUser[i].CreatedAt.After(chirpsByUser[j].CreatedAt)
			})
		}
		//Set Array of main package Chirps Struct
		resChirps := make([]Chirp, len(chirpsByUser))

		for i := range chirpsByUser {
			resChirps[i].ID = chirpsByUser[i].ID
			resChirps[i].CreatedAt = chirpsByUser[i].CreatedAt
			resChirps[i].UpdatedAt = chirpsByUser[i].UpdatedAt
			resChirps[i].Body = chirpsByUser[i].Body
			resChirps[i].UserID = chirpsByUser[i].UserID
		}
		//Pass new Array of main package Chirps Struct as payload for respondWithJSON helper function
		respondWithJSON(w, http.StatusOK, resChirps)
		return
	}

	//Use GetAllChirps function to pull all Chirps from postgres
	chirps, err := cfg.db.GetAllChirps(r.Context())
	if err != nil {
		log.Printf("error retrieving all chirps: %s", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	//Sort chirpsByUser Slice
	if srt == "asc" {
		sort.Slice(chirps, func(i, j int) bool {
			return chirps[i].CreatedAt.Before(chirps[j].CreatedAt)
		})
	} else if srt == "desc" {
		sort.Slice(chirps, func(i, j int) bool {
			return chirps[i].CreatedAt.After(chirps[j].CreatedAt)
		})
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

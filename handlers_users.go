package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/TimAndrews13/chirpy/internal/auth"
	"github.com/TimAndrews13/chirpy/internal/database"
	"github.com/google/uuid"
)

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

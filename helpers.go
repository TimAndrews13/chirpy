package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

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

package main

import (
	"log"
	"net/http"
)

// Add Readiness Endpoint
func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	//Create ServeMux
	mux := http.NewServeMux()

	//Register Handler Funciton
	mux.HandleFunc("/healthz", handlerReadiness)

	//Register Handle
	mux.Handle("/app/", http.StripPrefix("/app", http.FileServer(http.Dir("."))))

	//Define Server Params
	server := &http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	log.Printf("Serving on port: 8080\n")
	log.Fatal(server.ListenAndServe())
}

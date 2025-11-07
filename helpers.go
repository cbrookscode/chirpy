package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func respondWithError(w http.ResponseWriter, msg string, code int, err error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		log.Println(err)
	}
	if code > 499 {
		log.Printf("Responding with 5XX error: %v\n", msg)
	}

	type errorResp struct {
		Error string `json:"error"`
	}
	payload := errorResp{Error: msg}

	respondWithJson(w, code, payload)
}

func respondWithJson(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")

	bytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling json: %v\n", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(code)
	w.Write(bytes)
}

func filterProfanity(text string) string {
	badWords := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}

	words := strings.Fields(text)
	for i, word := range words {
		if _, ok := badWords[strings.ToLower(word)]; ok {
			words[i] = "****"
		}
	}
	return strings.Join(words, " ")
}

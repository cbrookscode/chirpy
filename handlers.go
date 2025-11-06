package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cbrookscode/chirpy/internal/database"
	"github.com/google/uuid"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
}

func (a *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (a *apiConfig) handlerMetrics(reswrit http.ResponseWriter, req *http.Request) {
	reswrit.Header().Add("Content-Type", "text/html; charset=utf-8")
	reswrit.WriteHeader(http.StatusOK)
	new := fmt.Sprintf(
		`<html>
			<body>
				<h1>Welcome, Chirpy Admin</h1>
				<p>Chirpy has been visited %d times!</p>
			</body>
		</html>`, a.fileserverHits.Load())
	reswrit.Write([]byte(new))
}

func (a *apiConfig) handlerCreateUser(resWriter http.ResponseWriter, req *http.Request) {
	type incoming struct {
		Email string `json:"email"`
	}

	type User struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string    `json:"email"`
	}

	userinfo := incoming{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&userinfo)
	if err != nil {
		log.Printf("Error decoding json data in POST request: %v\n", err)
		respondWithError(resWriter, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	if userinfo.Email == "" {
		respondWithError(resWriter, "Provided an empty string for username", 400, nil)
		return
	}
	dbUser, err := a.db.CreateUser(req.Context(), sql.NullString{String: userinfo.Email, Valid: true})
	if err != nil {
		respondWithError(resWriter, "Couldn't create user", http.StatusInternalServerError, err)
		return
	}

	// to control json tags map dbuser to User struct
	updatedUser := User{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt.Time,
		UpdatedAt: dbUser.UpdatedAt.Time,
		Email:     dbUser.Email.String,
	}
	respondWithJson(resWriter, http.StatusCreated, updatedUser)
}

func (a *apiConfig) handlerReset(reswrit http.ResponseWriter, req *http.Request) {
	reswrit.Header().Add("Content-Type", "text/plain; charset=utf-8")
	if a.platform != "dev" {
		respondWithError(reswrit, "Reset is only allowed in dev environment", http.StatusForbidden, nil)
		return
	}
	err := a.db.DeleteUsers(req.Context())
	if err != nil {
		respondWithError(reswrit, "Something went wrong", 500, err)
		return
	}

	a.fileserverHits.Store(0)
	new := fmt.Sprintf("Users have been deleted, and counter has been reset: %v\n", a.fileserverHits.Load())
	reswrit.WriteHeader(http.StatusOK)
	reswrit.Write([]byte(new))
}

func respondWithError(w http.ResponseWriter, msg string, code int, err error) {
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

func handlerReadiness(resWriter http.ResponseWriter, req *http.Request) {
	resWriter.Header().Add("Content-Type", "text/plain; charset=utf-8")
	resWriter.WriteHeader(http.StatusOK)
	resWriter.Write([]byte("OK"))
}

func handlerValidateChirp(resWriter http.ResponseWriter, req *http.Request) {
	type incoming struct {
		Body string `json:"body"`
	}

	type returnVals struct {
		CleanedBody string `json:"cleaned_body"`
	}

	chirp := incoming{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&chirp)
	if err != nil {
		log.Printf("Error decoding json data in POST request: %v\n", err)
		respondWithError(resWriter, "Something went wrong", 500, err)
		return
	}

	filteredChirp := filterProfanity(chirp.Body)

	if len(filteredChirp) <= 140 {
		// is valid
		payload := returnVals{CleanedBody: filteredChirp}
		respondWithJson(resWriter, 200, payload)
	} else {
		// is not valid
		respondWithError(resWriter, "Chrip is too long", 400, nil)
	}
}

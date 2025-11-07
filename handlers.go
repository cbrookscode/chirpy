package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
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

func (a *apiConfig) handlerGetChirps(resWriter http.ResponseWriter, req *http.Request) {
	resWriter.Header().Add("Content-Type", "text/html; charset=utf-8")
	listOfChirps := []Chirp{}

	chirps, err := a.db.GetChirpsAscByCreated(req.Context())
	if err != nil {
		respondWithError(resWriter, "Couldn't grab users in ascending order from database", http.StatusInternalServerError, err)
		return
	}

	for _, chirp := range chirps {
		listOfChirps = append(listOfChirps, Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt.Time,
			UpdatedAt: chirp.UpdatedAt.Time,
			Body:      chirp.Body.String,
			UserID:    chirp.UserID.UUID,
		})
	}

	respondWithJson(resWriter, http.StatusOK, listOfChirps)
}

func (a *apiConfig) handlerGetSingleChirp(resWriter http.ResponseWriter, req *http.Request) {
	resWriter.Header().Add("Content-Type", "text/html; charset=utf-8")

	stringid := req.PathValue("chirpID")
	convertedID, err := uuid.Parse(stringid)
	if err != nil {
		respondWithError(resWriter, "user id provided is not a valid UUID", http.StatusBadRequest, nil)
		return
	}
	dbChirp, err := a.db.GetSingleChirp(req.Context(), convertedID)
	if err != nil {
		respondWithError(resWriter, "Chirp not found", http.StatusNotFound, err)
		return
	}

	finalChirp := Chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt.Time,
		UpdatedAt: dbChirp.UpdatedAt.Time,
		Body:      dbChirp.Body.String,
		UserID:    dbChirp.UserID.UUID,
	}

	respondWithJson(resWriter, http.StatusOK, finalChirp)
}

func (a *apiConfig) handlerReset(reswrit http.ResponseWriter, req *http.Request) {
	reswrit.Header().Add("Content-Type", "text/plain; charset=utf-8")
	if a.platform != "dev" {
		respondWithError(reswrit, "Reset is only allowed in dev environment", http.StatusForbidden, nil)
		return
	}
	err := a.db.DeleteUsers(req.Context())
	if err != nil {
		respondWithError(reswrit, "Failed to delete user records", 500, err)
		return
	}

	err = a.db.DeleteChirps(req.Context())
	if err != nil {
		respondWithError(reswrit, "Failed to delete chirps records", http.StatusInternalServerError, err)
		return
	}

	a.fileserverHits.Store(0)
	new := fmt.Sprintf("Users and chirps have been deleted, and counter has been reset: %v\n", a.fileserverHits.Load())
	reswrit.WriteHeader(http.StatusOK)
	reswrit.Write([]byte(new))
}

func handlerReadiness(resWriter http.ResponseWriter, req *http.Request) {
	resWriter.Header().Add("Content-Type", "text/plain; charset=utf-8")
	resWriter.WriteHeader(http.StatusOK)
	resWriter.Write([]byte("OK"))
}

func (cfg *apiConfig) handlerChirps(resWriter http.ResponseWriter, req *http.Request) {
	type incoming struct {
		Body   string `json:"body"`
		UserID string `json:"user_id"`
	}

	resWriter.Header().Add("Content-Type", "text/plain; charset=utf-8")

	chirp := incoming{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&chirp)
	if err != nil {
		log.Printf("Error decoding json data in POST request: %v\n", err)
		respondWithError(resWriter, "Something went wrong", 500, err)
		return
	}

	// Filter profanity and make sure chirp is less than or equal to 140 characters
	filteredChirp := filterProfanity(chirp.Body)
	if len(filteredChirp) <= 140 { // valid
		convertedID, err := uuid.Parse(chirp.UserID)
		if err != nil {
			respondWithError(resWriter, "user id provided is not a valid UUID", http.StatusBadRequest, nil)
			return
		}

		dbChirp, err := cfg.db.CreateChirp(req.Context(), database.CreateChirpParams{ // store chirp in db
			Body: sql.NullString{
				String: filteredChirp,
				Valid:  true},
			UserID: uuid.NullUUID{UUID: convertedID, Valid: true},
		})
		if err != nil {
			respondWithError(resWriter, "Error storing chrip in database", http.StatusInternalServerError, err)
			return
		}

		payload := Chirp{ // adjust returned struct to customize json tags
			ID:        dbChirp.ID,
			CreatedAt: dbChirp.CreatedAt.Time,
			UpdatedAt: dbChirp.UpdatedAt.Time,
			Body:      dbChirp.Body.String,
			UserID:    dbChirp.UserID.UUID,
		}
		respondWithJson(resWriter, http.StatusCreated, payload)
	} else { // is not valid
		respondWithError(resWriter, "Chrip is too long", 400, nil)
	}
}

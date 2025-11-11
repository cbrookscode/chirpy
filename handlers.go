package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/cbrookscode/chirpy/internal/auth"
	"github.com/cbrookscode/chirpy/internal/database"
	"github.com/google/uuid"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	secret         string
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

type User struct {
	ID           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
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
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	userinfo := incoming{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&userinfo)
	if err != nil {
		log.Printf("Error decoding json data in POST request: %v\n", err)
		respondWithError(resWriter, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	if userinfo.Email == "" || userinfo.Password == "" {
		respondWithError(resWriter, "Provided an empty string for username or password", 400, nil)
		return
	}

	hash, err := auth.HashPassword(userinfo.Password)
	if err != nil {
		respondWithError(resWriter, "Failed to hash password", http.StatusInternalServerError, err)
		return
	}
	dbUser, err := a.db.CreateUser(req.Context(), database.CreateUserParams{
		Email: sql.NullString{
			String: userinfo.Email, Valid: true,
		},
		HashedPassword: sql.NullString{
			String: hash, Valid: true,
		},
	})
	if err != nil {
		respondWithError(resWriter, "Couldn't create user", http.StatusInternalServerError, err)
		return
	}

	respondWithJson(resWriter, http.StatusCreated, User{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt.Time,
		UpdatedAt: dbUser.UpdatedAt.Time,
		Email:     dbUser.Email.String,
	})
}

func (a *apiConfig) handlerGetChirps(resWriter http.ResponseWriter, req *http.Request) {
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
	stringid := req.PathValue("chirpID")
	convertedID, err := uuid.Parse(stringid)
	if err != nil {
		respondWithError(resWriter, "chirp id provided is not a valid UUID", http.StatusBadRequest, nil)
		return
	}
	dbChirp, err := a.db.GetSingleChirp(req.Context(), convertedID)
	if err != nil {
		respondWithError(resWriter, "Chirp not found", http.StatusNotFound, err)
		return
	}

	respondWithJson(resWriter, http.StatusOK, Chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt.Time,
		UpdatedAt: dbChirp.UpdatedAt.Time,
		Body:      dbChirp.Body.String,
		UserID:    dbChirp.UserID.UUID,
	})
}

func (a *apiConfig) handlerReset(reswrit http.ResponseWriter, req *http.Request) {
	if a.platform != "dev" {
		respondWithError(reswrit, "Reset is only allowed in dev environment", http.StatusForbidden, nil)
		return
	}
	err := a.db.DeleteRefreshTokens(req.Context())
	if err != nil {
		log.Printf("issue deleting refresh token records: %v", err)
		respondWithError(reswrit, "Failed to refresh token records", 500, err)
		return
	}
	err = a.db.DeleteUsers(req.Context())
	if err != nil {
		log.Printf("issue deleting user records: %v", err)
		respondWithError(reswrit, "Failed to delete user records", 500, err)
		return
	}

	err = a.db.DeleteChirps(req.Context())
	if err != nil {
		respondWithError(reswrit, "Failed to delete chirps records", http.StatusInternalServerError, err)
		return
	}

	a.fileserverHits.Store(0)

	reswrit.Header().Add("Content-Type", "text/plain; charset=utf-8")
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

	tokenString, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(resWriter, "Token not provied", http.StatusUnauthorized, nil)
		return
	}
	userUUID, err := auth.ValidateJWT(tokenString, cfg.secret)
	if err != nil {
		respondWithError(resWriter, "Token invalid", http.StatusUnauthorized, nil)
		return
	}

	chirp := incoming{}
	decoder := json.NewDecoder(req.Body)
	err = decoder.Decode(&chirp)
	if err != nil {
		log.Printf("Error decoding json data in POST request: %v\n", err)
		respondWithError(resWriter, "Something went wrong", 500, err)
		return
	}

	// Filter profanity and make sure chirp is less than or equal to 140 characters
	filteredChirp := filterProfanity(chirp.Body)
	if len(filteredChirp) <= 140 { // valid
		dbChirp, err := cfg.db.CreateChirp(req.Context(), database.CreateChirpParams{ // store chirp in db
			Body: sql.NullString{
				String: filteredChirp,
				Valid:  true},
			UserID: uuid.NullUUID{UUID: userUUID, Valid: true},
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

func (cfg *apiConfig) handlerValidateUser(resWriter http.ResponseWriter, req *http.Request) {
	type incoming struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	userinfo := incoming{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&userinfo)
	if err != nil {
		log.Printf("Error decoding json data in POST request: %v\n", err)
		respondWithError(resWriter, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	if userinfo.Email == "" || userinfo.Password == "" {
		respondWithError(resWriter, "Provided an empty string for username or password", 400, nil)
		return
	}

	dbUser, err := cfg.db.GetUserByEmail(req.Context(), sql.NullString{String: userinfo.Email, Valid: true})
	if err != nil {
		respondWithError(resWriter, "No user found", http.StatusUnauthorized, err)
		return
	}

	match, err := auth.CheckPasswordHash(userinfo.Password, dbUser.HashedPassword.String)
	if err != nil {
		respondWithError(resWriter, "Issue checking password hash match", http.StatusInternalServerError, err)
		return
	}
	if !match {
		respondWithError(resWriter, "Invalid password", http.StatusUnauthorized, nil)
		return
	}

	tokenString, err := auth.MakeJWT(dbUser.ID, cfg.secret)
	if err != nil {
		respondWithError(resWriter, "Issue generating token", http.StatusInternalServerError, err)
		return
	}
	refreshString, err := auth.MakeRefreshToken()
	if err != nil {
		respondWithError(resWriter, "Issue generating refresh token", http.StatusInternalServerError, err)
		return
	}
	later := time.Now().UTC().Add(time.Hour * 1440)
	_, err = cfg.db.StoreRefreshToken(req.Context(), database.StoreRefreshTokenParams{
		Token:     refreshString,
		UserID:    dbUser.ID,
		ExpiresAt: sql.NullTime{Time: later, Valid: true},
	})
	if err != nil {
		respondWithError(resWriter, "Issue storing refresh token in database", http.StatusInternalServerError, err)
		return
	}
	respondWithJson(resWriter, http.StatusOK, User{
		ID:           dbUser.ID,
		CreatedAt:    dbUser.CreatedAt.Time,
		UpdatedAt:    dbUser.UpdatedAt.Time,
		Email:        dbUser.Email.String,
		Token:        tokenString,
		RefreshToken: refreshString,
	})
}

func (cfg *apiConfig) handlerRefreshToken(resWriter http.ResponseWriter, req *http.Request) {
	refTokenString, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(resWriter, "Refresh token not provied", http.StatusUnauthorized, nil)
		return
	}

	dbRefToken, err := cfg.db.GrabRefreshToken(req.Context(), refTokenString)
	if err != nil {
		respondWithError(resWriter, "Refresh token not valid", http.StatusUnauthorized, nil)
		return
	}
	if dbRefToken.ExpiresAt.Time.Before(time.Now().UTC()) {
		respondWithError(resWriter, "Refresh token expired", http.StatusUnauthorized, nil)
		return
	}
	if dbRefToken.RevokedAt.Valid {
		respondWithError(resWriter, "Refresh token revoked", http.StatusUnauthorized, nil)
		return
	}

	tokenstring, err := auth.MakeJWT(dbRefToken.UserID, cfg.secret)
	if err != nil {
		respondWithError(resWriter, "Failed to get new token", http.StatusInternalServerError, err)
		return
	}

	respondWithJson(resWriter, http.StatusOK, struct {
		Token string `json:"token"`
	}{
		Token: tokenstring,
	})
}

func (cfg *apiConfig) handlerRevokeRefToken(resWriter http.ResponseWriter, req *http.Request) {
	refTokenString, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(resWriter, "Refresh token not provided", http.StatusUnauthorized, nil)
		return
	}

	err = cfg.db.RevokeRefreshToken(req.Context(), refTokenString)
	if err != nil {
		respondWithError(resWriter, "issue revoking provided token", http.StatusInternalServerError, err)
		return
	}
	respondWithJson(resWriter, http.StatusNoContent, struct{}{})
}

func (cfg *apiConfig) handlerUpdateUser(resWriter http.ResponseWriter, req *http.Request) {
	TokenString, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(resWriter, "token not provided", http.StatusUnauthorized, nil)
		return
	}

	userUUID, err := auth.ValidateJWT(TokenString, cfg.secret)
	if err != nil {
		respondWithError(resWriter, "Invalid token", http.StatusUnauthorized, nil)
		return
	}

	type incoming struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	userinfo := incoming{}
	decoder := json.NewDecoder(req.Body)
	err = decoder.Decode(&userinfo)
	if err != nil {
		log.Printf("Error decoding json data in request: %v\n", err)
		respondWithError(resWriter, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	if userinfo.Email == "" || userinfo.Password == "" {
		respondWithError(resWriter, "Provided an empty string for username or password", 400, nil)
		return
	}

	hashPW, err := auth.HashPassword(userinfo.Password)
	if err != nil {
		respondWithError(resWriter, "issue hashing password", http.StatusInternalServerError, err)
		return
	}

	updatedUser, err := cfg.db.UpdateUser(req.Context(), database.UpdateUserParams{
		HashedPassword: sql.NullString{String: hashPW, Valid: true},
		Email:          sql.NullString{String: userinfo.Email, Valid: true},
		ID:             userUUID,
	})
	if err != nil {
		respondWithError(resWriter, "issue updating user info in database", http.StatusInternalServerError, err)
		return
	}
	respondWithJson(resWriter, http.StatusOK, User{
		ID:        updatedUser.ID,
		CreatedAt: updatedUser.CreatedAt.Time,
		UpdatedAt: updatedUser.UpdatedAt.Time,
		Email:     updatedUser.Email.String,
	})
}

func (cfg *apiConfig) handlerDeleteChirp(resWriter http.ResponseWriter, req *http.Request) {
	TokenString, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(resWriter, "token not provided", http.StatusUnauthorized, nil)
		return
	}

	userUUID, err := auth.ValidateJWT(TokenString, cfg.secret)
	if err != nil {
		respondWithError(resWriter, "Invalid token", http.StatusUnauthorized, nil)
		return
	}

	stringid := req.PathValue("chirpID")
	convertedID, err := uuid.Parse(stringid)
	if err != nil {
		respondWithError(resWriter, "chirp id provided is not a valid UUID", http.StatusBadRequest, nil)
		return
	}

	dbChirp, err := cfg.db.GetSingleChirp(req.Context(), convertedID)
	if err != nil {
		respondWithError(resWriter, "Chirp not found", http.StatusNotFound, err)
		return
	}

	if userUUID != dbChirp.UserID.UUID {
		respondWithError(resWriter, "You are not the author of this chirp", http.StatusForbidden, nil)
		return
	}

	err = cfg.db.DeleteSingleChirp(req.Context(), convertedID)
	if err != nil {
		respondWithError(resWriter, "issue deleting provided chirp", http.StatusInternalServerError, err)
		return
	}
	respondWithJson(resWriter, http.StatusNoContent, struct{}{})

}

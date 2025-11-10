package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {
	params := &argon2id.Params{
		Memory:      128 * 1024,
		Iterations:  4,
		Parallelism: uint8(runtime.NumCPU()),
		SaltLength:  16,
		KeyLength:   32,
	}
	hash, err := argon2id.CreateHash(password, params)
	if err != nil {
		return "", err
	}
	return hash, nil
}

func CheckPasswordHash(password, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, err
	}
	if match {
		return true, nil
	}
	return false, nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string) (string, error) {
	now := &jwt.NumericDate{Time: time.Now().UTC()}
	later := &jwt.NumericDate{Time: time.Now().UTC().Add(3600 * time.Second)}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt:  now,
		ExpiresAt: later,
		Subject:   userID.String(),
	})
	// log.Printf("Issued at: %v, Expires At: %v", now, later)
	tokenString, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", fmt.Errorf("couldn't sign string with provided secret: %v", err)
	}
	return tokenString, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	parsedToken, err := jwt.ParseWithClaims(
		tokenString,
		&jwt.RegisteredClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(tokenSecret), nil
		},
	)
	if err != nil {
		log.Printf("issue parsing token string: %v", err)
		return uuid.Nil, err
	}
	if parsedToken.Valid {
		uuidString, err := parsedToken.Claims.GetSubject()
		if err != nil {
			log.Printf("issue getting subject from parse token: %v", err)
			return uuid.Nil, err
		}
		validUUID, err := uuid.Parse(uuidString)
		if err != nil {
			log.Printf("issue parsing uuid string: %v", err)
			return uuid.Nil, err
		}
		return validUUID, nil
	}
	return uuid.Nil, fmt.Errorf("invalid token")
}

func GetBearerToken(headers http.Header) (string, error) {
	authHeader, exist := headers["Authorization"]
	if !exist {
		return "", fmt.Errorf("no auth info")
	}
	return strings.TrimPrefix(authHeader[0], "Bearer "), nil
}

func MakeRefreshToken() (string, error) {
	randomData := make([]byte, 32)
	_, err := rand.Read(randomData)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(randomData), nil
}

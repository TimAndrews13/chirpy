package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	return hash, nil
}

func CheckPasswordHash(password, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		log.Fatal(err)
		return false, err
	}
	return match, nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	signingKey := []byte(tokenSecret)

	currentTime := time.Now().UTC()
	expireTime := currentTime.Add(expiresIn)
	registeredClaim := jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  jwt.NewNumericDate(currentTime),
		ExpiresAt: jwt.NewNumericDate(expireTime),
		Subject:   userID.String(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, registeredClaim)

	return token.SignedString(signingKey)
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&jwt.RegisteredClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(tokenSecret), nil
		},
	)
	if err != nil {
		return uuid.Nil, err
	}
	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return uuid.Nil, errors.New("invalid token claims")
	}
	subjectUUID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, errors.New("error parsing uuid")
	}
	return subjectUUID, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("authorizaiton header not found\n")
	}
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimSpace(authHeader[7:])
		if token == "" {
			return "", errors.New("token is missing from authorization header\n")
		}
		return token, nil
	}
	return "", errors.New("unsupported or invalid authorization scheme")
}

func MakeRefreshToken() string {
	b := make([]byte, 32)
	n, err := rand.Read(b)
	if err != nil {
		log.Printf("error while generating random bytes: %v", err)
		return ""
	}
	if n != 32 {
		log.Printf("expected to read 32 bytes, but read %d bytes", n)
		return ""
	}
	encodedString := hex.EncodeToString(b)
	return encodedString
}

func GetAPIKey(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("authorizaiton header not found\n")
	}
	if strings.HasPrefix(authHeader, "ApiKey ") {
		token := strings.TrimSpace(authHeader[7:])
		if token == "" {
			return "", errors.New("token is missing from authorization header\n")
		}
		return token, nil
	}
	return "", errors.New("unsupported or invalid authorization scheme")
}

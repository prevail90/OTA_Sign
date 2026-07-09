package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid launch token")
	ErrExpiredToken = errors.New("expired launch token")
)

func SignLaunchClaims(claims LaunchClaims, secret string) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := sign(encodedPayload, secret)

	return encodedPayload + "." + signature, nil
}

func VerifyLaunchToken(token string, secret string, now time.Time) (LaunchClaims, error) {
	var claims LaunchClaims

	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return claims, ErrInvalidToken
	}

	expected := sign(parts[0], secret)
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return claims, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return claims, ErrInvalidToken
	}

	if err := json.Unmarshal(payload, &claims); err != nil {
		return claims, ErrInvalidToken
	}

	if claims.ExpiresAt.Before(now) {
		return claims, ErrExpiredToken
	}

	return claims, nil
}

func sign(encodedPayload string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

package siloplugin

import (
	"aura/database"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// DownloadToken is what a download URL is for, Silo fetches the URL without any other auth
type DownloadToken struct {
	TMDB_ID       string    `json:"t"`
	ItemType      string    `json:"k"`
	SetID         string    `json:"s"`
	ImageID       string    `json:"i"`
	ImageType     string    `json:"y"`
	Modified      time.Time `json:"m"`
	SeasonNumber  *int      `json:"sn,omitempty"`
	EpisodeNumber *int      `json:"en,omitempty"`
	Creator       string    `json:"c"`
	Refresh       bool      `json:"r,omitempty"` // From a refresh, only a favorite creator's set is queued
}

// Set in tests
var getAuthSecret = func(ctx context.Context) (string, error) {
	secret, Err := database.GetAuthTokenSecret(ctx)
	if Err.Message != "" {
		return "", errors.New(Err.Message)
	}
	return secret, nil
}

// signingKey is a key only for download tokens, made from aura's auth secret
func signingKey(ctx context.Context) ([]byte, error) {
	secret, err := getAuthSecret(ctx)
	if err != nil {
		return nil, err
	}
	if secret == "" {
		return nil, errors.New("aura has no auth secret yet")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("aura silo plugin download"))
	return mac.Sum(nil), nil
}

// SignDownloadToken makes the token part of a download URL
func SignDownloadToken(ctx context.Context, token DownloadToken) (string, error) {
	key, err := signingKey(ctx)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// VerifyDownloadToken checks the signature and returns what the token is for
func VerifyDownloadToken(ctx context.Context, signed string) (DownloadToken, error) {
	encoded, signature, found := strings.Cut(signed, ".")
	if !found {
		return DownloadToken{}, errors.New("malformed download token")
	}
	key, err := signingKey(ctx)
	if err != nil {
		return DownloadToken{}, err
	}
	got, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return DownloadToken{}, errors.New("malformed download token")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(encoded))
	if !hmac.Equal(got, mac.Sum(nil)) {
		return DownloadToken{}, errors.New("invalid download token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return DownloadToken{}, errors.New("malformed download token")
	}
	var token DownloadToken
	if err := json.Unmarshal(payload, &token); err != nil {
		return DownloadToken{}, errors.New("malformed download token")
	}
	return token, nil
}

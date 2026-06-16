package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"openwatcher/internal/config"
)

var ErrAccessTokenMissing = errors.New("codex access token missing")

type Credentials struct {
	AccessToken string
	AccountID   string
}

type fileShape struct {
	Tokens struct {
		AccessToken string `json:"access_token"`
		AccountID   string `json:"account_id"`
	} `json:"tokens"`
	AccessToken string `json:"access_token"`
	AccountID   string `json:"account_id"`
}

func ReadCredentials(codexHome string) (Credentials, error) {
	resolvedHome, err := config.ResolveCodexHome(codexHome)
	if err != nil {
		return Credentials{}, err
	}
	path := filepath.Join(resolvedHome, "auth.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Credentials{}, err
	}

	var file fileShape
	if err := json.Unmarshal(data, &file); err != nil {
		return Credentials{}, err
	}

	creds := Credentials{
		AccessToken: file.Tokens.AccessToken,
		AccountID:   file.Tokens.AccountID,
	}
	if creds.AccessToken == "" {
		creds.AccessToken = file.AccessToken
	}
	if creds.AccountID == "" {
		creds.AccountID = file.AccountID
	}
	if creds.AccessToken == "" {
		return Credentials{}, ErrAccessTokenMissing
	}
	return creds, nil
}

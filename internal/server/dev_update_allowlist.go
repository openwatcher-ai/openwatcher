package server

import (
	"net/http"
	"strings"

	"openwatcher/internal/pairing"
)

func (a *App) devUpdateAuthorized(r *http.Request) bool {
	a.mu.RLock()
	expectedTokenHash := strings.TrimSpace(a.cfg.TokenHashForSlot(a.pairingSlot))
	allowlistPath := pairing.ResolveRelativeToConfig(a.configPath, strings.TrimSpace(a.cfg.DevUpdateAllowlist))
	a.mu.RUnlock()
	if expectedTokenHash == "" || allowlistPath == "" {
		return false
	}
	token := strings.TrimSpace(watcherHeaderValue(r.Header, "Token"))
	if !pairing.VerifyTokenHash(token, expectedTokenHash) {
		return false
	}
	tokenHash := pairing.HashToken(token)
	entries, err := pairing.LoadAllowlist(allowlistPath)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if strings.EqualFold(entry, tokenHash) {
			return true
		}
	}
	return false
}

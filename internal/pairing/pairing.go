package pairing

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

const MinimumTokenLength = 32

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func VerifyTokenHash(token, expectedHash string) bool {
	if token == "" || expectedHash == "" {
		return false
	}
	actual := HashToken(token)
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expectedHash)) == 1
}

func IsUsableToken(token string) bool {
	return len(strings.TrimSpace(token)) >= MinimumTokenLength
}

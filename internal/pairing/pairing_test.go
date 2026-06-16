package pairing

import "testing"

func TestHashAndVerifyToken(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	hash := HashToken(token)

	if hash == token {
		t.Fatal("hash must not store the plain token")
	}
	if !VerifyTokenHash(token, hash) {
		t.Fatal("expected token to verify")
	}
	if VerifyTokenHash("wrong-token-0123456789abcdef0123", hash) {
		t.Fatal("wrong token verified")
	}
}

func TestIsUsableToken(t *testing.T) {
	if IsUsableToken("short") {
		t.Fatal("short token should be rejected")
	}
	if !IsUsableToken("0123456789abcdef0123456789abcdef") {
		t.Fatal("32 byte token should be accepted")
	}
}

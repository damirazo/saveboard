package identity

import (
	"encoding/json"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	for _, password := range []string{"short", strings.Repeat("x", 73), strings.Repeat("я", 37)} {
		if _, err := HashPassword(password); err == nil {
			t.Fatal("invalid password length accepted")
		}
	}
	hash, err := HashPassword("a valid password")
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("a valid password")); err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("wrong password")); err == nil {
		t.Fatal("wrong password accepted")
	}
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil || cost != 12 {
		t.Fatal("incorrect bcrypt work factor")
	}
}

func TestCredentialModelsExcludeSecrets(t *testing.T) {
	secret := "do-not-serialize"
	models := []any{
		User{PasswordHash: &secret},
		Session{TokenHash: []byte(secret)},
		RefreshToken{TokenHash: []byte(secret)},
		UserToken{TokenHash: []byte(secret)},
		OAuthIdentity{AccessTokenEncrypted: []byte(secret), RefreshTokenEncrypted: []byte(secret)},
		OAuthFlow{StateHash: []byte(secret), BrowserTokenHash: []byte(secret), NonceHash: []byte(secret), PKCEVerifierEncrypted: []byte(secret)},
	}
	for _, model := range models {
		b, err := json.Marshal(model)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(b, &fields); err != nil {
			t.Fatal(err)
		}
		for name := range fields {
			if strings.Contains(name, "hash") || strings.Contains(name, "encrypted") || strings.Contains(name, "password") {
				t.Fatalf("secret field serialized: %s", name)
			}
		}
	}
}

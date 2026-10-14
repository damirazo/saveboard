package identity

import (
	"strings"
	"testing"
)

func TestRegistrationValidation(t *testing.T) {
	for _, test := range []struct{ username, email, password string }{
		{"ab", "a@example.com", "password"}, {"bad@name", "a@example.com", "password"},
		{"valid", "Name <a@example.com>", "password"}, {"valid", "a @example.com", "password"},
		{"valid", "not-email", "password"}, {"valid", "a@example.com", "short"},
		{"valid", "a@example.com", strings.Repeat("я", 37)}, {"valid", "a@example.com", "password\x00"},
	} {
		if validateRegistration(test.username, test.email, test.password) == nil {
			t.Errorf("accepted invalid registration: username=%q email=%q", test.username, test.email)
		}
	}
	if err := validateRegistration("Test_1", "test@example.com", "Надёжный-пароль"); err != nil {
		t.Fatal(err)
	}
}

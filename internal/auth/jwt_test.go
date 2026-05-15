package auth

import (
	"strings"
	"testing"
	"time"
)

func TestTokenManagerSignAndVerify(t *testing.T) {
	manager := NewTokenManager("secret", "localweb")
	manager.now = func() time.Time {
		return time.Unix(100, 0)
	}

	token, err := manager.Sign("/abc", time.Hour)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	claims, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if claims.RoutePath != "/abc" {
		t.Fatalf("RoutePath = %q, want /abc", claims.RoutePath)
	}
}

func TestTokenManagerRejectsTamperedToken(t *testing.T) {
	manager := NewTokenManager("secret", "localweb")
	token, err := manager.Sign("/abc", time.Hour)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	tampered := token[:len(token)-1] + "x"
	if _, err := manager.Verify(tampered); err == nil {
		t.Fatal("Verify returned nil error for tampered token")
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	manager := NewTokenManager("secret", "localweb")
	manager.now = func() time.Time {
		return time.Unix(100, 0)
	}
	token, err := manager.Sign("/abc", time.Second)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	manager.now = func() time.Time {
		return time.Unix(102, 0)
	}
	if _, err := manager.Verify(token); err != ErrExpiredToken {
		t.Fatalf("Verify error = %v, want ErrExpiredToken", err)
	}
}

func TestCheckPassword(t *testing.T) {
	if !CheckPassword("pw", "pw", "") {
		t.Fatal("plain password should match")
	}
	if CheckPassword("bad", "pw", "") {
		t.Fatal("bad plain password should not match")
	}
	hash := SHA256Hex("pw")
	if !CheckPassword("pw", "", hash) {
		t.Fatal("sha256 password should match")
	}
	if strings.Contains(hash, "pw") {
		t.Fatal("hash should not contain password")
	}
}

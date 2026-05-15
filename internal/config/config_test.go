package config

import (
	"testing"
	"time"
)

func TestParseAppliesDefaultsAndSortsRouters(t *testing.T) {
	cfg, err := Parse([]byte(`{
		"port": 8080,
		"jwt": {"secret": "secret"},
		"routers": [
			{"path": "/abc", "port": 10203, "password": "pw"},
			{"path": "/abc/long", "port": 10204, "password": "pw"}
		]
	}`))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.BindHost != DefaultBindHost {
		t.Fatalf("BindHost = %q, want %q", cfg.BindHost, DefaultBindHost)
	}
	if cfg.JWT.CookieName != DefaultCookieName {
		t.Fatalf("CookieName = %q, want %q", cfg.JWT.CookieName, DefaultCookieName)
	}
	if cfg.TokenTTL() != time.Duration(DefaultTokenTTLSeconds)*time.Second {
		t.Fatalf("TokenTTL = %s", cfg.TokenTTL())
	}
	if got := cfg.Routers[0].Path; got != "/abc/long" {
		t.Fatalf("first router path = %q, want longest path first", got)
	}
	if !cfg.Routers[0].WebSocketEnabled() {
		t.Fatalf("websocket should be enabled by default")
	}
}

func TestParseAcceptsLegacyRouterAlias(t *testing.T) {
	cfg, err := Parse([]byte(`{
		"jwt": {"secret": "secret"},
		"router": [
			{
				"path": "/abc",
				"port": 10203,
				"password": "pw",
				"httpheader": "X-Test: ok"
			}
		]
	}`))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(cfg.Routers) != 1 {
		t.Fatalf("router count = %d, want 1", len(cfg.Routers))
	}
	if got := cfg.Routers[0].HTTPHeaders["X-Test"]; got != "ok" {
		t.Fatalf("legacy header = %q, want ok", got)
	}
}

func TestParseRejectsDuplicatePaths(t *testing.T) {
	_, err := Parse([]byte(`{
		"jwt": {"secret": "secret"},
		"routers": [
			{"path": "/abc", "port": 10203, "password": "pw"},
			{"path": "/abc/", "port": 10204, "password": "pw"}
		]
	}`))
	if err == nil {
		t.Fatal("Parse returned nil error for duplicate normalized paths")
	}
}

func TestParseRejectsInvalidPasswordSHA256(t *testing.T) {
	_, err := Parse([]byte(`{
		"jwt": {"secret": "secret"},
		"routers": [
			{"path": "/abc", "port": 10203, "password_sha256": "bad"}
		]
	}`))
	if err == nil {
		t.Fatal("Parse returned nil error for invalid password_sha256")
	}
}

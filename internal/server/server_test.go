package server

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"localweb/internal/config"
)

func TestProtectedRouteLoginAndProxy(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-Path", r.URL.Path)
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer backend.Close()

	host, port := splitTestServerURL(t, backend.URL)
	cfg := testConfig(host, port)
	handler, err := New(cfg, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	gateway := httptest.NewServer(handler)
	defer gateway.Close()

	unauthorized, err := http.Get(gateway.URL + "/abc/api")
	if err != nil {
		t.Fatalf("GET unauthorized route: %v", err)
	}
	defer unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", unauthorized.StatusCode)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Post(
		gateway.URL+config.DefaultLoginPath,
		"application/x-www-form-urlencoded",
		strings.NewReader("route=%2Fabc&next=%2Fabc%2Fapi&password=pw"),
	)
	if err != nil {
		t.Fatalf("POST login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login status = %d, want 303", resp.StatusCode)
	}
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("login response did not set cookie")
	}
	if cookies[0].Path != "/abc" {
		t.Fatalf("cookie path = %q, want /abc", cookies[0].Path)
	}

	req, err := http.NewRequest(http.MethodGet, gateway.URL+"/abc/api", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.AddCookie(cookies[0])
	proxied, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET proxied route: %v", err)
	}
	defer proxied.Body.Close()
	body, err := io.ReadAll(proxied.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if proxied.StatusCode != http.StatusOK {
		t.Fatalf("proxied status = %d, want 200", proxied.StatusCode)
	}
	if got := string(body); got != "/api" {
		t.Fatalf("backend path = %q, want /api", got)
	}
}

func TestRouteRootRedirectsToTrailingSlash(t *testing.T) {
	cfg := testConfig("127.0.0.1", 1)
	handler, err := New(cfg, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.test/abc?x=1", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", recorder.Code)
	}
	if got := recorder.Header().Get("Location"); got != "/abc/?x=1" {
		t.Fatalf("Location = %q, want /abc/?x=1", got)
	}
}

func TestRewriteResponseHeaders(t *testing.T) {
	header := http.Header{}
	header.Set("Location", "/login?next=/")
	header.Add("Set-Cookie", "sid=1; Path=/; HttpOnly")
	resp := &http.Response{Header: header}

	rewriteResponseHeaders(resp, config.RouterConfig{
		Path:            "/abc",
		StripPathPrefix: true,
	}, "127.0.0.1:10203")

	if got := resp.Header.Get("Location"); got != "/abc/login?next=/" {
		t.Fatalf("Location = %q, want /abc/login?next=/", got)
	}
	if got := resp.Header.Values("Set-Cookie")[0]; got != "sid=1; Path=/abc; HttpOnly" {
		t.Fatalf("Set-Cookie = %q, want rewritten path", got)
	}
}

func TestRouteMatchesBoundary(t *testing.T) {
	tests := []struct {
		route string
		path  string
		want  bool
	}{
		{"/abc", "/abc", true},
		{"/abc", "/abc/api", true},
		{"/abc", "/abc123", false},
		{"/", "/anything", true},
	}

	for _, tt := range tests {
		if got := routeMatches(tt.route, tt.path); got != tt.want {
			t.Fatalf("routeMatches(%q, %q) = %v, want %v", tt.route, tt.path, got, tt.want)
		}
	}
}

func splitTestServerURL(t *testing.T, rawURL string) (string, int) {
	t.Helper()
	hostPort := strings.TrimPrefix(rawURL, "http://")
	host, portString, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", hostPort, err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatalf("Atoi(%q): %v", portString, err)
	}
	return host, port
}

func testConfig(host string, port int) *config.Config {
	websocket := true
	cfg, err := config.Parse([]byte(`{
		"port": 8080,
		"jwt": {"secret": "secret"},
		"routers": [
			{
				"path": "/abc",
				"port": 1,
				"password": "pw",
				"strip_path_prefix": true,
				"websocket": true
			}
		]
	}`))
	if err != nil {
		panic(err)
	}
	cfg.Routers[0].TargetHost = host
	cfg.Routers[0].Port = port
	cfg.Routers[0].WebSocket = &websocket
	return cfg
}

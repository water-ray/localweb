package server

import (
	"errors"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"localweb/internal/auth"
	"localweb/internal/config"
)

var loginTemplate = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Login</title>
<style>
body{font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;margin:0;min-height:100vh;display:grid;place-items:center;background:#f5f7fb;color:#172033}
main{width:min(360px,calc(100vw - 32px));background:#fff;border:1px solid #d9e0ea;border-radius:8px;padding:24px;box-shadow:0 10px 30px rgba(15,23,42,.08)}
h1{font-size:20px;margin:0 0 16px}
label{display:block;font-size:14px;margin-bottom:8px;color:#526071}
input[type=password]{width:100%;box-sizing:border-box;border:1px solid #bac4d0;border-radius:6px;padding:10px 12px;font-size:16px}
button{width:100%;margin-top:16px;border:0;border-radius:6px;background:#1f6feb;color:#fff;padding:10px 12px;font-size:15px;cursor:pointer}
.error{margin:0 0 14px;color:#b42318;font-size:14px}
.path{margin:0 0 14px;color:#526071;font-size:13px;word-break:break-all}
</style>
</head>
<body>
<main>
<h1>Login</h1>
{{if .Error}}<p class="error">{{.Error}}</p>{{end}}
<p class="path">{{.RoutePath}}</p>
<form method="post" action="{{.LoginPath}}">
<input type="hidden" name="route" value="{{.RoutePath}}">
<input type="hidden" name="next" value="{{.Next}}">
<label for="password">Password</label>
<input id="password" name="password" type="password" autocomplete="current-password" autofocus required>
<button type="submit">Continue</button>
</form>
</main>
</body>
</html>`))

type Handler struct {
	cfg     *config.Config
	tokens  *auth.TokenManager
	proxies map[string]*httputil.ReverseProxy
	limiter *loginLimiter
	logger  *log.Logger
}

type loginPageData struct {
	LoginPath string
	RoutePath string
	Next      string
	Error     string
}

func New(cfg *config.Config, logger *log.Logger) (*Handler, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	if logger == nil {
		logger = log.Default()
	}

	handler := &Handler{
		cfg:     cfg,
		tokens:  auth.NewTokenManager(cfg.JWT.Secret, cfg.JWT.Issuer),
		proxies: make(map[string]*httputil.ReverseProxy, len(cfg.Routers)),
		limiter: newLoginLimiter(cfg.Security.RateLimitPerMinute),
		logger:  logger,
	}

	for _, route := range cfg.Routers {
		routeCopy := route
		handler.proxies[route.Path] = newReverseProxy(routeCopy, logger)
	}
	return handler, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == h.cfg.Security.LoginPath {
		h.serveLogin(w, r)
		return
	}

	route := h.matchRoute(r.URL.Path)
	if route == nil {
		http.NotFound(w, r)
		return
	}

	if shouldRedirectToRouteSlash(r, route) {
		redirectToRouteSlash(w, r, route)
		return
	}

	if isWebSocketRequest(r) && !route.WebSocketEnabled() {
		http.Error(w, "websocket is disabled for this route", http.StatusForbidden)
		return
	}

	if !h.isAuthorized(r, route) {
		h.serveUnauthorized(w, r, route)
		return
	}

	proxy := h.proxies[route.Path]
	if proxy == nil {
		http.Error(w, "proxy is not configured", http.StatusBadGateway)
		return
	}
	proxy.ServeHTTP(w, r)
}

func (h *Handler) serveLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		route := h.findRouteByPath(r.URL.Query().Get("route"))
		if route == nil {
			next := sanitizeNext(r.URL.Query().Get("next"), "")
			route = h.matchRoute(next)
		}
		if route == nil {
			http.NotFound(w, r)
			return
		}
		h.renderLogin(w, r, route, sanitizeNext(r.URL.Query().Get("next"), route.Path), "", http.StatusOK)
	case http.MethodPost:
		h.handleLoginPost(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	route := h.findRouteByPath(r.Form.Get("route"))
	if route == nil {
		http.Error(w, "unknown route", http.StatusBadRequest)
		return
	}
	next := sanitizeNext(r.Form.Get("next"), route.Path)

	limitKey := clientIP(r) + "|" + route.Path
	if !h.limiter.Allow(limitKey) {
		h.renderLogin(w, r, route, next, "too many login attempts", http.StatusTooManyRequests)
		return
	}

	password := r.Form.Get("password")
	if !auth.CheckPassword(password, route.Password, route.PasswordSHA256) {
		h.renderLogin(w, r, route, next, "invalid password", http.StatusUnauthorized)
		return
	}

	token, err := h.tokens.Sign(route.Path, h.cfg.TokenTTL())
	if err != nil {
		h.logger.Printf("sign token for %s: %v", route.Path, err)
		http.Error(w, "failed to create token", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.JWT.CookieName,
		Value:    token,
		Path:     route.Path,
		HttpOnly: true,
		Secure:   h.cfg.Security.CookieSecure,
		SameSite: sameSiteMode(h.cfg.Security.CookieSameSite),
		Expires:  time.Now().Add(h.cfg.TokenTTL()),
		MaxAge:   int(h.cfg.TokenTTL().Seconds()),
	})
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (h *Handler) serveUnauthorized(w http.ResponseWriter, r *http.Request, route *config.RouterConfig) {
	if isWebSocketRequest(r) || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	h.renderLogin(w, r, route, r.URL.RequestURI(), "", http.StatusUnauthorized)
}

func (h *Handler) renderLogin(w http.ResponseWriter, r *http.Request, route *config.RouterConfig, next string, message string, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	err := loginTemplate.Execute(w, loginPageData{
		LoginPath: h.cfg.Security.LoginPath,
		RoutePath: route.Path,
		Next:      sanitizeNext(next, route.Path),
		Error:     message,
	})
	if err != nil {
		h.logger.Printf("render login page: %v", err)
	}
}

func (h *Handler) isAuthorized(r *http.Request, route *config.RouterConfig) bool {
	token := ""
	if cookie, err := r.Cookie(h.cfg.JWT.CookieName); err == nil {
		token = cookie.Value
	}
	if token == "" {
		token = auth.BearerToken(r.Header.Get("Authorization"))
	}
	if token == "" {
		return false
	}

	claims, err := h.tokens.Verify(token)
	if err != nil {
		return false
	}
	return claims.RoutePath == route.Path
}

func (h *Handler) matchRoute(requestPath string) *config.RouterConfig {
	for i := range h.cfg.Routers {
		route := &h.cfg.Routers[i]
		if routeMatches(route.Path, requestPath) {
			return route
		}
	}
	return nil
}

func (h *Handler) findRouteByPath(routePath string) *config.RouterConfig {
	for i := range h.cfg.Routers {
		route := &h.cfg.Routers[i]
		if route.Path == routePath {
			return route
		}
	}
	return nil
}

func newReverseProxy(route config.RouterConfig, logger *log.Logger) *httputil.ReverseProxy {
	targetHost := net.JoinHostPort(route.TargetHost, strconv.Itoa(route.Port))
	timeout := time.Duration(route.TimeoutSeconds) * time.Second
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = timeout
	transport.DialContext = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext

	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			originalHost := req.Host
			originalURI := req.URL.RequestURI()
			req.URL.Scheme = "http"
			req.URL.Host = targetHost
			req.Host = targetHost
			if route.StripPathPrefix {
				req.URL.Path = stripRoutePrefix(req.URL.Path, route.Path)
				req.URL.RawPath = ""
			}
			req.Header.Set("X-Forwarded-Host", originalHost)
			req.Header.Set("X-Forwarded-Uri", originalURI)
			req.Header.Set("X-Forwarded-Prefix", route.Path)
			for name, value := range route.HTTPHeaders {
				req.Header.Set(name, value)
			}
		},
		Transport:     transport,
		FlushInterval: 100 * time.Millisecond,
		ModifyResponse: func(resp *http.Response) error {
			rewriteResponseHeaders(resp, route, targetHost)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			logger.Printf("proxy %s -> %s failed: %v", r.URL.Path, targetHost, err)
			http.Error(w, "bad gateway", http.StatusBadGateway)
		},
	}
}

func routeMatches(routePath string, requestPath string) bool {
	if routePath == "/" {
		return strings.HasPrefix(requestPath, "/")
	}
	return requestPath == routePath || strings.HasPrefix(requestPath, routePath+"/")
}

func stripRoutePrefix(requestPath string, routePath string) string {
	if routePath == "/" {
		if requestPath == "" {
			return "/"
		}
		return requestPath
	}
	trimmed := strings.TrimPrefix(requestPath, routePath)
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}
	return trimmed
}

func shouldRedirectToRouteSlash(r *http.Request, route *config.RouterConfig) bool {
	if !route.StripPathPrefix || !route.ForceTrailingSlashEnabled() {
		return false
	}
	if r.URL.Path != route.Path {
		return false
	}
	return r.Method == http.MethodGet || r.Method == http.MethodHead
}

func redirectToRouteSlash(w http.ResponseWriter, r *http.Request, route *config.RouterConfig) {
	target := route.Path + "/"
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}

func sanitizeNext(next string, fallback string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		if fallback == "" {
			return "/"
		}
		return fallback
	}
	if parsed, err := url.Parse(next); err == nil && parsed.IsAbs() {
		if fallback == "" {
			return "/"
		}
		return fallback
	}
	return next
}

func isWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") && headerContainsToken(r.Header.Get("Connection"), "upgrade")
}

func headerContainsToken(header string, token string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}

func rewriteResponseHeaders(resp *http.Response, route config.RouterConfig, targetHost string) {
	if route.StripPathPrefix && route.RewriteRedirectsEnabled() {
		rewriteHeaderURL(resp.Header, "Location", route.Path, targetHost)
		rewriteHeaderURL(resp.Header, "Content-Location", route.Path, targetHost)
	}
	if route.StripPathPrefix && route.RewriteCookiePathEnabled() {
		rewriteSetCookiePath(resp.Header, route.Path)
	}
}

func rewriteHeaderURL(header http.Header, name string, routePath string, targetHost string) {
	value := header.Get(name)
	if value == "" {
		return
	}

	rewritten, ok := rewriteProxyURL(value, routePath, targetHost)
	if ok {
		header.Set(name, rewritten)
	}
}

func rewriteProxyURL(value string, routePath string, targetHost string) (string, bool) {
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return prefixPath(routePath, value), true
	}

	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() {
		return "", false
	}
	if parsed.Host != targetHost || parsed.Scheme != "http" {
		return "", false
	}
	parsed.Scheme = ""
	parsed.Host = ""
	parsed.Path = prefixPath(routePath, parsed.Path)
	return parsed.String(), true
}

func prefixPath(routePath string, targetPath string) string {
	if routePath == "/" {
		if targetPath == "" {
			return "/"
		}
		return targetPath
	}
	if targetPath == "" || targetPath == "/" {
		return routePath + "/"
	}
	if strings.HasPrefix(targetPath, routePath+"/") || targetPath == routePath {
		return targetPath
	}
	return routePath + targetPath
}

func rewriteSetCookiePath(header http.Header, routePath string) {
	values := header.Values("Set-Cookie")
	if len(values) == 0 {
		return
	}
	header.Del("Set-Cookie")
	for _, value := range values {
		header.Add("Set-Cookie", rewriteOneSetCookiePath(value, routePath))
	}
}

func rewriteOneSetCookiePath(value string, routePath string) string {
	parts := strings.Split(value, ";")
	for i, part := range parts {
		name, cookiePath, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || !strings.EqualFold(name, "Path") {
			continue
		}
		if cookiePath == "" || cookiePath == "/" {
			parts[i] = " Path=" + routePath
			return strings.Join(parts, ";")
		}
		if strings.HasPrefix(cookiePath, routePath+"/") || cookiePath == routePath {
			return strings.Join(parts, ";")
		}
		if strings.HasPrefix(cookiePath, "/") {
			parts[i] = " Path=" + prefixPath(routePath, cookiePath)
			return strings.Join(parts, ";")
		}
	}
	return value
}

func sameSiteMode(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax", "":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteLaxMode
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

type loginLimiter struct {
	mu      sync.Mutex
	limit   int
	now     func() time.Time
	buckets map[string]*loginBucket
}

type loginBucket struct {
	windowStart time.Time
	count       int
}

func newLoginLimiter(limit int) *loginLimiter {
	return &loginLimiter{
		limit:   limit,
		now:     time.Now,
		buckets: make(map[string]*loginBucket),
	}
}

func (l *loginLimiter) Allow(key string) bool {
	if l.limit < 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	bucket := l.buckets[key]
	if bucket == nil || now.Sub(bucket.windowStart) >= time.Minute {
		l.buckets[key] = &loginBucket{windowStart: now, count: 1}
		return true
	}
	if bucket.count >= l.limit {
		return false
	}
	bucket.count++
	return true
}

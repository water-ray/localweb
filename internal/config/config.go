package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBindHost           = "0.0.0.0"
	DefaultPort               = 80
	DefaultTokenTTLSeconds    = 7 * 24 * 60 * 60
	DefaultCookieName         = "localweb_token"
	DefaultIssuer             = "localweb"
	DefaultLoginPath          = "/_localweb/login"
	DefaultCookieSameSite     = "Lax"
	DefaultRateLimitPerMinute = 30
	DefaultTargetHost         = "127.0.0.1"
	DefaultTimeoutSeconds     = 60
)

type Config struct {
	BindHost string         `json:"bind_host"`
	Port     int            `json:"port"`
	JWT      JWTConfig      `json:"jwt"`
	Security SecurityConfig `json:"security"`
	Routers  []RouterConfig `json:"routers"`
}

type JWTConfig struct {
	Secret     string `json:"secret"`
	SecretEnv  string `json:"secret_env"`
	TTLSeconds int64  `json:"ttl_seconds"`
	CookieName string `json:"cookie_name"`
	Issuer     string `json:"issuer"`
}

type SecurityConfig struct {
	LoginPath          string `json:"login_path"`
	CookieSecure       bool   `json:"cookie_secure"`
	CookieSameSite     string `json:"cookie_same_site"`
	RateLimitPerMinute int    `json:"rate_limit_per_minute"`
}

type RouterConfig struct {
	Name               string            `json:"name"`
	Path               string            `json:"path"`
	TargetHost         string            `json:"target_host"`
	Port               int               `json:"port"`
	Password           string            `json:"password"`
	PasswordEnv        string            `json:"password_env"`
	PasswordSHA256     string            `json:"password_sha256"`
	StripPathPrefix    bool              `json:"strip_path_prefix"`
	ForceTrailingSlash *bool             `json:"force_trailing_slash"`
	RewriteRedirects   *bool             `json:"rewrite_redirects"`
	RewriteCookiePath  *bool             `json:"rewrite_cookie_path"`
	WebSocket          *bool             `json:"websocket"`
	TimeoutSeconds     int               `json:"timeout_seconds"`
	HTTPHeaders        map[string]string `json:"http_headers"`
}

func Load(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (*Config, error) {
	var raw struct {
		BindHost string         `json:"bind_host"`
		Port     int            `json:"port"`
		JWT      JWTConfig      `json:"jwt"`
		Security SecurityConfig `json:"security"`
		Routers  []RouterConfig `json:"routers"`
		Router   []RouterConfig `json:"router"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config json: %w", err)
	}

	if len(raw.Routers) > 0 && len(raw.Router) > 0 {
		return nil, errors.New("use only one of routers or router")
	}

	cfg := &Config{
		BindHost: raw.BindHost,
		Port:     raw.Port,
		JWT:      raw.JWT,
		Security: raw.Security,
		Routers:  raw.Routers,
	}
	if len(cfg.Routers) == 0 {
		cfg.Routers = raw.Router
	}

	cfg.applyDefaults()
	if err := cfg.resolveSecrets(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cfg.sortRouters()
	return cfg, nil
}

func (r *RouterConfig) UnmarshalJSON(data []byte) error {
	type routerAlias RouterConfig
	var raw struct {
		routerAlias
		LegacyHTTPHeader json.RawMessage `json:"httpheader"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*r = RouterConfig(raw.routerAlias)
	if len(raw.LegacyHTTPHeader) > 0 && len(r.HTTPHeaders) == 0 {
		headers, err := parseLegacyHeaders(raw.LegacyHTTPHeader)
		if err != nil {
			return err
		}
		r.HTTPHeaders = headers
	}
	return nil
}

func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535: %d", c.Port)
	}
	if c.JWT.Secret == "" {
		return errors.New("jwt.secret or jwt.secret_env is required")
	}
	if c.JWT.TTLSeconds <= 0 {
		return errors.New("jwt.ttl_seconds must be greater than 0")
	}
	if !strings.HasPrefix(c.Security.LoginPath, "/") {
		return fmt.Errorf("security.login_path must start with /: %s", c.Security.LoginPath)
	}
	if strings.ContainsAny(c.Security.LoginPath, "?#") {
		return fmt.Errorf("security.login_path must not contain query or fragment: %s", c.Security.LoginPath)
	}
	if len(c.Routers) == 0 {
		return errors.New("at least one router is required")
	}

	seenPaths := make(map[string]struct{}, len(c.Routers))
	for i, router := range c.Routers {
		label := fmt.Sprintf("routers[%d]", i)
		if router.Path == "" || !strings.HasPrefix(router.Path, "/") {
			return fmt.Errorf("%s.path must start with /", label)
		}
		if strings.ContainsAny(router.Path, "?#") {
			return fmt.Errorf("%s.path must not contain query or fragment: %s", label, router.Path)
		}
		if router.Path == c.Security.LoginPath {
			return fmt.Errorf("%s.path conflicts with security.login_path", label)
		}
		if _, ok := seenPaths[router.Path]; ok {
			return fmt.Errorf("duplicate router path: %s", router.Path)
		}
		seenPaths[router.Path] = struct{}{}

		if router.Port < 1 || router.Port > 65535 {
			return fmt.Errorf("%s.port must be between 1 and 65535: %d", label, router.Port)
		}
		if router.TargetHost == "" {
			return fmt.Errorf("%s.target_host is required", label)
		}
		if router.Password == "" && router.PasswordSHA256 == "" {
			return fmt.Errorf("%s.password, password_env, or password_sha256 is required", label)
		}
		if router.PasswordSHA256 != "" {
			decoded, err := hex.DecodeString(router.PasswordSHA256)
			if err != nil || len(decoded) != sha256.Size {
				return fmt.Errorf("%s.password_sha256 must be a SHA-256 hex digest", label)
			}
		}
		if router.TimeoutSeconds <= 0 {
			return fmt.Errorf("%s.timeout_seconds must be greater than 0", label)
		}
	}
	return nil
}

func (c *Config) Address() string {
	return net.JoinHostPort(c.BindHost, strconv.Itoa(c.Port))
}

func (c *Config) TokenTTL() time.Duration {
	return time.Duration(c.JWT.TTLSeconds) * time.Second
}

func (r RouterConfig) WebSocketEnabled() bool {
	return r.WebSocket == nil || *r.WebSocket
}

func (r RouterConfig) ForceTrailingSlashEnabled() bool {
	return r.ForceTrailingSlash == nil || *r.ForceTrailingSlash
}

func (r RouterConfig) RewriteRedirectsEnabled() bool {
	return r.RewriteRedirects == nil || *r.RewriteRedirects
}

func (r RouterConfig) RewriteCookiePathEnabled() bool {
	return r.RewriteCookiePath == nil || *r.RewriteCookiePath
}

func (c *Config) applyDefaults() {
	if c.BindHost == "" {
		c.BindHost = DefaultBindHost
	}
	if c.Port == 0 {
		c.Port = DefaultPort
	}
	if c.JWT.TTLSeconds == 0 {
		c.JWT.TTLSeconds = DefaultTokenTTLSeconds
	}
	if c.JWT.CookieName == "" {
		c.JWT.CookieName = DefaultCookieName
	}
	if c.JWT.Issuer == "" {
		c.JWT.Issuer = DefaultIssuer
	}
	if c.Security.LoginPath == "" {
		c.Security.LoginPath = DefaultLoginPath
	}
	c.Security.LoginPath = normalizeHTTPPath(c.Security.LoginPath)
	if c.Security.CookieSameSite == "" {
		c.Security.CookieSameSite = DefaultCookieSameSite
	}
	if c.Security.RateLimitPerMinute == 0 {
		c.Security.RateLimitPerMinute = DefaultRateLimitPerMinute
	}

	for i := range c.Routers {
		router := &c.Routers[i]
		router.Path = normalizeHTTPPath(router.Path)
		if router.TargetHost == "" {
			router.TargetHost = DefaultTargetHost
		}
		if router.TimeoutSeconds == 0 {
			router.TimeoutSeconds = DefaultTimeoutSeconds
		}
		if router.HTTPHeaders == nil {
			router.HTTPHeaders = map[string]string{}
		}
	}
}

func (c *Config) resolveSecrets() error {
	if c.JWT.SecretEnv != "" {
		value := os.Getenv(c.JWT.SecretEnv)
		if value == "" {
			return fmt.Errorf("environment variable %s for jwt.secret is empty", c.JWT.SecretEnv)
		}
		c.JWT.Secret = value
	}

	for i := range c.Routers {
		router := &c.Routers[i]
		if router.PasswordEnv == "" {
			continue
		}
		value := os.Getenv(router.PasswordEnv)
		if value == "" {
			return fmt.Errorf("environment variable %s for routers[%d].password is empty", router.PasswordEnv, i)
		}
		router.Password = value
	}
	return nil
}

func (c *Config) sortRouters() {
	sort.SliceStable(c.Routers, func(i, j int) bool {
		return len(c.Routers[i].Path) > len(c.Routers[j].Path)
	})
}

func normalizeHTTPPath(value string) string {
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	cleaned := path.Clean(value)
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func parseLegacyHeaders(data json.RawMessage) (map[string]string, error) {
	var headers map[string]string
	if err := json.Unmarshal(data, &headers); err == nil {
		if headers == nil {
			headers = map[string]string{}
		}
		return headers, nil
	}

	var line string
	if err := json.Unmarshal(data, &line); err != nil {
		return nil, fmt.Errorf("parse legacy httpheader: %w", err)
	}
	headers = map[string]string{}
	for _, part := range strings.FieldsFunc(line, func(r rune) bool {
		return r == '\n' || r == ';'
	}) {
		name, value, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name != "" {
			headers[name] = value
		}
	}
	return headers, nil
}

package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrMalformedToken = errors.New("malformed token")
	ErrInvalidToken   = errors.New("invalid token")
	ErrExpiredToken   = errors.New("expired token")
)

type TokenManager struct {
	secret []byte
	issuer string
	now    func() time.Time
}

type Claims struct {
	RoutePath string `json:"route_path"`
	Issuer    string `json:"iss,omitempty"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func NewTokenManager(secret string, issuer string) *TokenManager {
	return &TokenManager{
		secret: []byte(secret),
		issuer: issuer,
		now:    time.Now,
	}
}

func (m *TokenManager) Sign(routePath string, ttl time.Duration) (string, error) {
	if len(m.secret) == 0 {
		return "", errors.New("jwt secret is empty")
	}
	now := m.now().UTC()
	claims := Claims{
		RoutePath: routePath,
		Issuer:    m.issuer,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}

	headerJSON, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	unsigned := header + "." + payload
	signature := m.sign(unsigned)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (m *TokenManager) Verify(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrMalformedToken
	}

	unsigned := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrMalformedToken
	}
	if !hmac.Equal(signature, m.sign(unsigned)) {
		return nil, ErrInvalidToken
	}

	headerData, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrMalformedToken
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if err := json.Unmarshal(headerData, &header); err != nil {
		return nil, ErrMalformedToken
	}
	if header.Algorithm != "HS256" || header.Type != "JWT" {
		return nil, ErrInvalidToken
	}

	payloadData, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrMalformedToken
	}
	var claims Claims
	if err := json.Unmarshal(payloadData, &claims); err != nil {
		return nil, ErrMalformedToken
	}
	if claims.RoutePath == "" || claims.ExpiresAt == 0 {
		return nil, ErrInvalidToken
	}
	if m.issuer != "" && claims.Issuer != m.issuer {
		return nil, ErrInvalidToken
	}
	if m.now().UTC().Unix() > claims.ExpiresAt {
		return nil, ErrExpiredToken
	}
	return &claims, nil
}

func (m *TokenManager) sign(unsigned string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(unsigned))
	return mac.Sum(nil)
}

func CheckPassword(candidate string, plain string, sha256Hex string) bool {
	if plain != "" {
		return constantTimeStringEqual(candidate, plain)
	}
	if sha256Hex == "" {
		return false
	}
	decoded, err := hex.DecodeString(sha256Hex)
	if err != nil || len(decoded) != sha256.Size {
		return false
	}
	sum := sha256.Sum256([]byte(candidate))
	return hmac.Equal(sum[:], decoded)
}

func constantTimeStringEqual(left string, right string) bool {
	leftSum := sha256.Sum256([]byte(left))
	rightSum := sha256.Sum256([]byte(right))
	return hmac.Equal(leftSum[:], rightSum[:])
}

func BearerToken(header string) string {
	prefix := "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

func SHA256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:])
}

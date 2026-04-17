package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type JWTUser struct {
	Subject string
	Name    string
	Email   string
}

type JWTClaims struct {
	Subject   string
	Name      string
	Email     string
	TokenType string
	Issuer    string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type JWTManager struct {
	secret    []byte
	issuer    string
	ttl       time.Duration
	tokenType string
	now       func() time.Time
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type jwtPayload struct {
	Sub   string `json:"sub"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Type  string `json:"typ_token"`
	Iss   string `json:"iss"`
	Iat   int64  `json:"iat"`
	Exp   int64  `json:"exp"`
}

func NewJWTManager(secret, issuer string, ttl time.Duration, now func() time.Time) (*JWTManager, error) {
	return NewTypedJWTManager(secret, issuer, ttl, "access", now)
}

func NewTypedJWTManager(secret, issuer string, ttl time.Duration, tokenType string, now func() time.Time) (*JWTManager, error) {
	if len(strings.TrimSpace(secret)) < 32 {
		return nil, errors.New("jwt secret must be at least 32 characters")
	}
	if strings.TrimSpace(issuer) == "" {
		return nil, errors.New("jwt issuer cannot be empty")
	}
	if ttl <= 0 {
		return nil, errors.New("jwt ttl must be > 0")
	}
	if strings.TrimSpace(tokenType) == "" {
		return nil, errors.New("jwt token type cannot be empty")
	}
	if now == nil {
		now = time.Now
	}

	return &JWTManager{
		secret:    []byte(secret),
		issuer:    issuer,
		ttl:       ttl,
		tokenType: strings.TrimSpace(tokenType),
		now:       now,
	}, nil
}

func (m *JWTManager) Generate(user JWTUser) (string, error) {
	if strings.TrimSpace(user.Subject) == "" {
		return "", errors.New("jwt subject cannot be empty")
	}

	headerBytes, err := json.Marshal(jwtHeader{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		return "", fmt.Errorf("marshal jwt header: %w", err)
	}

	now := m.now().UTC()
	payloadBytes, err := json.Marshal(jwtPayload{
		Sub:   user.Subject,
		Name:  user.Name,
		Email: user.Email,
		Type:  m.tokenType,
		Iss:   m.issuer,
		Iat:   now.Unix(),
		Exp:   now.Add(m.ttl).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("marshal jwt payload: %w", err)
	}

	headerPart := base64.RawURLEncoding.EncodeToString(headerBytes)
	payloadPart := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := headerPart + "." + payloadPart
	signature := signHS256(signingInput, m.secret)

	return signingInput + "." + signature, nil
}

func (m *JWTManager) Parse(token string) (*JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid jwt format")
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSignature := signHS256(signingInput, m.secret)
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSignature)) {
		return nil, errors.New("invalid jwt signature")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid jwt header")
	}

	var header jwtHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, errors.New("invalid jwt header")
	}
	if header.Alg != "HS256" || header.Typ != "JWT" {
		return nil, errors.New("unsupported jwt header")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid jwt payload")
	}

	var payload jwtPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, errors.New("invalid jwt payload")
	}

	if payload.Iss != m.issuer {
		return nil, errors.New("invalid jwt issuer")
	}
	if payload.Type != m.tokenType {
		return nil, errors.New("invalid jwt type")
	}
	if strings.TrimSpace(payload.Sub) == "" {
		return nil, errors.New("invalid jwt subject")
	}

	nowUnix := m.now().UTC().Unix()
	if payload.Exp <= nowUnix {
		return nil, errors.New("jwt expired")
	}
	if payload.Iat > nowUnix+30 {
		return nil, errors.New("invalid jwt issued_at")
	}

	return &JWTClaims{
		Subject:   payload.Sub,
		Name:      payload.Name,
		Email:     payload.Email,
		TokenType: payload.Type,
		Issuer:    payload.Iss,
		IssuedAt:  time.Unix(payload.Iat, 0).UTC(),
		ExpiresAt: time.Unix(payload.Exp, 0).UTC(),
	}, nil
}

func signHS256(signingInput string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func parsePositiveInt(value string) (int, error) {
	var parsed int
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a number")
		}
		parsed = parsed*10 + int(r-'0')
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("must be > 0")
	}
	return parsed, nil
}

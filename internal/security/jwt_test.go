package security

import (
	"strings"
	"testing"
	"time"
)

func TestJWTManager_GenerateAndParse(t *testing.T) {
	now := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	manager, err := NewJWTManager(strings.Repeat("a", 32), "peer-ledger", time.Hour, func() time.Time {
		return now
	})
	if err != nil {
		t.Fatalf("NewJWTManager() error: %v", err)
	}

	token, err := manager.Generate(JWTUser{
		Subject: "user-001",
		Name:    "Lucas",
		Email:   "lucas@mail.com",
	})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if claims.Subject != "user-001" {
		t.Fatalf("expected subject user-001, got %s", claims.Subject)
	}
}

func TestJWTManager_ParseExpired(t *testing.T) {
	now := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	manager, _ := NewJWTManager(strings.Repeat("a", 32), "peer-ledger", time.Minute, func() time.Time {
		return now
	})
	token, err := manager.Generate(JWTUser{Subject: "user-001"})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	expiredManager, _ := NewJWTManager(strings.Repeat("a", 32), "peer-ledger", time.Minute, func() time.Time {
		return now.Add(2 * time.Minute)
	})
	if _, err := expiredManager.Parse(token); err == nil {
		t.Fatalf("expected expired token error")
	}
}

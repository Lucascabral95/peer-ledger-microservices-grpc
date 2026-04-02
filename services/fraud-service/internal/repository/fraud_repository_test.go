package repository

import (
	"context"
	"testing"
	"time"

	"github.com/peer-ledger/services/fraud-service/internal/config"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()

	loc, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatalf("LoadLocation() error: %v", err)
	}

	return &config.Config{
		GRPCPort:                "50052",
		PerTxLimitCents:         2000000,
		DailyLimitCents:         5000000,
		VelocityMaxCount:        5,
		VelocityWindow:          10 * time.Minute,
		PairCooldown:            30 * time.Second,
		IdempotencyTTL:          24 * time.Hour,
		Timezone:                loc,
		CleanupInterval:         time.Minute,
		GracefulShutdownTimeout: 10 * time.Second,
	}
}

func newRepo(t *testing.T) *FraudRepository {
	t.Helper()
	repo, err := NewFraudRepository(testConfig(t))
	if err != nil {
		t.Fatalf("NewFraudRepository() error: %v", err)
	}
	return repo
}

func TestEvaluate_ApprovedUpdatesState(t *testing.T) {
	repo := newRepo(t)
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)

	decision, err := repo.Evaluate(EvaluateInput{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    10000,
		IdempotencyKey: "k-ok",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("Evaluate() unexpected error: %v", err)
	}
	if !decision.Allowed || decision.RuleCode != RuleCodeOK {
		t.Fatalf("expected approved decision, got %+v", decision)
	}

	dailyKey := buildDailyKey("user-001", now)
	if repo.dailyBySenderAndDate[dailyKey] != 10000 {
		t.Fatalf("expected daily usage 10000, got %d", repo.dailyBySenderAndDate[dailyKey])
	}
	if len(repo.velocityBySender["user-001"]) != 1 {
		t.Fatalf("expected one velocity entry")
	}
	if _, ok := repo.cooldownByPair[buildPairKey("user-001", "user-002")]; !ok {
		t.Fatalf("expected pair cooldown to be stored")
	}
}

func TestEvaluate_LimitPerTxAndCachedReplay(t *testing.T) {
	repo := newRepo(t)
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)

	first, err := repo.Evaluate(EvaluateInput{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    2000001,
		IdempotencyKey: "k-limit",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("Evaluate() unexpected error: %v", err)
	}
	if first.RuleCode != RuleCodeLimitPerTx {
		t.Fatalf("expected LIMIT_PER_TX, got %+v", first)
	}

	second, err := repo.Evaluate(EvaluateInput{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    2000001,
		IdempotencyKey: "k-limit",
		Now:            now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("Evaluate() unexpected error on replay: %v", err)
	}
	if second != first {
		t.Fatalf("expected cached decision, got %+v vs %+v", second, first)
	}
}

func TestEvaluate_DailyLimit(t *testing.T) {
	repo := newRepo(t)
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)

	repo.dailyBySenderAndDate[buildDailyKey("user-001", now)] = 4999999

	decision, err := repo.Evaluate(EvaluateInput{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    2,
		IdempotencyKey: "k-daily",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("Evaluate() unexpected error: %v", err)
	}
	if decision.RuleCode != RuleCodeLimitDaily {
		t.Fatalf("expected LIMIT_DAILY, got %+v", decision)
	}
}

func TestEvaluate_VelocityLimit(t *testing.T) {
	repo := newRepo(t)
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	repo.velocityBySender["user-001"] = []time.Time{
		now.Add(-9 * time.Minute),
		now.Add(-8 * time.Minute),
		now.Add(-7 * time.Minute),
		now.Add(-6 * time.Minute),
		now.Add(-5 * time.Minute),
	}

	decision, err := repo.Evaluate(EvaluateInput{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    100,
		IdempotencyKey: "k-velocity",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("Evaluate() unexpected error: %v", err)
	}
	if decision.RuleCode != RuleCodeLimitVelocity {
		t.Fatalf("expected LIMIT_VELOCITY, got %+v", decision)
	}
}

func TestEvaluate_CooldownDirectional(t *testing.T) {
	repo := newRepo(t)
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	repo.cooldownByPair[buildPairKey("user-001", "user-002")] = now.Add(-10 * time.Second)

	blocked, err := repo.Evaluate(EvaluateInput{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    100,
		IdempotencyKey: "k-cooldown-a",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("Evaluate() unexpected error: %v", err)
	}
	if blocked.RuleCode != RuleCodeCooldownPair {
		t.Fatalf("expected COOLDOWN_PAIR, got %+v", blocked)
	}

	allowed, err := repo.Evaluate(EvaluateInput{
		SenderID:       "user-002",
		ReceiverID:     "user-001",
		AmountCents:    100,
		IdempotencyKey: "k-cooldown-b",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("Evaluate() unexpected error reverse direction: %v", err)
	}
	if !allowed.Allowed {
		t.Fatalf("expected reverse direction to be allowed, got %+v", allowed)
	}
}

func TestEvaluate_IdempotencyMismatch(t *testing.T) {
	repo := newRepo(t)
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)

	_, err := repo.Evaluate(EvaluateInput{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    100,
		IdempotencyKey: "k-idem",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("Evaluate() unexpected error: %v", err)
	}

	decision, err := repo.Evaluate(EvaluateInput{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    200,
		IdempotencyKey: "k-idem",
		Now:            now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("Evaluate() unexpected error: %v", err)
	}
	if decision.RuleCode != RuleCodeIdempotencyMismatch {
		t.Fatalf("expected IDEMPOTENCY_REUSED_MISMATCH, got %+v", decision)
	}
}

func TestCleanupLocked_RemovesExpiredState(t *testing.T) {
	repo := newRepo(t)
	now := time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)

	repo.idempotency["expired"] = DecisionCache{
		Decision:  Decision{Allowed: true, RuleCode: RuleCodeOK},
		ExpiresAt: now.Add(-time.Second),
	}
	repo.velocityBySender["user-001"] = []time.Time{now.Add(-11 * time.Minute)}
	repo.cooldownByPair[buildPairKey("user-001", "user-002")] = now.Add(-31 * time.Second)
	repo.dailyBySenderAndDate["user-001|2026-04-01"] = 100

	repo.cleanupLocked(now)

	if len(repo.idempotency) != 0 {
		t.Fatalf("expected expired idempotency cache to be removed")
	}
	if len(repo.velocityBySender) != 0 {
		t.Fatalf("expected stale velocity entries to be removed")
	}
	if len(repo.cooldownByPair) != 0 {
		t.Fatalf("expected stale cooldown entries to be removed")
	}
	if len(repo.dailyBySenderAndDate) != 0 {
		t.Fatalf("expected previous-day daily entries to be removed")
	}
}

func TestStartJanitor_CleansAndStops(t *testing.T) {
	cfg := testConfig(t)
	cfg.CleanupInterval = 10 * time.Millisecond
	repo, err := NewFraudRepository(cfg)
	if err != nil {
		t.Fatalf("NewFraudRepository() error: %v", err)
	}

	now := time.Now().UTC()
	repo.idempotency["expired"] = DecisionCache{
		Decision:  Decision{Allowed: true, RuleCode: RuleCodeOK},
		ExpiresAt: now.Add(-time.Second),
	}

	ctx, cancel := context.WithCancel(context.Background())
	repo.StartJanitor(ctx)
	time.Sleep(30 * time.Millisecond)
	cancel()
	time.Sleep(5 * time.Millisecond)

	repo.mu.RLock()
	defer repo.mu.RUnlock()
	if len(repo.idempotency) != 0 {
		t.Fatalf("expected janitor to remove expired idempotency cache")
	}
}

func TestHelpers(t *testing.T) {
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	if got := buildDailyKey("user-001", now); got != "user-001|2026-04-01" {
		t.Fatalf("unexpected daily key: %s", got)
	}
	if got := buildPairKey("a", "b"); got != "a->b" {
		t.Fatalf("unexpected pair key: %s", got)
	}

	recent := keepRecent([]time.Time{
		now.Add(-11 * time.Minute),
		now.Add(-9 * time.Minute),
		now.Add(-time.Minute),
	}, now.Add(-10*time.Minute))
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent entries, got %d", len(recent))
	}
}

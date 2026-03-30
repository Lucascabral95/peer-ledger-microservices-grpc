package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/peer-ledger/services/fraud-service/internal/config"
)

const (
	RuleCodeOK                     = "OK"
	RuleCodeLimitPerTx             = "LIMIT_PER_TX"
	RuleCodeLimitDaily             = "LIMIT_DAILY"
	RuleCodeLimitVelocity          = "LIMIT_VELOCITY"
	RuleCodeCooldownPair           = "COOLDOWN_PAIR"
	RuleCodeIdempotencyMismatch    = "IDEMPOTENCY_REUSED_MISMATCH"
	reasonOK                       = "transfer allowed"
	reasonLimitPerTx               = "transfer exceeds per-transaction limit"
	reasonLimitDaily               = "transfer exceeds daily limit"
	reasonLimitVelocity            = "transfer velocity limit exceeded"
	reasonCooldownPair             = "cooldown active for sender-receiver pair"
	reasonIdempotencyPayloadChange = "idempotency key reused with different payload"
)

type Decision struct {
	Allowed  bool
	Reason   string
	RuleCode string
}

type EvaluateInput struct {
	SenderID       string
	ReceiverID     string
	AmountCents    int64
	IdempotencyKey string
	Now            time.Time
}

type DecisionCache struct {
	Decision    Decision
	AmountCents int64
	SenderID    string
	ReceiverID  string
	ExpiresAt   time.Time
}

type Evaluator interface {
	Evaluate(input EvaluateInput) (Decision, error)
}

type FraudRepository struct {
	mu sync.RWMutex

	perTxLimitCents  int64
	dailyLimitCents  int64
	velocityMaxCount int
	velocityWindow   time.Duration
	pairCooldown     time.Duration
	idempotencyTTL   time.Duration
	cleanupInterval  time.Duration
	timezone         *time.Location

	dailyBySenderAndDate map[string]int64
	velocityBySender     map[string][]time.Time
	cooldownByPair       map[string]time.Time
	idempotency          map[string]DecisionCache
}

func NewFraudRepository(cfg *config.Config) (*FraudRepository, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &FraudRepository{
		perTxLimitCents:      cfg.PerTxLimitCents,
		dailyLimitCents:      cfg.DailyLimitCents,
		velocityMaxCount:     cfg.VelocityMaxCount,
		velocityWindow:       cfg.VelocityWindow,
		pairCooldown:         cfg.PairCooldown,
		idempotencyTTL:       cfg.IdempotencyTTL,
		cleanupInterval:      cfg.CleanupInterval,
		timezone:             cfg.Timezone,
		dailyBySenderAndDate: make(map[string]int64),
		velocityBySender:     make(map[string][]time.Time),
		cooldownByPair:       make(map[string]time.Time),
		idempotency:          make(map[string]DecisionCache),
	}, nil
}

func (r *FraudRepository) Evaluate(input EvaluateInput) (Decision, error) {
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}

	senderID := strings.TrimSpace(input.SenderID)
	receiverID := strings.TrimSpace(input.ReceiverID)
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.cleanupLocked(now)

	if cache, hit := r.idempotency[idempotencyKey]; hit {
		if now.After(cache.ExpiresAt) {
			delete(r.idempotency, idempotencyKey)
		} else {
			if cache.SenderID == senderID && cache.ReceiverID == receiverID && cache.AmountCents == input.AmountCents {
				return cache.Decision, nil
			}

			return Decision{
				Allowed:  false,
				Reason:   reasonIdempotencyPayloadChange,
				RuleCode: RuleCodeIdempotencyMismatch,
			}, nil
		}
	}

	if input.AmountCents > r.perTxLimitCents {
		decision := Decision{
			Allowed:  false,
			Reason:   reasonLimitPerTx,
			RuleCode: RuleCodeLimitPerTx,
		}
		r.storeIdempotencyLocked(idempotencyKey, senderID, receiverID, input.AmountCents, decision, now)
		return decision, nil
	}

	dailyKey := buildDailyKey(senderID, now.In(r.timezone))
	dailyUsed := r.dailyBySenderAndDate[dailyKey]
	if dailyUsed+input.AmountCents > r.dailyLimitCents {
		decision := Decision{
			Allowed:  false,
			Reason:   reasonLimitDaily,
			RuleCode: RuleCodeLimitDaily,
		}
		r.storeIdempotencyLocked(idempotencyKey, senderID, receiverID, input.AmountCents, decision, now)
		return decision, nil
	}

	windowStart := now.Add(-r.velocityWindow)
	recent := keepRecent(r.velocityBySender[senderID], windowStart)
	r.velocityBySender[senderID] = recent
	if len(recent) >= r.velocityMaxCount {
		decision := Decision{
			Allowed:  false,
			Reason:   reasonLimitVelocity,
			RuleCode: RuleCodeLimitVelocity,
		}
		r.storeIdempotencyLocked(idempotencyKey, senderID, receiverID, input.AmountCents, decision, now)
		return decision, nil
	}

	pairKey := buildPairKey(senderID, receiverID)
	if last, ok := r.cooldownByPair[pairKey]; ok && now.Sub(last) < r.pairCooldown {
		decision := Decision{
			Allowed:  false,
			Reason:   reasonCooldownPair,
			RuleCode: RuleCodeCooldownPair,
		}
		r.storeIdempotencyLocked(idempotencyKey, senderID, receiverID, input.AmountCents, decision, now)
		return decision, nil
	}

	approved := Decision{
		Allowed:  true,
		Reason:   reasonOK,
		RuleCode: RuleCodeOK,
	}

	r.dailyBySenderAndDate[dailyKey] = dailyUsed + input.AmountCents
	r.velocityBySender[senderID] = append(recent, now)
	r.cooldownByPair[pairKey] = now
	r.storeIdempotencyLocked(idempotencyKey, senderID, receiverID, input.AmountCents, approved, now)

	return approved, nil
}

func (r *FraudRepository) StartJanitor(ctx context.Context) {
	ticker := time.NewTicker(r.cleanupInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				r.mu.Lock()
				r.cleanupLocked(now)
				r.mu.Unlock()
			}
		}
	}()
}

func (r *FraudRepository) cleanupLocked(now time.Time) {
	for key, cache := range r.idempotency {
		if now.After(cache.ExpiresAt) {
			delete(r.idempotency, key)
		}
	}

	windowStart := now.Add(-r.velocityWindow)
	for senderID, entries := range r.velocityBySender {
		filtered := keepRecent(entries, windowStart)
		if len(filtered) == 0 {
			delete(r.velocityBySender, senderID)
			continue
		}
		r.velocityBySender[senderID] = filtered
	}

	for pair, last := range r.cooldownByPair {
		if now.Sub(last) >= r.pairCooldown {
			delete(r.cooldownByPair, pair)
		}
	}

	currentDate := now.In(r.timezone).Format(time.DateOnly)
	for key := range r.dailyBySenderAndDate {
		parts := strings.Split(key, "|")
		if len(parts) != 2 {
			delete(r.dailyBySenderAndDate, key)
			continue
		}
		if parts[1] != currentDate {
			delete(r.dailyBySenderAndDate, key)
		}
	}
}

func (r *FraudRepository) storeIdempotencyLocked(idempotencyKey, senderID, receiverID string, amountCents int64, decision Decision, now time.Time) {
	if idempotencyKey == "" {
		return
	}
	r.idempotency[idempotencyKey] = DecisionCache{
		Decision:    decision,
		AmountCents: amountCents,
		SenderID:    senderID,
		ReceiverID:  receiverID,
		ExpiresAt:   now.Add(r.idempotencyTTL),
	}
}

func keepRecent(entries []time.Time, windowStart time.Time) []time.Time {
	filtered := make([]time.Time, 0, len(entries))
	for _, entry := range entries {
		if !entry.Before(windowStart) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func buildDailyKey(senderID string, now time.Time) string {
	return senderID + "|" + now.Format(time.DateOnly)
}

func buildPairKey(senderID, receiverID string) string {
	return senderID + "->" + receiverID
}

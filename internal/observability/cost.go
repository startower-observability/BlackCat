package observability

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// pricePerMInputTokens maps model names to cost per 1M input tokens (USD).
var pricePerMInputTokens = map[string]float64{
	"gpt-4o":            2.50,
	"gpt-4o-mini":       0.15,
	"gpt-4":             30.0,
	"gpt-3.5-turbo":     0.50,
	"claude-3-5-sonnet": 3.00,
	"claude-3-opus":     15.0,
	"gemini-1.5-pro":    1.25,
	"gemini-1.5-flash":  0.075,
	"gpt-5-mini":    0.15,
	"gpt-4.1-mini":  0.15,
}

// pricePerMOutputTokens maps model names to cost per 1M output tokens (USD).
var pricePerMOutputTokens = map[string]float64{
	"gpt-4o":            10.0,
	"gpt-4o-mini":       0.60,
	"gpt-4":             60.0,
	"gpt-3.5-turbo":     1.50,
	"claude-3-5-sonnet": 15.0,
	"claude-3-opus":     75.0,
	"gemini-1.5-pro":    5.00,
	"gemini-1.5-flash":  0.30,
	"gpt-5-mini":    0.60,
	"gpt-4.1-mini":  0.60,
}

const (
	defaultInputPricePerM  = 2.00
	defaultOutputPricePerM = 8.00
)

// ModelUsage is per-model aggregate for one user.
type ModelUsage struct {
	Model             string
	Provider          string
	TotalInputTokens  int64
	TotalOutputTokens int64
	EstimatedCostUSD  float64
	CallCount         int64
}

// UserModelUsage adds UserID to ModelUsage.
type UserModelUsage struct {
	UserID string
	ModelUsage
}

// CostTracker records per-user token usage and estimated cost in SQLite.
// It is safe for concurrent use.
type CostTracker struct {
	db *sql.DB
	mu sync.Mutex
}

// NewCostTracker creates a CostTracker using the provided *sql.DB (shared connection).
// Auto-creates the schema on first call.
func NewCostTracker(db *sql.DB) (*CostTracker, error) {
	if err := createCostSchema(db); err != nil {
		return nil, fmt.Errorf("observability: create cost schema: %w", err)
	}
	return &CostTracker{db: db}, nil
}

func createCostSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS token_usage (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		session_id TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL,
		provider TEXT NOT NULL DEFAULT '',
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_usage_user ON token_usage(user_id);
	CREATE INDEX IF NOT EXISTS idx_usage_model ON token_usage(model);
	`
	_, err := db.Exec(schema)
	return err
}

// Record stores a usage event for a user/session/model.
func (t *CostTracker) Record(ctx context.Context, userID, sessionID, model, provider string, inputTokens, outputTokens int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	_, err := t.db.ExecContext(ctx,
		`INSERT INTO token_usage (user_id, session_id, model, provider, input_tokens, output_tokens)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		userID, sessionID, model, provider, inputTokens, outputTokens,
	)
	if err != nil {
		return fmt.Errorf("observability: record usage: %w", err)
	}
	return nil
}

// UserSummary returns aggregate usage for a user, grouped by model and provider.
func (t *CostTracker) UserSummary(ctx context.Context, userID string) ([]ModelUsage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	rows, err := t.db.QueryContext(ctx,
		`SELECT model, provider, SUM(input_tokens), SUM(output_tokens), COUNT(*)
		 FROM token_usage
		 WHERE user_id = ?
		 GROUP BY model, provider
		 ORDER BY model`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("observability: user summary: %w", err)
	}
	defer rows.Close()

	var results []ModelUsage
	for rows.Next() {
		var mu ModelUsage
		if err := rows.Scan(&mu.Model, &mu.Provider, &mu.TotalInputTokens, &mu.TotalOutputTokens, &mu.CallCount); err != nil {
			return nil, fmt.Errorf("observability: scan user summary: %w", err)
		}
		mu.EstimatedCostUSD = estimateCost(mu.Model, mu.TotalInputTokens, mu.TotalOutputTokens)
		results = append(results, mu)
	}
	return results, rows.Err()
}

// UserSummarySince returns aggregate usage for a user since a given time, grouped by model and provider.
func (t *CostTracker) UserSummarySince(ctx context.Context, userID string, since time.Time) ([]ModelUsage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	rows, err := t.db.QueryContext(ctx,
		`SELECT model, provider, SUM(input_tokens), SUM(output_tokens), COUNT(*)
		 FROM token_usage
		 WHERE user_id = ? AND recorded_at >= ?
		 GROUP BY model, provider
		 ORDER BY model`,
		userID, since,
	)
	if err != nil {
		return nil, fmt.Errorf("observability: user summary since: %w", err)
	}
	defer rows.Close()

	var results []ModelUsage
	for rows.Next() {
		var mu ModelUsage
		if err := rows.Scan(&mu.Model, &mu.Provider, &mu.TotalInputTokens, &mu.TotalOutputTokens, &mu.CallCount); err != nil {
			return nil, fmt.Errorf("observability: scan user summary since: %w", err)
		}
		mu.EstimatedCostUSD = estimateCost(mu.Model, mu.TotalInputTokens, mu.TotalOutputTokens)
		results = append(results, mu)
	}
	return results, rows.Err()
}

// AllSummary returns usage for all users, grouped by user, model, and provider.
func (t *CostTracker) AllSummary(ctx context.Context) ([]UserModelUsage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	rows, err := t.db.QueryContext(ctx,
		`SELECT user_id, model, provider, SUM(input_tokens), SUM(output_tokens), COUNT(*)
		 FROM token_usage
		 GROUP BY user_id, model, provider
		 ORDER BY user_id, model`,
	)
	if err != nil {
		return nil, fmt.Errorf("observability: all summary: %w", err)
	}
	defer rows.Close()

	var results []UserModelUsage
	for rows.Next() {
		var umu UserModelUsage
		if err := rows.Scan(&umu.UserID, &umu.Model, &umu.Provider, &umu.TotalInputTokens, &umu.TotalOutputTokens, &umu.CallCount); err != nil {
			return nil, fmt.Errorf("observability: scan all summary: %w", err)
		}
		umu.EstimatedCostUSD = estimateCost(umu.Model, umu.TotalInputTokens, umu.TotalOutputTokens)
		results = append(results, umu)
	}
	return results, rows.Err()
}

// AllSummarySince returns aggregate usage for all users since a given time.
func (t *CostTracker) AllSummarySince(ctx context.Context, since time.Time) ([]UserModelUsage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	rows, err := t.db.QueryContext(ctx,
		`SELECT user_id, model, provider, SUM(input_tokens), SUM(output_tokens), COUNT(*)
		 FROM token_usage
		 WHERE recorded_at >= ?
		 GROUP BY user_id, model, provider
		 ORDER BY user_id, model`,
		since,
	)
	if err != nil {
		return nil, fmt.Errorf("observability: all summary since: %w", err)
	}
	defer rows.Close()

	var results []UserModelUsage
	for rows.Next() {
		var umu UserModelUsage
		if err := rows.Scan(&umu.UserID, &umu.Model, &umu.Provider, &umu.TotalInputTokens, &umu.TotalOutputTokens, &umu.CallCount); err != nil {
			return nil, fmt.Errorf("observability: scan all summary since: %w", err)
		}
		umu.EstimatedCostUSD = estimateCost(umu.Model, umu.TotalInputTokens, umu.TotalOutputTokens)
		results = append(results, umu)
	}
	return results, rows.Err()
}

// estimateCost computes estimated cost in USD based on model pricing.
func estimateCost(model string, inputTokens, outputTokens int64) float64 {
	inPrice, ok := pricePerMInputTokens[model]
	if !ok {
		inPrice = defaultInputPricePerM
	}
	outPrice, ok := pricePerMOutputTokens[model]
	if !ok {
		outPrice = defaultOutputPricePerM
	}
	return (float64(inputTokens)/1e6)*inPrice + (float64(outputTokens)/1e6)*outPrice
}

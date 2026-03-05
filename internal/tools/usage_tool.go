package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/startower-observability/blackcat/internal/observability"
)

const (
	usageToolName        = "check_usage"
	usageToolDescription = "Check token usage and estimated cost for the current user"
)

var usageToolParameters = json.RawMessage(`{
	"type": "object",
	"properties": {
		"period": {
			"type": "string",
			"description": "Time period: 'today', '24h', '7d', '30d', or 'all' (default: '24h')"
		}
	}
}`)

// UsageTool queries token usage for all users from the cost tracker.
type UsageTool struct {
	costTracker *observability.CostTracker
}

// NewUsageTool creates a UsageTool.
func NewUsageTool(costTracker *observability.CostTracker) *UsageTool {
	return &UsageTool{costTracker: costTracker}
}

func (t *UsageTool) Name() string                { return usageToolName }
func (t *UsageTool) Description() string         { return usageToolDescription }
func (t *UsageTool) Parameters() json.RawMessage { return usageToolParameters }

func (t *UsageTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var args struct {
		Period string `json:"period"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &args)
	}
	if args.Period == "" {
		args.Period = "24h"
	}

	var usages []observability.UserModelUsage
	var err error
	var periodLabel string

	now := time.Now()
	switch args.Period {
	case "today":
		since := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		usages, err = t.costTracker.AllSummarySince(ctx, since)
		periodLabel = "today"
	case "7d":
		usages, err = t.costTracker.AllSummarySince(ctx, now.Add(-7*24*time.Hour))
		periodLabel = "last 7 days"
	case "30d":
		usages, err = t.costTracker.AllSummarySince(ctx, now.Add(-30*24*time.Hour))
		periodLabel = "last 30 days"
	case "all":
		usages, err = t.costTracker.AllSummary(ctx)
		periodLabel = "all time"
	default: // "24h"
		usages, err = t.costTracker.AllSummarySince(ctx, now.Add(-24*time.Hour))
		periodLabel = "last 24 hours"
	}
	if err != nil {
		return "", fmt.Errorf("check_usage: query failed: %w", err)
	}

	if len(usages) == 0 {
		return fmt.Sprintf("No token usage recorded for %s.", periodLabel), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Token usage (%s):\n\n", periodLabel))
	var totalCost float64
	for _, u := range usages {
		sb.WriteString(fmt.Sprintf("• %s (%s): %d input + %d output tokens — $%.4f\n",
			u.Model, u.Provider, u.TotalInputTokens, u.TotalOutputTokens, u.EstimatedCostUSD))
		totalCost += u.EstimatedCostUSD
	}
	sb.WriteString(fmt.Sprintf("\nTotal estimated cost: $%.4f", totalCost))
	return sb.String(), nil
}

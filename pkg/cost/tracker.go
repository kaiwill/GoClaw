package cost

import (
	"fmt"
	"sync"
	"time"
)

type CostEntry struct {
	Provider     string
	Model        string
	InputTokens  int64
	OutputTokens int64
	Cost         float64
	Timestamp    time.Time
	Metadata     map[string]interface{}
}

type CostTracker struct {
	mu            sync.RWMutex
	entries       []CostEntry
	providerRates map[string]ProviderRate
	dailyLimit    float64
	monthlyLimit  float64
	totalCost     float64
}

type ProviderRate struct {
	InputPricePer1M  float64
	OutputPricePer1M float64
}

func NewCostTracker() *CostTracker {
	return &CostTracker{
		entries:       make([]CostEntry, 0),
		providerRates: getDefaultRates(),
	}
}

func (t *CostTracker) RecordCost(provider, model string, inputTokens, outputTokens int64, metadata map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	rate, exists := t.providerRates[provider]
	if !exists {
		rate = ProviderRate{
			InputPricePer1M:  10.0,
			OutputPricePer1M: 30.0,
		}
	}

	inputCost := float64(inputTokens) / 1_000_000 * rate.InputPricePer1M
	outputCost := float64(outputTokens) / 1_000_000 * rate.OutputPricePer1M
	totalCost := inputCost + outputCost

	entry := CostEntry{
		Provider:     provider,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         totalCost,
		Timestamp:    time.Now(),
		Metadata:     metadata,
	}

	t.entries = append(t.entries, entry)
	t.totalCost += totalCost

	return nil
}

func (t *CostTracker) GetTotalCost() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.totalCost
}

func (t *CostTracker) GetCostByProvider(provider string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var total float64
	for _, entry := range t.entries {
		if entry.Provider == provider {
			total += entry.Cost
		}
	}

	return total
}

func (t *CostTracker) GetCostByModel(provider, model string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var total float64
	for _, entry := range t.entries {
		if entry.Provider == provider && entry.Model == model {
			total += entry.Cost
		}
	}

	return total
}

func (t *CostTracker) GetDailyCost(date time.Time) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	var total float64
	for _, entry := range t.entries {
		if entry.Timestamp.After(startOfDay) && entry.Timestamp.Before(endOfDay) {
			total += entry.Cost
		}
	}

	return total
}

func (t *CostTracker) GetMonthlyCost(year int, month time.Month) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	startOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	var total float64
	for _, entry := range t.entries {
		if entry.Timestamp.After(startOfMonth) && entry.Timestamp.Before(endOfMonth) {
			total += entry.Cost
		}
	}

	return total
}

func (t *CostTracker) SetRate(provider string, rate ProviderRate) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.providerRates[provider] = rate
}

func (t *CostTracker) SetLimits(daily, monthly float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.dailyLimit = daily
	t.monthlyLimit = monthly
}

func (t *CostTracker) CheckLimits() (bool, string) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	dailyCost := t.GetDailyCost(time.Now())
	if t.dailyLimit > 0 && dailyCost > t.dailyLimit {
		return false, fmt.Sprintf("daily cost limit exceeded: $%.2f > $%.2f", dailyCost, t.dailyLimit)
	}

	monthlyCost := t.GetMonthlyCost(time.Now().Year(), time.Now().Month())
	if t.monthlyLimit > 0 && monthlyCost > t.monthlyLimit {
		return false, fmt.Sprintf("monthly cost limit exceeded: $%.2f > $%.2f", monthlyCost, t.monthlyLimit)
	}

	return true, ""
}

func (t *CostTracker) GetEntries(limit int) []CostEntry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 || limit > len(t.entries) {
		limit = len(t.entries)
	}

	result := make([]CostEntry, limit)
	copy(result, t.entries[len(t.entries)-limit:])

	return result
}

func (t *CostTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = make([]CostEntry, 0)
	t.totalCost = 0
}

func getDefaultRates() map[string]ProviderRate {
	return map[string]ProviderRate{
		"openai": {
			InputPricePer1M:  15.0,
			OutputPricePer1M: 60.0,
		},
		"anthropic": {
			InputPricePer1M:  15.0,
			OutputPricePer1M: 75.0,
		},
		"gemini": {
			InputPricePer1M:  1.25,
			OutputPricePer1M: 5.0,
		},
		"ollama": {
			InputPricePer1M:  0.0,
			OutputPricePer1M: 0.0,
		},
	}
}

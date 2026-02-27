package restore

import (
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"

	"money-loves-me/internal/model"
)

// StrategyStore abstracts strategy persistence for restore.
type StrategyStore interface {
	GetAll() ([]model.Strategy, error)
	Create(strategy *model.Strategy) error
	Update(strategy *model.Strategy) error
}

// RiskStore abstracts risk config persistence for restore.
type RiskStore interface {
	Get() (*model.RiskConfig, error)
	Save(config *model.RiskConfig) error
}

// StrategyConfig represents a strategy's configuration for save/restore.
type StrategyConfig struct {
	Name   string                     `json:"name"`
	Type   string                     `json:"type"`
	Params map[string]decimal.Decimal `json:"params"`
	Active bool                       `json:"active"`
}

// RiskParams represents risk configuration for save/restore.
type RiskParams struct {
	MaxOrderAmount      decimal.Decimal            `json:"max_order_amount"`
	MaxDailyLoss        decimal.Decimal            `json:"max_daily_loss"`
	StopLossPercents    map[string]decimal.Decimal `json:"stop_loss_percents"`
	MaxPositionPercents map[string]decimal.Decimal `json:"max_position_percents"`
}

// ConfigRestorer handles saving and restoring strategy and risk configs from the database.
type ConfigRestorer struct {
	strategyStore StrategyStore
	riskStore     RiskStore
}

// NewConfigRestorer creates a new ConfigRestorer.
func NewConfigRestorer(ss StrategyStore, rs RiskStore) *ConfigRestorer {
	return &ConfigRestorer{strategyStore: ss, riskStore: rs}
}

// SaveStrategyConfigs persists strategy configurations to the database.
func (r *ConfigRestorer) SaveStrategyConfigs(configs []StrategyConfig) error {
	for _, cfg := range configs {
		paramsJSON, err := json.Marshal(cfg.Params)
		if err != nil {
			return fmt.Errorf("failed to marshal params for %s: %w", cfg.Name, err)
		}
		record := &model.Strategy{
			Name:   cfg.Name,
			Type:   cfg.Type,
			Params: paramsJSON,
			Active: cfg.Active,
		}
		if err := r.strategyStore.Create(record); err != nil {
			return fmt.Errorf("failed to save strategy %s: %w", cfg.Name, err)
		}
	}
	return nil
}

// RestoreStrategyConfigs loads strategy configurations from the database.
func (r *ConfigRestorer) RestoreStrategyConfigs() ([]StrategyConfig, error) {
	records, err := r.strategyStore.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to load strategies: %w", err)
	}

	configs := make([]StrategyConfig, 0, len(records))
	for _, rec := range records {
		var params map[string]decimal.Decimal
		if err := json.Unmarshal(rec.Params, &params); err != nil {
			return nil, fmt.Errorf("failed to unmarshal params for %s: %w", rec.Name, err)
		}
		configs = append(configs, StrategyConfig{
			Name:   rec.Name,
			Type:   rec.Type,
			Params: params,
			Active: rec.Active,
		})
	}
	return configs, nil
}

// SaveRiskConfig persists risk configuration to the database.
func (r *ConfigRestorer) SaveRiskConfig(params RiskParams) error {
	slJSON, err := json.Marshal(params.StopLossPercents)
	if err != nil {
		return fmt.Errorf("failed to marshal stop loss percents: %w", err)
	}
	mpJSON, err := json.Marshal(params.MaxPositionPercents)
	if err != nil {
		return fmt.Errorf("failed to marshal max position percents: %w", err)
	}
	record := &model.RiskConfig{
		MaxOrderAmount:      params.MaxOrderAmount,
		MaxDailyLoss:        params.MaxDailyLoss,
		StopLossPercents:    slJSON,
		MaxPositionPercents: mpJSON,
	}
	return r.riskStore.Save(record)
}

// RestoreRiskConfig loads risk configuration from the database.
func (r *ConfigRestorer) RestoreRiskConfig() (*RiskParams, error) {
	record, err := r.riskStore.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to load risk config: %w", err)
	}

	var slPercents map[string]decimal.Decimal
	if err := json.Unmarshal(record.StopLossPercents, &slPercents); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stop loss percents: %w", err)
	}
	var mpPercents map[string]decimal.Decimal
	if err := json.Unmarshal(record.MaxPositionPercents, &mpPercents); err != nil {
		return nil, fmt.Errorf("failed to unmarshal max position percents: %w", err)
	}

	return &RiskParams{
		MaxOrderAmount:      record.MaxOrderAmount,
		MaxDailyLoss:        record.MaxDailyLoss,
		StopLossPercents:    slPercents,
		MaxPositionPercents: mpPercents,
	}, nil
}

package restore

import (
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"

	"money-loves-me/internal/model"
)

// StrategyStore 为恢复功能抽象策略持久化存储。
type StrategyStore interface {
	GetAll() ([]model.Strategy, error)
	Create(strategy *model.Strategy) error
	Update(strategy *model.Strategy) error
}

// RiskStore 为恢复功能抽象风控配置持久化存储。
type RiskStore interface {
	Get() (*model.RiskConfig, error)
	Save(config *model.RiskConfig) error
}

// StrategyConfig 表示策略的保存/恢复配置。
type StrategyConfig struct {
	Name   string                     `json:"name"`
	Type   string                     `json:"type"`
	Params map[string]decimal.Decimal `json:"params"`
	Active bool                       `json:"active"`
}

// RiskParams 表示风控的保存/恢复配置。
type RiskParams struct {
	MaxOrderAmount      decimal.Decimal            `json:"max_order_amount"`
	MaxDailyLoss        decimal.Decimal            `json:"max_daily_loss"`
	StopLossPercents    map[string]decimal.Decimal `json:"stop_loss_percents"`
	MaxPositionPercents map[string]decimal.Decimal `json:"max_position_percents"`
}

// ConfigRestorer 处理从数据库保存和恢复策略及风控配置。
type ConfigRestorer struct {
	strategyStore StrategyStore
	riskStore     RiskStore
}

// NewConfigRestorer 创建新的 ConfigRestorer。
func NewConfigRestorer(ss StrategyStore, rs RiskStore) *ConfigRestorer {
	return &ConfigRestorer{strategyStore: ss, riskStore: rs}
}

// SaveStrategyConfigs 将策略配置持久化到数据库。
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

// RestoreStrategyConfigs 从数据库加载策略配置。
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

// SaveRiskConfig 将风控配置持久化到数据库。
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

// RestoreRiskConfig 从数据库加载风控配置。
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

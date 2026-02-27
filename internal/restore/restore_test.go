package restore

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"pgregory.net/rapid"

	"money-loves-me/internal/model"
	"money-loves-me/internal/store"
)

func setupTestDB(t testing.TB) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Strategy{}, &model.RiskConfig{}))
	return db
}

// Feature: binance-trading-system, Property 29: 策略配置启动恢复
// 对于任何已保存的策略配置和风控参数，恢复后的配置
// 应与保存时完全一致。
// **Validates: Requirements 9.4**
func TestProperty29_StrategyConfigStartupRestore(t *testing.T) {
	db := setupTestDB(t)
	rapid.Check(t, func(t *rapid.T) {
		// 在迭代之间清理表
		db.Exec("DELETE FROM strategies")
		db.Exec("DELETE FROM risk_configs")
		ss := store.NewStrategyStore(db)
		rs := store.NewRiskStore(db)
		restorer := NewConfigRestorer(ss, rs)

		// 生成随机策略配置
		numStrategies := rapid.IntRange(1, 5).Draw(t, "numStrategies")
		originalConfigs := make([]StrategyConfig, numStrategies)
		for i := 0; i < numStrategies; i++ {
			name := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "stratName")
			stype := rapid.SampledFrom([]string{"MA_CROSS", "RSI", "BOLLINGER"}).Draw(t, "stratType")
			numParams := rapid.IntRange(1, 4).Draw(t, "numParams")
			params := make(map[string]decimal.Decimal)
			for j := 0; j < numParams; j++ {
				pname := rapid.StringMatching(`[a-z]{2,6}`).Draw(t, "paramName")
				pval := decimal.NewFromFloat(rapid.Float64Range(0.1, 100).Draw(t, "paramVal"))
				params[pname] = pval
			}
			originalConfigs[i] = StrategyConfig{
				Name:   name,
				Type:   stype,
				Params: params,
				Active: rapid.Bool().Draw(t, "active"),
			}
		}

		// 保存策略配置
		err := restorer.SaveStrategyConfigs(originalConfigs)
		require.NoError(t, err)

		// 恢复策略配置
		restored, err := restorer.RestoreStrategyConfigs()
		require.NoError(t, err)
		require.Len(t, restored, len(originalConfigs))

		for i, orig := range originalConfigs {
			assert.Equal(t, orig.Name, restored[i].Name)
			assert.Equal(t, orig.Type, restored[i].Type)
			assert.Equal(t, orig.Active, restored[i].Active)
			for pname, pval := range orig.Params {
				restoredVal, ok := restored[i].Params[pname]
				assert.True(t, ok, "param %s should exist", pname)
				assert.True(t, pval.Equal(restoredVal),
					"param %s: original %s != restored %s", pname, pval.String(), restoredVal.String())
			}
		}

		// 生成并保存随机风控参数
		riskParams := RiskParams{
			MaxOrderAmount: decimal.NewFromFloat(rapid.Float64Range(100, 10000).Draw(t, "maxOrder")),
			MaxDailyLoss:   decimal.NewFromFloat(rapid.Float64Range(50, 5000).Draw(t, "maxLoss")),
			StopLossPercents: map[string]decimal.Decimal{
				"BTCUSDT": decimal.NewFromFloat(rapid.Float64Range(0.01, 0.1).Draw(t, "slBTC")),
			},
			MaxPositionPercents: map[string]decimal.Decimal{
				"BTCUSDT": decimal.NewFromFloat(rapid.Float64Range(0.1, 0.5).Draw(t, "mpBTC")),
			},
		}

		err = restorer.SaveRiskConfig(riskParams)
		require.NoError(t, err)

		restoredRisk, err := restorer.RestoreRiskConfig()
		require.NoError(t, err)
		require.NotNil(t, restoredRisk)

		assert.True(t, riskParams.MaxOrderAmount.Equal(restoredRisk.MaxOrderAmount))
		assert.True(t, riskParams.MaxDailyLoss.Equal(restoredRisk.MaxDailyLoss))
		for k, v := range riskParams.StopLossPercents {
			rv, ok := restoredRisk.StopLossPercents[k]
			assert.True(t, ok)
			assert.True(t, v.Equal(rv))
		}
		for k, v := range riskParams.MaxPositionPercents {
			rv, ok := restoredRisk.MaxPositionPercents[k]
			assert.True(t, ok)
			assert.True(t, v.Equal(rv))
		}
	})
}

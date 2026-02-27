package store

import (
	"encoding/json"
	"testing"
	"time"

	"money-loves-me/internal/model"

	"gorm.io/gorm"
	"pgregory.net/rapid"
)

// Feature: binance-trading-system, Property 12: 时间范围和条件过滤正确性
// Validates: Requirements 5.5, 7.5, 9.7, 10.7
//
// Property 12: 对于任何时间范围和过滤条件（symbol、strategy_name），
// 所有返回的记录必须具有在指定范围内的时间戳，
// 并且必须匹配指定的过滤条件。

func TestProperty12_TradeFilterTimeRangeAndConditions(t *testing.T) {
	db := setupTestDB(t)
	orderStore := NewOrderStore(db)
	tradeStore := NewTradeStore(db)

	rapid.Check(t, func(rt *rapid.T) {
		// 生成一个基准时间，并将交易分布在一个较宽的时间窗口内
		baseTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

		symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT"}
		strategies := []string{"ma_cross", "rsi", "bollinger"}

		// 为外键约束创建一个父订单
		order := &model.Order{
			Symbol:       "BTCUSDT",
			Side:         "BUY",
			Type:         "MARKET",
			Quantity:     genDecimal(rt, "order_qty"),
			Price:        genDecimal(rt, "order_price"),
			Status:       "FILLED",
			StrategyName: "ma_cross",
		}
		if err := orderStore.Create(order); err != nil {
			t.Fatalf("failed to create parent order: %v", err)
		}

		// 插入具有不同时间戳、交易对和策略名称的交易记录
		numTrades := rapid.IntRange(5, 20).Draw(rt, "num_trades")
		for i := 0; i < numTrades; i++ {
			offsetHours := rapid.IntRange(-72, 72).Draw(rt, "offset_hours")
			sym := rapid.SampledFrom(symbols).Draw(rt, "sym")
			strat := rapid.SampledFrom(strategies).Draw(rt, "strat")

			trade := &model.Trade{
				OrderID:        order.ID,
				Symbol:         sym,
				Side:           genSide(rt),
				Price:          genDecimal(rt, "price"),
				Quantity:       genDecimal(rt, "qty"),
				Amount:         genDecimal(rt, "amount"),
				Fee:            genDecimal(rt, "fee"),
				FeeAsset:       "USDT",
				StrategyName:   strat,
				DecisionReason: json.RawMessage(`{"indicators":{},"trigger_rule":"test","market_state":"test"}`),
				BalanceBefore:  genDecimal(rt, "bal_before"),
				BalanceAfter:   genDecimal(rt, "bal_after"),
				ExecutedAt:     baseTime.Add(time.Duration(offsetHours) * time.Hour),
			}
			if err := tradeStore.Create(trade); err != nil {
				t.Fatalf("failed to create trade: %v", err)
			}
		}

		// 生成随机查询参数
		startOffset := rapid.IntRange(-48, 24).Draw(rt, "start_offset")
		endOffset := rapid.IntRange(startOffset+1, 73).Draw(rt, "end_offset")
		queryStart := baseTime.Add(time.Duration(startOffset) * time.Hour)
		queryEnd := baseTime.Add(time.Duration(endOffset) * time.Hour)

		// 可选地按交易对和策略过滤
		filterSymbol := rapid.SampledFrom(append(symbols, "")).Draw(rt, "filter_symbol")
		filterStrategy := rapid.SampledFrom(append(strategies, "")).Draw(rt, "filter_strategy")

		filter := TradeFilter{
			Symbol:       filterSymbol,
			StrategyName: filterStrategy,
			Start:        queryStart,
			End:          queryEnd,
		}

		results, err := tradeStore.GetByFilter(filter)
		if err != nil {
			t.Fatalf("GetByFilter failed: %v", err)
		}

		// 断言：所有返回的记录的时间戳都在范围内
		for _, trade := range results {
			if trade.ExecutedAt.Before(queryStart) {
				t.Errorf("trade %d has ExecutedAt %v before query start %v", trade.ID, trade.ExecutedAt, queryStart)
			}
			if trade.ExecutedAt.After(queryEnd) {
				t.Errorf("trade %d has ExecutedAt %v after query end %v", trade.ID, trade.ExecutedAt, queryEnd)
			}

			// 断言：所有返回的记录都匹配过滤条件
			if filterSymbol != "" && trade.Symbol != filterSymbol {
				t.Errorf("trade %d has Symbol %s, expected %s", trade.ID, trade.Symbol, filterSymbol)
			}
			if filterStrategy != "" && trade.StrategyName != filterStrategy {
				t.Errorf("trade %d has StrategyName %s, expected %s", trade.ID, trade.StrategyName, filterStrategy)
			}
		}
	})
}

func TestProperty12_OrderFilterTimeRangeAndConditions(t *testing.T) {
	db := setupTestDB(t)
	orderStore := NewOrderStore(db)

	rapid.Check(t, func(rt *rapid.T) {
		baseTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

		symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT"}
		statuses := []string{"SUBMITTED", "PARTIAL", "FILLED", "CANCELLED"}

		// 插入具有不同时间戳、交易对和状态的订单
		numOrders := rapid.IntRange(5, 20).Draw(rt, "num_orders")
		for i := 0; i < numOrders; i++ {
			offsetHours := rapid.IntRange(-72, 72).Draw(rt, "offset_hours")
			sym := rapid.SampledFrom(symbols).Draw(rt, "sym")
			status := rapid.SampledFrom(statuses).Draw(rt, "status")

			// 我们需要手动设置 CreatedAt；GORM 的 autoCreateTime 不允许我们
			// 控制该值，所以我们使用 db.Session 来绕过它。
			order := &model.Order{
				Symbol:       sym,
				Side:         genSide(rt),
				Type:         genOrderType(rt),
				Quantity:     genDecimal(rt, "qty"),
				Price:        genDecimal(rt, "price"),
				Status:       status,
				Fee:          genDecimal(rt, "fee"),
				FeeAsset:     "USDT",
				StrategyName: genStrategyName(rt),
				CreatedAt:    baseTime.Add(time.Duration(offsetHours) * time.Hour),
			}
			if err := db.Session(&gorm.Session{SkipHooks: true}).Create(order).Error; err != nil {
				t.Fatalf("failed to create order: %v", err)
			}
		}

		// 生成随机查询参数
		startOffset := rapid.IntRange(-48, 24).Draw(rt, "start_offset")
		endOffset := rapid.IntRange(startOffset+1, 73).Draw(rt, "end_offset")
		queryStart := baseTime.Add(time.Duration(startOffset) * time.Hour)
		queryEnd := baseTime.Add(time.Duration(endOffset) * time.Hour)

		// 可选地按交易对和状态过滤
		filterSymbol := rapid.SampledFrom(append(symbols, "")).Draw(rt, "filter_symbol")
		filterStatus := rapid.SampledFrom(append(statuses, "")).Draw(rt, "filter_status")

		filter := OrderFilter{
			Symbol: filterSymbol,
			Status: filterStatus,
			Start:  queryStart,
			End:    queryEnd,
		}

		results, err := orderStore.GetByFilter(filter)
		if err != nil {
			t.Fatalf("GetByFilter failed: %v", err)
		}

		// 断言：所有返回的记录的时间戳都在范围内
		for _, order := range results {
			if order.CreatedAt.Before(queryStart) {
				t.Errorf("order %d has CreatedAt %v before query start %v", order.ID, order.CreatedAt, queryStart)
			}
			if order.CreatedAt.After(queryEnd) {
				t.Errorf("order %d has CreatedAt %v after query end %v", order.ID, order.CreatedAt, queryEnd)
			}

			// 断言：所有返回的记录都匹配过滤条件
			if filterSymbol != "" && order.Symbol != filterSymbol {
				t.Errorf("order %d has Symbol %s, expected %s", order.ID, order.Symbol, filterSymbol)
			}
			if filterStatus != "" && order.Status != filterStatus {
				t.Errorf("order %d has Status %s, expected %s", order.ID, order.Status, filterStatus)
			}
		}
	})
}

package order

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"money-loves-me/internal/model"
	"money-loves-me/internal/store"

	"github.com/shopspring/decimal"
)

// CSVHeaders 定义交易 CSV 导出的列标题。
var CSVHeaders = []string{
	"ID",
	"OrderID",
	"Symbol",
	"Side",
	"Price",
	"Quantity",
	"Amount",
	"Fee",
	"FeeAsset",
	"StrategyName",
	"DecisionReason",
	"BalanceBefore",
	"BalanceAfter",
	"ExecutedAt",
}

// ExportCSV 将匹配给定时间范围的交易记录以 CSV 格式写入 writer。
func ExportCSV(tradeStore TradeStoreInterface, start, end time.Time, writer io.Writer) error {
	trades, err := tradeStore.GetByFilter(store.TradeFilter{
		Start: start,
		End:   end,
	})
	if err != nil {
		return fmt.Errorf("failed to query trades for CSV export: %w", err)
	}

	return WriteTradesCSV(trades, writer)
}

// WriteTradesCSV 将交易切片以 CSV 格式写入 writer。
func WriteTradesCSV(trades []model.Trade, writer io.Writer) error {
	w := csv.NewWriter(writer)
	defer w.Flush()

	// 写入表头行。
	if err := w.Write(CSVHeaders); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// 将每条交易记录写为一行。
	for _, trade := range trades {
		record := []string{
			strconv.FormatInt(trade.ID, 10),
			strconv.FormatInt(trade.OrderID, 10),
			trade.Symbol,
			trade.Side,
			trade.Price.String(),
			trade.Quantity.String(),
			trade.Amount.String(),
			trade.Fee.String(),
			trade.FeeAsset,
			trade.StrategyName,
			string(trade.DecisionReason),
			trade.BalanceBefore.String(),
			trade.BalanceAfter.String(),
			trade.ExecutedAt.UTC().Format(time.RFC3339Nano),
		}
		if err := w.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// ParseTradesCSV 从 CSV reader 中读取交易记录。这是 WriteTradesCSV 的逆操作。
func ParseTradesCSV(reader io.Reader) ([]model.Trade, error) {
	r := csv.NewReader(reader)

	// 读取并验证表头。
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}
	if len(header) != len(CSVHeaders) {
		return nil, fmt.Errorf("unexpected CSV header length: got %d, want %d", len(header), len(CSVHeaders))
	}
	for i, h := range header {
		if h != CSVHeaders[i] {
			return nil, fmt.Errorf("unexpected CSV header at column %d: got %q, want %q", i, h, CSVHeaders[i])
		}
	}

	var trades []model.Trade
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV row: %w", err)
		}
		if len(record) != len(CSVHeaders) {
			return nil, fmt.Errorf("unexpected CSV row length: got %d, want %d", len(record), len(CSVHeaders))
		}

		trade, err := parseTradeRecord(record)
		if err != nil {
			return nil, err
		}
		trades = append(trades, *trade)
	}

	return trades, nil
}

func parseTradeRecord(record []string) (*model.Trade, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(record[0]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid ID %q: %w", record[0], err)
	}

	orderID, err := strconv.ParseInt(strings.TrimSpace(record[1]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid OrderID %q: %w", record[1], err)
	}

	price, err := decimalFromCSV(record[4], "Price")
	if err != nil {
		return nil, err
	}
	quantity, err := decimalFromCSV(record[5], "Quantity")
	if err != nil {
		return nil, err
	}
	amount, err := decimalFromCSV(record[6], "Amount")
	if err != nil {
		return nil, err
	}
	fee, err := decimalFromCSV(record[7], "Fee")
	if err != nil {
		return nil, err
	}
	balanceBefore, err := decimalFromCSV(record[11], "BalanceBefore")
	if err != nil {
		return nil, err
	}
	balanceAfter, err := decimalFromCSV(record[12], "BalanceAfter")
	if err != nil {
		return nil, err
	}

	executedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(record[13]))
	if err != nil {
		return nil, fmt.Errorf("invalid ExecutedAt %q: %w", record[13], err)
	}

	return &model.Trade{
		ID:             id,
		OrderID:        orderID,
		Symbol:         record[2],
		Side:           record[3],
		Price:          price,
		Quantity:       quantity,
		Amount:         amount,
		Fee:            fee,
		FeeAsset:       record[8],
		StrategyName:   record[9],
		DecisionReason: []byte(record[10]),
		BalanceBefore:  balanceBefore,
		BalanceAfter:   balanceAfter,
		ExecutedAt:     executedAt,
	}, nil
}

func decimalFromCSV(s, field string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid %s %q: %w", field, s, err)
	}
	return d, nil
}

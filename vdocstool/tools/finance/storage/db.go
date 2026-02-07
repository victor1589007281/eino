// Package storage 数据库存储层
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// DB 数据库连接管理
type DB struct {
	db     *sql.DB
	config *DBConfig
	mu     sync.RWMutex
}

// DBConfig 数据库配置
type DBConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultDBConfig 默认配置
func DefaultDBConfig() *DBConfig {
	return &DBConfig{
		Host:            "localhost",
		Port:            5432,
		User:            "postgres",
		Password:        "",
		Database:        "finance",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
}

// NewDB 创建数据库连接
func NewDB(config *DBConfig) (*DB, error) {
	if config == nil {
		config = DefaultDBConfig()
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.Database, config.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database failed: %w", err)
	}

	// 配置连接池
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database failed: %w", err)
	}

	return &DB{
		db:     db,
		config: config,
	}, nil
}

// Close 关闭连接
func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.Close()
}

// Ping 检查连接
func (d *DB) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// GetDB 获取原始数据库连接
func (d *DB) GetDB() *sql.DB {
	return d.db
}

// =====================================================
// K线数据操作
// =====================================================

// SaveKLines 保存K线数据
func (d *DB) SaveKLines(ctx context.Context, klines []*types.KLine) error {
	if len(klines) == 0 {
		return nil
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO kline (time, symbol, market, period, open, high, low, close, volume, amount, turnover_rate, adj_factor)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (time, symbol, market, period) DO UPDATE SET
			open = EXCLUDED.open,
			high = EXCLUDED.high,
			low = EXCLUDED.low,
			close = EXCLUDED.close,
			volume = EXCLUDED.volume,
			amount = EXCLUDED.amount,
			turnover_rate = EXCLUDED.turnover_rate,
			adj_factor = EXCLUDED.adj_factor
	`)
	if err != nil {
		return fmt.Errorf("prepare statement failed: %w", err)
	}
	defer stmt.Close()

	for _, kline := range klines {
		_, err := stmt.ExecContext(ctx,
			kline.Timestamp,
			kline.Symbol,
			string(kline.Market),
			kline.Period,
			kline.Open,
			kline.High,
			kline.Low,
			kline.Close,
			kline.Volume,
			kline.Amount,
			kline.TurnoverRate,
			kline.AdjFactor,
		)
		if err != nil {
			return fmt.Errorf("insert kline failed: %w", err)
		}
	}

	return tx.Commit()
}

// GetKLines 获取K线数据
func (d *DB) GetKLines(ctx context.Context, symbol string, market types.Market, period string, start, end time.Time, limit int) ([]*types.KLine, error) {
	query := `
		SELECT time, symbol, market, period, open, high, low, close, volume, amount, turnover_rate, adj_factor
		FROM kline
		WHERE symbol = $1 AND market = $2 AND period = $3 AND time >= $4 AND time <= $5
		ORDER BY time DESC
		LIMIT $6
	`

	rows, err := d.db.QueryContext(ctx, query, symbol, string(market), period, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("query klines failed: %w", err)
	}
	defer rows.Close()

	var klines []*types.KLine
	for rows.Next() {
		kline := &types.KLine{}
		var marketStr string
		err := rows.Scan(
			&kline.Timestamp,
			&kline.Symbol,
			&marketStr,
			&kline.Period,
			&kline.Open,
			&kline.High,
			&kline.Low,
			&kline.Close,
			&kline.Volume,
			&kline.Amount,
			&kline.TurnoverRate,
			&kline.AdjFactor,
		)
		if err != nil {
			return nil, fmt.Errorf("scan kline failed: %w", err)
		}
		kline.Market = types.Market(marketStr)
		klines = append(klines, kline)
	}

	return klines, nil
}

// =====================================================
// 行情快照操作
// =====================================================

// SaveQuoteSnapshot 保存行情快照
func (d *DB) SaveQuoteSnapshot(ctx context.Context, quote *types.Quote) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO quote_snapshot (time, symbol, market, name, price, open, high, low, pre_close, 
			change, change_pct, volume, amount, bid1, ask1, bid1_vol, ask1_vol, 
			turnover_rate, pe_ratio, pb_ratio, market_cap, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
		ON CONFLICT (time, symbol, market) DO UPDATE SET
			price = EXCLUDED.price,
			change = EXCLUDED.change,
			change_pct = EXCLUDED.change_pct,
			volume = EXCLUDED.volume,
			amount = EXCLUDED.amount
	`,
		quote.UpdateTime,
		quote.Symbol,
		string(quote.Market),
		quote.Name,
		quote.Price,
		quote.Open,
		quote.High,
		quote.Low,
		quote.PreClose,
		quote.Change,
		quote.ChangePct,
		quote.Volume,
		quote.Amount,
		quote.Bid1,
		quote.Ask1,
		quote.Bid1Vol,
		quote.Ask1Vol,
		quote.TurnoverRate,
		quote.PERatio,
		quote.PBRatio,
		quote.MarketCap,
		quote.Source,
	)
	return err
}

// =====================================================
// 新闻数据操作
// =====================================================

// SaveNews 保存新闻
func (d *DB) SaveNews(ctx context.Context, news *types.News) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO news (news_id, title, content, summary, source, category, importance, symbols, tags, url, publish_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (news_id) DO NOTHING
	`,
		news.ID,
		news.Title,
		news.Content,
		news.Summary,
		news.Source,
		news.Category,
		news.Importance,
		news.Symbols,
		news.Tags,
		news.URL,
		news.PublishTime,
	)
	return err
}

// GetNews 获取新闻列表
func (d *DB) GetNews(ctx context.Context, opts *NewsQueryOptions) ([]*types.News, error) {
	if opts == nil {
		opts = &NewsQueryOptions{Limit: 50}
	}

	query := `
		SELECT news_id, title, content, summary, source, category, importance, symbols, tags, url, publish_time, created_at
		FROM news
		WHERE 1=1
	`
	args := make([]interface{}, 0)
	argIdx := 1

	if opts.Source != "" {
		query += fmt.Sprintf(" AND source = $%d", argIdx)
		args = append(args, opts.Source)
		argIdx++
	}
	if opts.Category != "" {
		query += fmt.Sprintf(" AND category = $%d", argIdx)
		args = append(args, opts.Category)
		argIdx++
	}
	if opts.Symbol != "" {
		query += fmt.Sprintf(" AND $%d = ANY(symbols)", argIdx)
		args = append(args, opts.Symbol)
		argIdx++
	}
	if !opts.Since.IsZero() {
		query += fmt.Sprintf(" AND publish_time >= $%d", argIdx)
		args = append(args, opts.Since)
		argIdx++
	}

	query += " ORDER BY publish_time DESC"
	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, opts.Limit)

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query news failed: %w", err)
	}
	defer rows.Close()

	var newsList []*types.News
	for rows.Next() {
		n := &types.News{}
		err := rows.Scan(
			&n.ID,
			&n.Title,
			&n.Content,
			&n.Summary,
			&n.Source,
			&n.Category,
			&n.Importance,
			&n.Symbols,
			&n.Tags,
			&n.URL,
			&n.PublishTime,
			&n.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan news failed: %w", err)
		}
		newsList = append(newsList, n)
	}

	return newsList, nil
}

// NewsQueryOptions 新闻查询选项
type NewsQueryOptions struct {
	Source   string
	Category string
	Symbol   string
	Since    time.Time
	Limit    int
}

// =====================================================
// 投资组合操作
// =====================================================

// CreatePortfolio 创建组合
func (d *DB) CreatePortfolio(ctx context.Context, portfolio *types.Portfolio) error {
	err := d.db.QueryRowContext(ctx, `
		INSERT INTO portfolios (user_id, name, description, currency, initial_capital)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`,
		portfolio.UserID,
		portfolio.Name,
		portfolio.Description,
		portfolio.Currency,
		portfolio.InitialCapital,
	).Scan(&portfolio.ID, &portfolio.CreatedAt)
	return err
}

// GetPortfolio 获取组合
func (d *DB) GetPortfolio(ctx context.Context, userID string, name string) (*types.Portfolio, error) {
	p := &types.Portfolio{}
	err := d.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, description, currency, initial_capital, created_at, updated_at
		FROM portfolios
		WHERE user_id = $1 AND name = $2
	`, userID, name).Scan(
		&p.ID,
		&p.UserID,
		&p.Name,
		&p.Description,
		&p.Currency,
		&p.InitialCapital,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// GetPortfolios 获取用户所有组合
func (d *DB) GetPortfolios(ctx context.Context, userID string) ([]*types.Portfolio, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, user_id, name, description, currency, initial_capital, created_at, updated_at
		FROM portfolios
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var portfolios []*types.Portfolio
	for rows.Next() {
		p := &types.Portfolio{}
		err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.Name,
			&p.Description,
			&p.Currency,
			&p.InitialCapital,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		portfolios = append(portfolios, p)
	}
	return portfolios, nil
}

// =====================================================
// 持仓操作
// =====================================================

// SavePosition 保存持仓
func (d *DB) SavePosition(ctx context.Context, pos *types.Position) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO positions (portfolio_id, symbol, market, asset_type, quantity, avg_cost, current_price, market_value, profit_loss, profit_pct)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (portfolio_id, symbol, market) DO UPDATE SET
			quantity = EXCLUDED.quantity,
			avg_cost = EXCLUDED.avg_cost,
			current_price = EXCLUDED.current_price,
			market_value = EXCLUDED.market_value,
			profit_loss = EXCLUDED.profit_loss,
			profit_pct = EXCLUDED.profit_pct,
			updated_at = NOW()
	`,
		pos.PortfolioID,
		pos.Symbol,
		string(pos.Market),
		string(pos.AssetType),
		pos.Quantity,
		pos.AvgCost,
		pos.CurrentPrice,
		pos.MarketValue,
		pos.ProfitLoss,
		pos.ProfitPct,
	)
	return err
}

// GetPositions 获取持仓列表
func (d *DB) GetPositions(ctx context.Context, portfolioID int64) ([]*types.Position, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, portfolio_id, symbol, market, asset_type, quantity, avg_cost, current_price, market_value, profit_loss, profit_pct, updated_at
		FROM positions
		WHERE portfolio_id = $1
	`, portfolioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []*types.Position
	for rows.Next() {
		p := &types.Position{}
		var marketStr, assetTypeStr string
		err := rows.Scan(
			&p.ID,
			&p.PortfolioID,
			&p.Symbol,
			&marketStr,
			&assetTypeStr,
			&p.Quantity,
			&p.AvgCost,
			&p.CurrentPrice,
			&p.MarketValue,
			&p.ProfitLoss,
			&p.ProfitPct,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		p.Market = types.Market(marketStr)
		p.AssetType = types.AssetType(assetTypeStr)
		positions = append(positions, p)
	}
	return positions, nil
}

// DeletePosition 删除持仓
func (d *DB) DeletePosition(ctx context.Context, portfolioID int64, symbol string, market types.Market) error {
	_, err := d.db.ExecContext(ctx, `
		DELETE FROM positions
		WHERE portfolio_id = $1 AND symbol = $2 AND market = $3
	`, portfolioID, symbol, string(market))
	return err
}

// =====================================================
// 交易记录操作
// =====================================================

// SaveTransaction 保存交易记录
func (d *DB) SaveTransaction(ctx context.Context, tx *types.Transaction) error {
	return d.db.QueryRowContext(ctx, `
		INSERT INTO transactions (portfolio_id, symbol, market, trade_type, quantity, price, amount, fee, tax, note, trade_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`,
		tx.PortfolioID,
		tx.Symbol,
		string(tx.Market),
		tx.TradeType,
		tx.Quantity,
		tx.Price,
		tx.Amount,
		tx.Fee,
		tx.Tax,
		tx.Note,
		tx.TradeTime,
	).Scan(&tx.ID)
}

// GetTransactions 获取交易记录
func (d *DB) GetTransactions(ctx context.Context, portfolioID int64, limit int) ([]*types.Transaction, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, portfolio_id, symbol, market, trade_type, quantity, price, amount, fee, tax, note, trade_time, created_at
		FROM transactions
		WHERE portfolio_id = $1
		ORDER BY trade_time DESC
		LIMIT $2
	`, portfolioID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []*types.Transaction
	for rows.Next() {
		t := &types.Transaction{}
		var marketStr string
		err := rows.Scan(
			&t.ID,
			&t.PortfolioID,
			&t.Symbol,
			&marketStr,
			&t.TradeType,
			&t.Quantity,
			&t.Price,
			&t.Amount,
			&t.Fee,
			&t.Tax,
			&t.Note,
			&t.TradeTime,
			&t.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		t.Market = types.Market(marketStr)
		txs = append(txs, t)
	}
	return txs, nil
}

// =====================================================
// 提醒操作
// =====================================================

// CreateAlert 创建提醒
func (d *DB) CreateAlert(ctx context.Context, alert *types.Alert) error {
	return d.db.QueryRowContext(ctx, `
		INSERT INTO alerts (user_id, symbol, market, name, alert_type, threshold, status, notify_methods, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`,
		alert.UserID,
		alert.Symbol,
		string(alert.Market),
		alert.Name,
		alert.AlertType,
		alert.Threshold,
		alert.Status,
		alert.NotifyMethods,
		alert.Note,
	).Scan(&alert.ID, &alert.CreatedAt)
}

// GetActiveAlerts 获取活跃提醒
func (d *DB) GetActiveAlerts(ctx context.Context, userID string) ([]*types.Alert, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, user_id, symbol, market, name, alert_type, threshold, current_value, status, triggered_at, triggered_value, notify_methods, note, created_at, updated_at
		FROM alerts
		WHERE user_id = $1 AND status = 'active'
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []*types.Alert
	for rows.Next() {
		a := &types.Alert{}
		var marketStr string
		err := rows.Scan(
			&a.ID,
			&a.UserID,
			&a.Symbol,
			&marketStr,
			&a.Name,
			&a.AlertType,
			&a.Threshold,
			&a.CurrentValue,
			&a.Status,
			&a.TriggeredAt,
			&a.TriggeredValue,
			&a.NotifyMethods,
			&a.Note,
			&a.CreatedAt,
			&a.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		a.Market = types.Market(marketStr)
		alerts = append(alerts, a)
	}
	return alerts, nil
}

// UpdateAlertStatus 更新提醒状态
func (d *DB) UpdateAlertStatus(ctx context.Context, alertID int64, status string, triggeredValue float64) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE alerts SET
			status = $2,
			triggered_at = NOW(),
			triggered_value = $3,
			updated_at = NOW()
		WHERE id = $1
	`, alertID, status, triggeredValue)
	return err
}

// DeleteAlert 删除提醒
func (d *DB) DeleteAlert(ctx context.Context, alertID int64, userID string) error {
	_, err := d.db.ExecContext(ctx, `
		DELETE FROM alerts
		WHERE id = $1 AND user_id = $2
	`, alertID, userID)
	return err
}

// =====================================================
// 市场概览操作
// =====================================================

// SaveMarketOverview 保存市场概览
func (d *DB) SaveMarketOverview(ctx context.Context, overview *types.MarketOverview) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO market_overview (time, market, index_symbol, index_name, index_price, index_change, index_change_pct,
			advance_count, decline_count, unchanged_count, total_volume, total_amount,
			limit_up_count, limit_down_count, north_inflow, extra_data)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (time, market) DO UPDATE SET
			index_price = EXCLUDED.index_price,
			index_change = EXCLUDED.index_change,
			index_change_pct = EXCLUDED.index_change_pct,
			advance_count = EXCLUDED.advance_count,
			decline_count = EXCLUDED.decline_count,
			total_volume = EXCLUDED.total_volume,
			total_amount = EXCLUDED.total_amount
	`,
		overview.Time,
		string(overview.Market),
		overview.IndexSymbol,
		overview.IndexName,
		overview.IndexPrice,
		overview.IndexChange,
		overview.IndexChangePct,
		overview.AdvanceCount,
		overview.DeclineCount,
		overview.UnchangedCount,
		overview.TotalVolume,
		overview.TotalAmount,
		overview.LimitUpCount,
		overview.LimitDownCount,
		overview.NorthInflow,
		"{}",
	)
	return err
}

// GetLatestMarketOverview 获取最新市场概览
func (d *DB) GetLatestMarketOverview(ctx context.Context, market types.Market) (*types.MarketOverview, error) {
	o := &types.MarketOverview{}
	var marketStr string
	err := d.db.QueryRowContext(ctx, `
		SELECT time, market, index_symbol, index_name, index_price, index_change, index_change_pct,
			advance_count, decline_count, unchanged_count, total_volume, total_amount,
			limit_up_count, limit_down_count, north_inflow
		FROM market_overview
		WHERE market = $1
		ORDER BY time DESC
		LIMIT 1
	`, string(market)).Scan(
		&o.Time,
		&marketStr,
		&o.IndexSymbol,
		&o.IndexName,
		&o.IndexPrice,
		&o.IndexChange,
		&o.IndexChangePct,
		&o.AdvanceCount,
		&o.DeclineCount,
		&o.UnchangedCount,
		&o.TotalVolume,
		&o.TotalAmount,
		&o.LimitUpCount,
		&o.LimitDownCount,
		&o.NorthInflow,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	o.Market = types.Market(marketStr)
	return o, err
}

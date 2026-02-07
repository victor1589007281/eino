// Package portfolio 投资组合管理
package portfolio

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// Manager 投资组合管理器
type Manager struct {
	portfolios map[string]map[string]*types.Portfolio // userID -> name -> portfolio
	mu         sync.RWMutex
}

// NewManager 创建投资组合管理器
func NewManager() *Manager {
	return &Manager{
		portfolios: make(map[string]map[string]*types.Portfolio),
	}
}

// CreatePortfolio 创建投资组合
func (m *Manager) CreatePortfolio(ctx context.Context, userID, name, description, currency string, initialCapital float64) (*types.Portfolio, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.portfolios[userID]; !ok {
		m.portfolios[userID] = make(map[string]*types.Portfolio)
	}

	if _, ok := m.portfolios[userID][name]; ok {
		return nil, fmt.Errorf("portfolio %s already exists", name)
	}

	portfolio := &types.Portfolio{
		ID:             time.Now().UnixNano(),
		UserID:         userID,
		Name:           name,
		Description:    description,
		Currency:       currency,
		InitialCapital: initialCapital,
		Positions:      make([]*types.Position, 0),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	m.portfolios[userID][name] = portfolio
	return portfolio, nil
}

// GetPortfolio 获取投资组合
func (m *Manager) GetPortfolio(ctx context.Context, userID, name string) (*types.Portfolio, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if userPortfolios, ok := m.portfolios[userID]; ok {
		if portfolio, ok := userPortfolios[name]; ok {
			return portfolio, nil
		}
	}

	return nil, fmt.Errorf("portfolio %s not found", name)
}

// GetPortfolios 获取用户所有投资组合
func (m *Manager) GetPortfolios(ctx context.Context, userID string) ([]*types.Portfolio, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	portfolios := make([]*types.Portfolio, 0)
	if userPortfolios, ok := m.portfolios[userID]; ok {
		for _, p := range userPortfolios {
			portfolios = append(portfolios, p)
		}
	}

	return portfolios, nil
}

// DeletePortfolio 删除投资组合
func (m *Manager) DeletePortfolio(ctx context.Context, userID, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if userPortfolios, ok := m.portfolios[userID]; ok {
		if _, ok := userPortfolios[name]; ok {
			delete(userPortfolios, name)
			return nil
		}
	}

	return fmt.Errorf("portfolio %s not found", name)
}

// AddPosition 添加持仓
func (m *Manager) AddPosition(ctx context.Context, userID, portfolioName string, position *types.Position) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	portfolio, ok := m.getPortfolioUnsafe(userID, portfolioName)
	if !ok {
		return fmt.Errorf("portfolio %s not found", portfolioName)
	}

	// 检查是否已有该持仓
	for i, p := range portfolio.Positions {
		if p.Symbol == position.Symbol && p.Market == position.Market {
			// 合并持仓
			totalQty := p.Quantity + position.Quantity
			totalCost := p.Quantity*p.AvgCost + position.Quantity*position.AvgCost
			p.Quantity = totalQty
			p.AvgCost = totalCost / totalQty
			portfolio.Positions[i] = p
			portfolio.UpdatedAt = time.Now()
			return nil
		}
	}

	// 新增持仓
	position.ID = time.Now().UnixNano()
	position.PortfolioID = portfolio.ID
	portfolio.Positions = append(portfolio.Positions, position)
	portfolio.UpdatedAt = time.Now()

	return nil
}

// UpdatePosition 更新持仓
func (m *Manager) UpdatePosition(ctx context.Context, userID, portfolioName string, symbol string, market types.Market, quantity, avgCost float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	portfolio, ok := m.getPortfolioUnsafe(userID, portfolioName)
	if !ok {
		return fmt.Errorf("portfolio %s not found", portfolioName)
	}

	for i, p := range portfolio.Positions {
		if p.Symbol == symbol && p.Market == market {
			if quantity <= 0 {
				// 清仓
				portfolio.Positions = append(portfolio.Positions[:i], portfolio.Positions[i+1:]...)
			} else {
				p.Quantity = quantity
				p.AvgCost = avgCost
				p.UpdatedAt = time.Now()
				portfolio.Positions[i] = p
			}
			portfolio.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("position %s not found", symbol)
}

// RemovePosition 移除持仓
func (m *Manager) RemovePosition(ctx context.Context, userID, portfolioName string, symbol string, market types.Market) error {
	return m.UpdatePosition(ctx, userID, portfolioName, symbol, market, 0, 0)
}

// RecordTransaction 记录交易
func (m *Manager) RecordTransaction(ctx context.Context, userID, portfolioName string, tx *types.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	portfolio, ok := m.getPortfolioUnsafe(userID, portfolioName)
	if !ok {
		return fmt.Errorf("portfolio %s not found", portfolioName)
	}

	tx.ID = time.Now().UnixNano()
	tx.PortfolioID = portfolio.ID
	tx.CreatedAt = time.Now()

	// 更新持仓
	for i, p := range portfolio.Positions {
		if p.Symbol == tx.Symbol && p.Market == tx.Market {
			switch tx.TradeType {
			case "buy":
				totalQty := p.Quantity + tx.Quantity
				totalCost := p.Quantity*p.AvgCost + tx.Quantity*tx.Price + tx.Fee
				p.Quantity = totalQty
				p.AvgCost = totalCost / totalQty
			case "sell":
				p.Quantity -= tx.Quantity
				if p.Quantity <= 0 {
					portfolio.Positions = append(portfolio.Positions[:i], portfolio.Positions[i+1:]...)
					return nil
				}
			}
			p.UpdatedAt = time.Now()
			portfolio.Positions[i] = p
			portfolio.UpdatedAt = time.Now()
			return nil
		}
	}

	// 新建持仓(买入)
	if tx.TradeType == "buy" {
		position := &types.Position{
			ID:          time.Now().UnixNano(),
			PortfolioID: portfolio.ID,
			Symbol:      tx.Symbol,
			Market:      tx.Market,
			Quantity:    tx.Quantity,
			AvgCost:     (tx.Quantity*tx.Price + tx.Fee) / tx.Quantity,
			UpdatedAt:   time.Now(),
		}
		portfolio.Positions = append(portfolio.Positions, position)
	}

	portfolio.UpdatedAt = time.Now()
	return nil
}

// UpdatePrices 更新持仓价格
func (m *Manager) UpdatePrices(ctx context.Context, userID, portfolioName string, quotes map[string]*types.Quote) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	portfolio, ok := m.getPortfolioUnsafe(userID, portfolioName)
	if !ok {
		return fmt.Errorf("portfolio %s not found", portfolioName)
	}

	var totalValue, totalCost float64

	for i, p := range portfolio.Positions {
		key := fmt.Sprintf("%s_%s", p.Symbol, p.Market)
		if quote, ok := quotes[key]; ok {
			p.CurrentPrice = quote.Price
			p.MarketValue = p.Quantity * p.CurrentPrice
			p.ProfitLoss = p.MarketValue - p.Quantity*p.AvgCost
			if p.AvgCost > 0 {
				p.ProfitPct = (p.CurrentPrice - p.AvgCost) / p.AvgCost * 100
			}
			p.UpdatedAt = time.Now()
			portfolio.Positions[i] = p
		}
		totalValue += p.MarketValue
		totalCost += p.Quantity * p.AvgCost
	}

	portfolio.TotalValue = totalValue
	portfolio.TotalProfit = totalValue - totalCost
	if totalCost > 0 {
		portfolio.TotalProfitPct = portfolio.TotalProfit / totalCost * 100
	}
	portfolio.UpdatedAt = time.Now()

	return nil
}

// GetPortfolioSummary 获取组合概要
func (m *Manager) GetPortfolioSummary(ctx context.Context, userID, portfolioName string) (*PortfolioSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	portfolio, ok := m.getPortfolioUnsafe(userID, portfolioName)
	if !ok {
		return nil, fmt.Errorf("portfolio %s not found", portfolioName)
	}

	summary := &PortfolioSummary{
		Portfolio:      portfolio,
		PositionCount:  len(portfolio.Positions),
		TotalCost:      0,
		TotalValue:     portfolio.TotalValue,
		TotalProfit:    portfolio.TotalProfit,
		TotalProfitPct: portfolio.TotalProfitPct,
		AssetAllocation: make(map[string]float64),
	}

	for _, p := range portfolio.Positions {
		summary.TotalCost += p.Quantity * p.AvgCost
		
		// 资产配置统计
		assetType := string(p.AssetType)
		if assetType == "" {
			assetType = "stock"
		}
		summary.AssetAllocation[assetType] += p.MarketValue
	}

	// 转换为百分比
	if summary.TotalValue > 0 {
		for k, v := range summary.AssetAllocation {
			summary.AssetAllocation[k] = v / summary.TotalValue * 100
		}
	}

	return summary, nil
}

// PortfolioSummary 组合概要
type PortfolioSummary struct {
	Portfolio       *types.Portfolio   `json:"portfolio"`
	PositionCount   int                `json:"position_count"`
	TotalCost       float64            `json:"total_cost"`
	TotalValue      float64            `json:"total_value"`
	TotalProfit     float64            `json:"total_profit"`
	TotalProfitPct  float64            `json:"total_profit_pct"`
	AssetAllocation map[string]float64 `json:"asset_allocation"` // 资产类型 -> 占比
}

// getPortfolioUnsafe 获取组合(不加锁)
func (m *Manager) getPortfolioUnsafe(userID, portfolioName string) (*types.Portfolio, bool) {
	if userPortfolios, ok := m.portfolios[userID]; ok {
		if portfolio, ok := userPortfolios[portfolioName]; ok {
			return portfolio, true
		}
	}
	return nil, false
}

// Package sources 频率限制
package sources

import (
	"sync"
	"time"
)

// RateLimiter 频率限制器
type RateLimiter struct {
	limiters map[string]*TokenBucket
	mu       sync.RWMutex
}

// TokenBucket 令牌桶
type TokenBucket struct {
	rate       float64   // 每秒生成令牌数
	capacity   float64   // 桶容量
	tokens     float64   // 当前令牌数
	lastUpdate time.Time // 最后更新时间
	mu         sync.Mutex
}

// NewRateLimiter 创建频率限制器
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*TokenBucket),
	}
}

// NewTokenBucket 创建令牌桶
func NewTokenBucket(rps int) *TokenBucket {
	rate := float64(rps)
	return &TokenBucket{
		rate:       rate,
		capacity:   rate * 2, // 容量为速率的2倍,允许短时突发
		tokens:     rate,     // 初始满桶
		lastUpdate: time.Now(),
	}
}

// Allow 检查是否允许请求
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastUpdate).Seconds()
	tb.lastUpdate = now

	// 补充令牌
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}

	// 检查是否有足够的令牌
	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}

	return false
}

// AllowN 检查是否允许N个请求
func (tb *TokenBucket) AllowN(n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastUpdate).Seconds()
	tb.lastUpdate = now

	// 补充令牌
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}

	// 检查是否有足够的令牌
	need := float64(n)
	if tb.tokens >= need {
		tb.tokens -= need
		return true
	}

	return false
}

// GetTokens 获取当前令牌数
func (tb *TokenBucket) GetTokens() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastUpdate).Seconds()
	
	tokens := tb.tokens + elapsed*tb.rate
	if tokens > tb.capacity {
		tokens = tb.capacity
	}

	return tokens
}

// SetRate 设置速率
func (tb *TokenBucket) SetRate(rps int) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.rate = float64(rps)
	tb.capacity = float64(rps) * 2
}

// SetLimit 设置数据源限速
func (rl *RateLimiter) SetLimit(name string, rps int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if limiter, ok := rl.limiters[name]; ok {
		limiter.SetRate(rps)
	} else {
		rl.limiters[name] = NewTokenBucket(rps)
	}
}

// Allow 检查数据源是否允许请求
func (rl *RateLimiter) Allow(name string) bool {
	rl.mu.RLock()
	limiter, ok := rl.limiters[name]
	rl.mu.RUnlock()

	if !ok {
		return true // 未设置限速,默认允许
	}

	return limiter.Allow()
}

// AllowN 检查数据源是否允许N个请求
func (rl *RateLimiter) AllowN(name string, n int) bool {
	rl.mu.RLock()
	limiter, ok := rl.limiters[name]
	rl.mu.RUnlock()

	if !ok {
		return true
	}

	return limiter.AllowN(n)
}

// GetAvailableTokens 获取可用令牌数
func (rl *RateLimiter) GetAvailableTokens(name string) float64 {
	rl.mu.RLock()
	limiter, ok := rl.limiters[name]
	rl.mu.RUnlock()

	if !ok {
		return -1 // 未设置限速
	}

	return limiter.GetTokens()
}

// RemoveLimit 移除限速
func (rl *RateLimiter) RemoveLimit(name string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.limiters, name)
}

// GetLimits 获取所有限速配置
func (rl *RateLimiter) GetLimits() map[string]float64 {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	result := make(map[string]float64, len(rl.limiters))
	for name, limiter := range rl.limiters {
		result[name] = limiter.rate
	}
	return result
}

// DefaultSourceLimits 默认数据源限速配置
var DefaultSourceLimits = map[string]int{
	"eastmoney": 10, // 东方财富: 10 QPS
	"sina":      10, // 新浪: 10 QPS
	"tencent":   10, // 腾讯: 10 QPS
	"tiantian":  5,  // 天天基金: 5 QPS
	"cls":       5,  // 财联社: 5 QPS
	"jin10":     3,  // 金十: 3 QPS
	"xueqiu":    3,  // 雪球: 3 QPS
	"coingecko": 2,  // CoinGecko: 2 QPS (免费版限制)
}

// ApplyDefaultLimits 应用默认限速
func (rl *RateLimiter) ApplyDefaultLimits() {
	for name, rps := range DefaultSourceLimits {
		rl.SetLimit(name, rps)
	}
}

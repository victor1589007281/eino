package sources

import (
	"testing"
	"time"
)

func TestTokenBucket_Allow(t *testing.T) {
	tb := NewTokenBucket(10) // 10 QPS

	// 初始应该有足够的令牌
	for i := 0; i < 10; i++ {
		if !tb.Allow() {
			t.Errorf("Expected Allow() to return true for request %d", i)
		}
	}

	// 令牌用完后应该返回 false
	// 由于突发容量是2倍，可能还能通过几次
	denied := 0
	for i := 0; i < 20; i++ {
		if !tb.Allow() {
			denied++
		}
	}

	if denied == 0 {
		t.Error("Expected some requests to be denied after exhausting tokens")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	tb := NewTokenBucket(10) // 10 QPS

	// 用完所有令牌
	for i := 0; i < 30; i++ {
		tb.Allow()
	}

	// 等待令牌补充
	time.Sleep(200 * time.Millisecond) // 应该补充约2个令牌

	// 现在应该能通过
	if !tb.Allow() {
		t.Error("Expected Allow() to return true after token refill")
	}
}

func TestTokenBucket_AllowN(t *testing.T) {
	tb := NewTokenBucket(10) // 10 QPS, capacity 20, initial tokens 10

	// 请求10个令牌 (初始有10个)
	if !tb.AllowN(10) {
		t.Error("Expected AllowN(10) to return true")
	}

	// 立即再请求应该失败 (令牌已用完)
	if tb.AllowN(5) {
		t.Error("Expected AllowN(5) to return false after exhausting initial tokens")
	}

	// 等待一段时间让令牌补充
	time.Sleep(300 * time.Millisecond) // 应该补充约3个令牌
	
	if !tb.AllowN(2) {
		t.Error("Expected AllowN(2) to return true after token refill")
	}
}

func TestTokenBucket_GetTokens(t *testing.T) {
	tb := NewTokenBucket(10) // 10 QPS

	// 初始令牌数应该等于速率
	tokens := tb.GetTokens()
	if tokens < 9 || tokens > 11 {
		t.Errorf("Expected initial tokens around 10, got %f", tokens)
	}

	// 消耗一些令牌
	for i := 0; i < 5; i++ {
		tb.Allow()
	}

	tokens = tb.GetTokens()
	if tokens < 4 || tokens > 6 {
		t.Errorf("Expected tokens around 5 after consuming 5, got %f", tokens)
	}
}

func TestRateLimiter_SetLimit(t *testing.T) {
	rl := NewRateLimiter()

	// 设置限速
	rl.SetLimit("eastmoney", 10)
	rl.SetLimit("sina", 5)

	limits := rl.GetLimits()
	if limits["eastmoney"] != 10 {
		t.Errorf("Expected eastmoney limit to be 10, got %f", limits["eastmoney"])
	}
	if limits["sina"] != 5 {
		t.Errorf("Expected sina limit to be 5, got %f", limits["sina"])
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter()
	rl.SetLimit("eastmoney", 5)

	// 未设置限速的源应该允许
	if !rl.Allow("unknown") {
		t.Error("Expected Allow() to return true for unregistered source")
	}

	// 已设置限速的源
	allowed := 0
	for i := 0; i < 20; i++ {
		if rl.Allow("eastmoney") {
			allowed++
		}
	}

	// 由于令牌桶容量是2倍，允许的请求数应该在5-15之间
	if allowed < 5 || allowed > 15 {
		t.Errorf("Expected allowed requests between 5-15, got %d", allowed)
	}
}

func TestRateLimiter_RemoveLimit(t *testing.T) {
	rl := NewRateLimiter()
	rl.SetLimit("eastmoney", 5)

	// 移除限速
	rl.RemoveLimit("eastmoney")

	// 移除后应该允许所有请求
	for i := 0; i < 100; i++ {
		if !rl.Allow("eastmoney") {
			t.Error("Expected Allow() to return true after removing limit")
		}
	}
}

func TestRateLimiter_ApplyDefaultLimits(t *testing.T) {
	rl := NewRateLimiter()
	rl.ApplyDefaultLimits()

	limits := rl.GetLimits()

	// 验证默认限速
	if limits["eastmoney"] != 10 {
		t.Errorf("Expected eastmoney default limit 10, got %f", limits["eastmoney"])
	}
	if limits["sina"] != 10 {
		t.Errorf("Expected sina default limit 10, got %f", limits["sina"])
	}
	if limits["cls"] != 5 {
		t.Errorf("Expected cls default limit 5, got %f", limits["cls"])
	}
}

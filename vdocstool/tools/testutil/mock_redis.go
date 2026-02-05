// Package testutil Redis Mock 客户端
package testutil

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// MockRedis 模拟 Redis 客户端
type MockRedis struct {
	data   map[string]interface{}
	expiry map[string]time.Time
	mu     sync.RWMutex
}

// NewMockRedis 创建 Mock Redis
func NewMockRedis() *MockRedis {
	return &MockRedis{
		data:   make(map[string]interface{}),
		expiry: make(map[string]time.Time),
	}
}

// Set 设置键值
func (r *MockRedis) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data[key] = value
	if expiration > 0 {
		r.expiry[key] = time.Now().Add(expiration)
	}
	return nil
}

// Get 获取值
func (r *MockRedis) Get(ctx context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 检查过期
	if exp, ok := r.expiry[key]; ok && time.Now().After(exp) {
		return "", nil
	}

	value, ok := r.data[key]
	if !ok {
		return "", nil
	}

	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		b, _ := json.Marshal(v)
		return string(b), nil
	}
}

// Del 删除键
func (r *MockRedis) Del(ctx context.Context, keys ...string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var count int64
	for _, key := range keys {
		if _, ok := r.data[key]; ok {
			delete(r.data, key)
			delete(r.expiry, key)
			count++
		}
	}
	return count, nil
}

// Exists 检查键是否存在
func (r *MockRedis) Exists(ctx context.Context, keys ...string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int64
	for _, key := range keys {
		if exp, ok := r.expiry[key]; ok && time.Now().After(exp) {
			continue
		}
		if _, ok := r.data[key]; ok {
			count++
		}
	}
	return count, nil
}

// LPush 左侧推入列表
func (r *MockRedis) LPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	list, ok := r.data[key].([]interface{})
	if !ok {
		list = []interface{}{}
	}

	// 在前面插入
	newList := make([]interface{}, len(values)+len(list))
	copy(newList, values)
	copy(newList[len(values):], list)
	r.data[key] = newList

	return int64(len(newList)), nil
}

// RPush 右侧推入列表
func (r *MockRedis) RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	list, ok := r.data[key].([]interface{})
	if !ok {
		list = []interface{}{}
	}

	list = append(list, values...)
	r.data[key] = list

	return int64(len(list)), nil
}

// LRange 获取列表范围
func (r *MockRedis) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list, ok := r.data[key].([]interface{})
	if !ok {
		return []string{}, nil
	}

	length := int64(len(list))
	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop {
		return []string{}, nil
	}

	result := make([]string, 0, stop-start+1)
	for i := start; i <= stop; i++ {
		switch v := list[i].(type) {
		case string:
			result = append(result, v)
		case []byte:
			result = append(result, string(v))
		default:
			b, _ := json.Marshal(v)
			result = append(result, string(b))
		}
	}
	return result, nil
}

// LLen 获取列表长度
func (r *MockRedis) LLen(ctx context.Context, key string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list, ok := r.data[key].([]interface{})
	if !ok {
		return 0, nil
	}
	return int64(len(list)), nil
}

// LTrim 裁剪列表
func (r *MockRedis) LTrim(ctx context.Context, key string, start, stop int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	list, ok := r.data[key].([]interface{})
	if !ok {
		return nil
	}

	length := int64(len(list))
	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop {
		r.data[key] = []interface{}{}
		return nil
	}

	r.data[key] = list[start : stop+1]
	return nil
}

// HSet 设置哈希字段
func (r *MockRedis) HSet(ctx context.Context, key string, values ...interface{}) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	hash, ok := r.data[key].(map[string]interface{})
	if !ok {
		hash = make(map[string]interface{})
	}

	var count int64
	for i := 0; i < len(values)-1; i += 2 {
		field := values[i].(string)
		value := values[i+1]
		if _, exists := hash[field]; !exists {
			count++
		}
		hash[field] = value
	}

	r.data[key] = hash
	return count, nil
}

// HGet 获取哈希字段
func (r *MockRedis) HGet(ctx context.Context, key, field string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hash, ok := r.data[key].(map[string]interface{})
	if !ok {
		return "", nil
	}

	value, ok := hash[field]
	if !ok {
		return "", nil
	}

	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		b, _ := json.Marshal(v)
		return string(b), nil
	}
}

// HGetAll 获取所有哈希字段
func (r *MockRedis) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hash, ok := r.data[key].(map[string]interface{})
	if !ok {
		return map[string]string{}, nil
	}

	result := make(map[string]string)
	for k, v := range hash {
		switch val := v.(type) {
		case string:
			result[k] = val
		case []byte:
			result[k] = string(val)
		default:
			b, _ := json.Marshal(val)
			result[k] = string(b)
		}
	}
	return result, nil
}

// Keys 获取匹配的键
func (r *MockRedis) Keys(ctx context.Context, pattern string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var keys []string
	for key := range r.data {
		// 简化匹配：只支持 * 通配符
		if matchPattern(pattern, key) {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// FlushAll 清空所有数据
func (r *MockRedis) FlushAll(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data = make(map[string]interface{})
	r.expiry = make(map[string]time.Time)
	return nil
}

// Close 关闭连接
func (r *MockRedis) Close() error {
	return nil
}

// matchPattern 简单的模式匹配
func matchPattern(pattern, key string) bool {
	if pattern == "*" {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix
	}
	if len(pattern) > 0 && pattern[0] == '*' {
		suffix := pattern[1:]
		return len(key) >= len(suffix) && key[len(key)-len(suffix):] == suffix
	}
	return pattern == key
}

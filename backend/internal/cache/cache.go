package cache

import (
	"context"
	"errors"
	"time"
)

// ErrCacheMiss 缓存未命中
var ErrCacheMiss = errors.New("cache miss")

// ZMember 有序集合成员
type ZMember struct {
	Member string
	Score  float64
}

// Cache 缓存接口
//
// 桌面端默认使用进程内缓存（MemoryCache），Redis 作为可选实现，
// 通过配置 CACHE_DRIVER=memory|redis 切换。
type Cache interface {
	// Get 读取字符串值，未命中返回 ErrCacheMiss
	Get(ctx context.Context, key string) (string, error)
	// Set 写入字符串值，ttl 为 0 表示不过期
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	// Delete 删除键
	Delete(ctx context.Context, key string) error
	// SAdd 向集合添加成员
	SAdd(ctx context.Context, key string, members ...string) error
	// SMembers 读取集合全部成员
	SMembers(ctx context.Context, key string) ([]string, error)
	// SetMembers 整体覆盖集合内容（用于会话记忆列表等有界集合）
	SetMembers(ctx context.Context, key string, members []string, ttl time.Duration) error
	// ZAdd 向有序集合添加成员
	ZAdd(ctx context.Context, key string, members ...ZMember) error
	// SortedRevRange 按分数倒序读取有序集合成员，count 为最大返回数量
	SortedRevRange(ctx context.Context, key string, count int) ([]string, error)
	// Ping 健康检查
	Ping(ctx context.Context) error
	// Close 释放资源
	Close() error
}

// New 根据驱动名创建缓存实例
func New(driver, redisURL string, maxEntries int) (Cache, error) {
	switch driver {
	case "redis":
		return NewRedisCache(redisURL)
	default:
		return NewMemoryCache(maxEntries), nil
	}
}

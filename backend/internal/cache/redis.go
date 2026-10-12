package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache 基于 Redis 的缓存实现（可选）
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache 创建 Redis 缓存
func NewRedisCache(addr string) (*RedisCache, error) {
	if addr == "" {
		return nil, errors.New("redis address is empty")
	}

	client := redis.NewClient(&redis.Options{
		Addr:         normalizeAddr(addr),
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisCache{client: client}, nil
}

// normalizeAddr 兼容 redis:// 前缀的地址写法
func normalizeAddr(addr string) string {
	addr = strings.TrimPrefix(addr, "redis://")
	return strings.TrimSuffix(addr, "/")
}

// Get 读取字符串值
func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	value, err := c.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrCacheMiss
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// Set 写入字符串值
func (c *RedisCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// Delete 删除键
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// SAdd 向集合添加成员
func (c *RedisCache) SAdd(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	values := make([]interface{}, len(members))
	for i, m := range members {
		values[i] = m
	}
	return c.client.SAdd(ctx, key, values...).Err()
}

// SMembers 读取集合全部成员
func (c *RedisCache) SMembers(ctx context.Context, key string) ([]string, error) {
	members, err := c.client.SMembers(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, ErrCacheMiss
	}
	return members, nil
}

// SetMembers 整体覆盖集合内容
func (c *RedisCache) SetMembers(ctx context.Context, key string, members []string, ttl time.Duration) error {
	pipe := c.client.TxPipeline()
	pipe.Del(ctx, key)
	if len(members) > 0 {
		values := make([]interface{}, len(members))
		for i, m := range members {
			values[i] = m
		}
		pipe.SAdd(ctx, key, values...)
	}
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// ZAdd 向有序集合添加成员
func (c *RedisCache) ZAdd(ctx context.Context, key string, members ...ZMember) error {
	if len(members) == 0 {
		return nil
	}
	values := make([]redis.Z, len(members))
	for i, m := range members {
		values[i] = redis.Z{Score: m.Score, Member: m.Member}
	}
	return c.client.ZAdd(ctx, key, values...).Err()
}

// SortedRevRange 按分数倒序读取有序集合成员
func (c *RedisCache) SortedRevRange(ctx context.Context, key string, count int) ([]string, error) {
	if count <= 0 {
		count = -1
	}
	members, err := c.client.ZRevRange(ctx, key, 0, int64(count-1)).Result()
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, ErrCacheMiss
	}
	return members, nil
}

// Ping 健康检查
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close 释放资源
func (c *RedisCache) Close() error {
	return c.client.Close()
}

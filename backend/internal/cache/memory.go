package cache

import (
	"container/list"
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// 默认缓存容量
const defaultMaxEntries = 10000

// memEntry 缓存条目
type memEntry struct {
	key       string
	value     string
	set       map[string]struct{} // 集合类型
	zset      map[string]float64  // 有序集合类型
	expiresAt time.Time           // 零值表示永不过期
}

// expired 判断条目是否已过期
func (e *memEntry) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && now.After(e.expiresAt)
}

// MemoryCache 进程内缓存
//
// 组合了 TTL 过期与 LRU 淘汰：读操作会把条目移到队首，
// 容量超出时从队尾（最久未使用）开始淘汰，适合桌面端单进程场景。
type MemoryCache struct {
	mu      sync.Mutex
	items   map[string]*list.Element
	order   *list.List // 队首为最近使用
	maxSize int
}

// NewMemoryCache 创建进程内缓存
func NewMemoryCache(maxEntries int) *MemoryCache {
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	return &MemoryCache{
		items:   make(map[string]*list.Element),
		order:   list.New(),
		maxSize: maxEntries,
	}
}

// get 读取条目并在必要时惰性删除过期项
func (c *MemoryCache) get(key string) *memEntry {
	elem, ok := c.items[key]
	if !ok {
		return nil
	}

	entry := elem.Value.(*memEntry)
	if entry.expired(time.Now()) {
		c.order.Remove(elem)
		delete(c.items, key)
		return nil
	}

	c.order.MoveToFront(elem)
	return entry
}

// put 写入条目
func (c *MemoryCache) put(key string, entry *memEntry, ttl time.Duration) {
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}

	if elem, ok := c.items[key]; ok {
		// 已存在则原地替换并刷新位置
		elem.Value = entry
		c.order.MoveToFront(elem)
		return
	}

	entry.key = key
	c.items[key] = c.order.PushFront(entry)

	// LRU 淘汰
	for len(c.items) > c.maxSize {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		c.order.Remove(oldest)
		delete(c.items, oldest.Value.(*memEntry).key)
	}
}

// Get 读取字符串值
func (c *MemoryCache) Get(ctx context.Context, key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := c.get(key)
	if entry == nil {
		return "", ErrCacheMiss
	}
	return entry.value, nil
}

// Set 写入字符串值
func (c *MemoryCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.put(key, &memEntry{value: value}, ttl)
	return nil
}

// Delete 删除键
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.order.Remove(elem)
		delete(c.items, key)
	}
	return nil
}

// SAdd 向集合添加成员
func (c *MemoryCache) SAdd(ctx context.Context, key string, members ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := c.get(key)
	if entry == nil {
		entry = &memEntry{set: make(map[string]struct{})}
		c.put(key, entry, 0)
	}
	if entry.set == nil {
		entry.set = make(map[string]struct{})
	}
	for _, m := range members {
		entry.set[m] = struct{}{}
	}
	return nil
}

// SMembers 读取集合全部成员
func (c *MemoryCache) SMembers(ctx context.Context, key string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := c.get(key)
	if entry == nil || entry.set == nil {
		return nil, ErrCacheMiss
	}

	members := make([]string, 0, len(entry.set))
	for m := range entry.set {
		members = append(members, m)
	}
	sort.Strings(members)
	return members, nil
}

// SetMembers 整体覆盖集合内容
func (c *MemoryCache) SetMembers(ctx context.Context, key string, members []string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	set := make(map[string]struct{}, len(members))
	for _, m := range members {
		set[m] = struct{}{}
	}
	c.put(key, &memEntry{set: set}, ttl)
	return nil
}

// ZAdd 向有序集合添加成员
func (c *MemoryCache) ZAdd(ctx context.Context, key string, members ...ZMember) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := c.get(key)
	if entry == nil {
		entry = &memEntry{zset: make(map[string]float64)}
		c.put(key, entry, 0)
	}
	if entry.zset == nil {
		entry.zset = make(map[string]float64)
	}
	for _, m := range members {
		entry.zset[m.Member] = m.Score
	}
	return nil
}

// SortedRevRange 按分数倒序读取有序集合成员
func (c *MemoryCache) SortedRevRange(ctx context.Context, key string, count int) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := c.get(key)
	if entry == nil || entry.zset == nil {
		return nil, ErrCacheMiss
	}

	pairs := make([]ZMember, 0, len(entry.zset))
	for member, score := range entry.zset {
		pairs = append(pairs, ZMember{Member: member, Score: score})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Score == pairs[j].Score {
			return pairs[i].Member < pairs[j].Member
		}
		return pairs[i].Score > pairs[j].Score
	})

	if count > 0 && len(pairs) > count {
		pairs = pairs[:count]
	}

	members := make([]string, 0, len(pairs))
	for _, p := range pairs {
		members = append(members, p.Member)
	}
	return members, nil
}

// Ping 健康检查
func (c *MemoryCache) Ping(ctx context.Context) error {
	return nil
}

// Close 释放资源
func (c *MemoryCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.order.Init()
	return nil
}

// Len 返回当前缓存条目数（用于测试与监控）
func (c *MemoryCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// String 便于日志输出
func (c *MemoryCache) String() string {
	return fmt.Sprintf("memory-cache(max=%d)", c.maxSize)
}

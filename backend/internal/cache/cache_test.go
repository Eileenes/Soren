package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCacheSetGet(t *testing.T) {
	c := NewMemoryCache(10)
	ctx := context.Background()

	if _, err := c.Get(ctx, "missing"); err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss for absent key, got %v", err)
	}

	if err := c.Set(ctx, "key", "value", 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	value, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if value != "value" {
		t.Errorf("expected %q, got %q", "value", value)
	}
}

func TestMemoryCacheExpiresEntries(t *testing.T) {
	c := NewMemoryCache(10)
	ctx := context.Background()

	if err := c.Set(ctx, "ephemeral", "value", 15*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if _, err := c.Get(ctx, "ephemeral"); err != ErrCacheMiss {
		t.Errorf("expected entry to expire, got %v", err)
	}
	if c.Len() != 0 {
		t.Errorf("expired entry should be reclaimed, Len=%d", c.Len())
	}
}

func TestMemoryCacheEvictsLeastRecentlyUsed(t *testing.T) {
	c := NewMemoryCache(3)
	ctx := context.Background()

	for _, key := range []string{"a", "b", "c"} {
		if err := c.Set(ctx, key, key, 0); err != nil {
			t.Fatalf("set %s failed: %v", key, err)
		}
	}

	// 访问 a 与 b，使 c 成为最久未使用
	if _, err := c.Get(ctx, "a"); err != nil {
		t.Fatalf("get a failed: %v", err)
	}
	if _, err := c.Get(ctx, "b"); err != nil {
		t.Fatalf("get b failed: %v", err)
	}

	if err := c.Set(ctx, "d", "d", 0); err != nil {
		t.Fatalf("set d failed: %v", err)
	}

	if _, err := c.Get(ctx, "c"); err != ErrCacheMiss {
		t.Errorf("expected c to be evicted, got %v", err)
	}
	if _, err := c.Get(ctx, "a"); err != nil {
		t.Errorf("expected a to survive eviction, got %v", err)
	}
	if c.Len() > 3 {
		t.Errorf("cache exceeded capacity: %d", c.Len())
	}
}

func TestMemoryCacheDelete(t *testing.T) {
	c := NewMemoryCache(5)
	ctx := context.Background()

	_ = c.Set(ctx, "key", "value", 0)
	if err := c.Delete(ctx, "key"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, err := c.Get(ctx, "key"); err != ErrCacheMiss {
		t.Errorf("expected key to be deleted, got %v", err)
	}
	// 删除不存在的键不应报错
	if err := c.Delete(ctx, "nonexistent"); err != nil {
		t.Errorf("deleting absent key should be a no-op, got %v", err)
	}
}

func TestMemoryCacheSets(t *testing.T) {
	c := NewMemoryCache(5)
	ctx := context.Background()

	if _, err := c.SMembers(ctx, "set"); err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss for absent set, got %v", err)
	}

	if err := c.SAdd(ctx, "set", "a", "b", "a"); err != nil {
		t.Fatalf("sadd failed: %v", err)
	}
	members, err := c.SMembers(ctx, "set")
	if err != nil {
		t.Fatalf("smembers failed: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 unique members, got %v", members)
	}

	// 整体覆盖
	if err := c.SetMembers(ctx, "set", []string{"x", "y", "z"}, time.Hour); err != nil {
		t.Fatalf("setmembers failed: %v", err)
	}
	members, _ = c.SMembers(ctx, "set")
	if len(members) != 3 {
		t.Errorf("expected 3 members after overwrite, got %v", members)
	}
}

func TestMemoryCacheSortedSetOrdering(t *testing.T) {
	c := NewMemoryCache(5)
	ctx := context.Background()

	if err := c.ZAdd(ctx, "zset",
		ZMember{Member: "old", Score: 100},
		ZMember{Member: "new", Score: 900},
		ZMember{Member: "mid", Score: 500},
	); err != nil {
		t.Fatalf("zadd failed: %v", err)
	}

	members, err := c.SortedRevRange(ctx, "zset", 0)
	if err != nil {
		t.Fatalf("sorted rev range failed: %v", err)
	}
	expected := []string{"new", "mid", "old"}
	for i, want := range expected {
		if members[i] != want {
			t.Errorf("position %d: expected %q, got %q (full: %v)", i, want, members[i], members)
		}
	}

	// 限制数量
	limited, err := c.SortedRevRange(ctx, "zset", 2)
	if err != nil {
		t.Fatalf("sorted rev range failed: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("expected 2 members, got %v", limited)
	}

	// 同分数按成员名排序，保证结果稳定
	_ = c.ZAdd(ctx, "tie", ZMember{Member: "b", Score: 1}, ZMember{Member: "a", Score: 1})
	tied, _ := c.SortedRevRange(ctx, "tie", 0)
	if len(tied) != 2 || tied[0] != "a" {
		t.Errorf("expected deterministic tie ordering, got %v", tied)
	}
}

func TestMemoryCacheScoresAreUpdated(t *testing.T) {
	c := NewMemoryCache(5)
	ctx := context.Background()

	_ = c.ZAdd(ctx, "zset", ZMember{Member: "item", Score: 1})
	_ = c.ZAdd(ctx, "zset", ZMember{Member: "item", Score: 999})

	members, _ := c.SortedRevRange(ctx, "zset", 0)
	if len(members) != 1 {
		t.Fatalf("expected single member after score update, got %v", members)
	}
}

func TestMemoryCacheClose(t *testing.T) {
	c := NewMemoryCache(5)
	ctx := context.Background()

	_ = c.Set(ctx, "key", "value", 0)
	if err := c.Ping(ctx); err != nil {
		t.Errorf("memory cache ping should succeed: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if c.Len() != 0 {
		t.Errorf("close should clear the cache, Len=%d", c.Len())
	}
}

func TestNewSelectsDriver(t *testing.T) {
	// 默认（含未知值）回落到内存缓存
	for _, driver := range []string{"memory", "", "unknown"} {
		c, err := New(driver, "", 10)
		if err != nil {
			t.Fatalf("driver %q should not error: %v", driver, err)
		}
		if _, ok := c.(*MemoryCache); !ok {
			t.Errorf("driver %q should produce a MemoryCache, got %T", driver, c)
		}
	}
}

func TestNewMemoryCacheDefaultCapacity(t *testing.T) {
	c := NewMemoryCache(0)
	if c.maxSize != defaultMaxEntries {
		t.Errorf("expected default capacity %d, got %d", defaultMaxEntries, c.maxSize)
	}
}

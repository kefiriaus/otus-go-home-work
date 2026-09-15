package hw04lrucache

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func requirePresent(t *testing.T, c Cache, key Key, want interface{}) {
	t.Helper()

	val, ok := c.Get(key)
	require.True(t, ok, "ключ %q должен быть в кэше", key)
	require.Equal(t, want, val)
}

func requireAbsent(t *testing.T, c Cache, key Key) {
	t.Helper()

	val, ok := c.Get(key)
	require.False(t, ok, "ключ %q должен быть вытолкнут", key)
	require.Nil(t, val)
}

//nolint:funlen
func TestCache(t *testing.T) {
	t.Run("empty cache", func(t *testing.T) {
		c := NewCache(10)

		_, ok := c.Get("aaa")
		require.False(t, ok)

		_, ok = c.Get("bbb")
		require.False(t, ok)
	})

	t.Run("simple", func(t *testing.T) {
		c := NewCache(5)

		wasInCache := c.Set("aaa", 100)
		require.False(t, wasInCache)

		wasInCache = c.Set("bbb", 200)
		require.False(t, wasInCache)

		val, ok := c.Get("aaa")
		require.True(t, ok)
		require.Equal(t, 100, val)

		val, ok = c.Get("bbb")
		require.True(t, ok)
		require.Equal(t, 200, val)

		wasInCache = c.Set("aaa", 300)
		require.True(t, wasInCache)

		val, ok = c.Get("aaa")
		require.True(t, ok)
		require.Equal(t, 300, val)

		val, ok = c.Get("ccc")
		require.False(t, ok)
		require.Nil(t, val)
	})

	t.Run("purge logic", func(t *testing.T) {
		c := NewCache(3)

		c.Set("a", 1)
		c.Set("b", 2)
		c.Set("c", 3)
		c.Set("d", 4) // очередь переполнена — вылетает "a"

		requireAbsent(t, c, "a")
		requirePresent(t, c, "b", 2)
		requirePresent(t, c, "c", 3)
		requirePresent(t, c, "d", 4)

		c.Set("e", 5)
		require.Equal(t, 3, len(c.(*lruCache).items))
	})

	t.Run("purge least recently used", func(t *testing.T) {
		c := NewCache(3)

		c.Set("a", 1)
		c.Set("b", 2)
		c.Set("c", 3)

		requirePresent(t, c, "a", 1)

		wasInCache := c.Set("b", 22)
		require.True(t, wasInCache)

		c.Set("d", 4)

		requireAbsent(t, c, "c")
		requirePresent(t, c, "b", 22)
		requirePresent(t, c, "a", 1)
		requirePresent(t, c, "d", 4)
	})

	t.Run("get refreshes recency", func(t *testing.T) {
		c := NewCache(2)

		c.Set("a", 1)
		c.Set("b", 2)

		requirePresent(t, c, "a", 1) // "a" снова самый свежий

		c.Set("c", 3)

		requireAbsent(t, c, "b")
		requirePresent(t, c, "a", 1)
		requirePresent(t, c, "c", 3)
	})

	t.Run("update does not evict", func(t *testing.T) {
		c := NewCache(2)

		c.Set("a", 1)
		c.Set("b", 2)

		for i := 0; i < 10; i++ {
			require.True(t, c.Set("a", i))
			require.True(t, c.Set("b", i))
		}

		requirePresent(t, c, "a", 9)
		requirePresent(t, c, "b", 9)
	})

	t.Run("capacity of one", func(t *testing.T) {
		c := NewCache(1)

		c.Set("a", 1)
		requirePresent(t, c, "a", 1)

		c.Set("b", 2)
		requireAbsent(t, c, "a")
		requirePresent(t, c, "b", 2)
	})

	t.Run("zero capacity", func(t *testing.T) {
		c := NewCache(0)

		require.False(t, c.Set("a", 1))
		requireAbsent(t, c, "a")
	})

	t.Run("clear", func(t *testing.T) {
		c := NewCache(3)

		c.Set("a", 1)
		c.Set("b", 2)
		c.Clear()

		requireAbsent(t, c, "a")
		requireAbsent(t, c, "b")

		require.False(t, c.Set("a", 10), "после Clear ключ считается новым")
		requirePresent(t, c, "a", 10)

		c.Set("b", 20)
		c.Set("c", 30)
		c.Set("d", 40)
		requireAbsent(t, c, "a")
	})

	t.Run("nil and mixed values", func(t *testing.T) {
		c := NewCache(3)

		c.Set("nil", nil)
		c.Set("str", "value")
		c.Set("struct", struct{ A int }{A: 1})

		val, ok := c.Get("nil")
		require.True(t, ok, "nil-значение должно отличаться от отсутствия ключа")
		require.Nil(t, val)

		requirePresent(t, c, "str", "value")
		requirePresent(t, c, "struct", struct{ A int }{A: 1})
	})

	t.Run("keys are independent across caches", func(t *testing.T) {
		a := NewCache(2)
		b := NewCache(2)

		a.Set("k", 1)
		b.Set("k", 2)

		requirePresent(t, a, "k", 1)
		requirePresent(t, b, "k", 2)
	})
}

func TestCacheMultithreading(_ *testing.T) {
	c := NewCache(10)
	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 1_000_000; i++ {
			c.Set(Key(strconv.Itoa(i)), i)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 1_000_000; i++ {
			c.Get(Key(strconv.Itoa(rand.Intn(1_000_000))))
		}
	}()

	wg.Wait()
}

package tinylfu_test

import (
	"crypto/rand"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/go-tinylfu"
)

func TestCache(t *testing.T) {
	cache := tinylfu.New(1e3, 10e3)
	keys := []string{"one", "two", "three"}

	for _, key := range keys {
		cache.Set(&tinylfu.Item{
			Key:   key,
			Value: key,
		})

		got, ok := cache.Get(key)
		require.True(t, ok)
		require.Equal(t, key, got)
	}

	for _, key := range keys {
		got, ok := cache.Get(key)
		require.True(t, ok)
		require.Equal(t, key, got)

		cache.Set(&tinylfu.Item{
			Key:   key,
			Value: key + key,
		})
	}

	for _, key := range keys {
		got, ok := cache.Get(key)
		require.True(t, ok)
		require.Equal(t, key+key, got)
	}

	for _, key := range keys {
		cache.Del(key)
	}

	for _, key := range keys {
		_, ok := cache.Get(key)
		require.False(t, ok)
	}
}

func TestOOM(t *testing.T) {
	keys := make([]string, 10000)
	for i := range keys {
		keys[i] = randWord()
	}

	cache := tinylfu.New(1e3, 10e3)

	for i := 0; i < 5e6; i++ {
		key := keys[i%len(keys)]
		cache.Set(&tinylfu.Item{
			Key:   key,
			Value: key,
		})
	}
}

func randWord() string {
	buf := make([]byte, 64)
	io.ReadFull(rand.Reader, buf)
	return string(buf)
}

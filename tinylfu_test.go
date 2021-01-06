package tinylfu_test

import (
	"math/rand"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmihailenco/go-tinylfu"
)

func TestGinkgo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "kafkaq")
}

var _ = Describe("tinylfu", func() {
	It("works", func() {
		cache := tinylfu.New(1e3, 100e3)
		keys := []string{"one", "two", "three"}

		for _, key := range keys {
			cache.Set(&tinylfu.Item{
				Key:   key,
				Value: key,
			})

			got, ok := cache.Get(key)
			Expect(ok).To(BeTrue())
			Expect(got).To(Equal(key))
		}

		for _, key := range keys {
			got, ok := cache.Get(key)
			Expect(ok).To(BeTrue())
			Expect(got).To(Equal(key))

			cache.Set(&tinylfu.Item{
				Key:   key,
				Value: key + key,
			})
		}

		for _, key := range keys {
			got, ok := cache.Get(key)
			Expect(ok).To(BeTrue())
			Expect(got).To(Equal(key + key))
		}

		for _, key := range keys {
			cache.Del(key)
		}

		for _, key := range keys {
			got, ok := cache.Get(key)
			Expect(ok).To(BeFalse())
			Expect(got).To(BeNil())
		}
	})

	It("does not OOM", func() {
		cache := tinylfu.New(1e3, 100e3)

		for i := 0; i < 10e6; i++ {
			key := randString(10)
			cache.Set(&tinylfu.Item{
				Key:   key,
				Value: key,
			})
		}
	})
})

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func BenchmarkSet(b *testing.B) {
	cache := tinylfu.New(1e3, 100e3)

	for i := 0; i < b.N; i++ {
		key := randString(10)
		cache.Set(&tinylfu.Item{
			Key:   key,
			Value: key,
		})
	}
}

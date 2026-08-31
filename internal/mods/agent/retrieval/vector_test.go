package retrieval

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalEmbeddingIsDeterministicAndRanksChineseAndEnglishTerms(t *testing.T) {
	first := EmbedText("Kimi 的 Responses API 支持函数调用和结构化输出。")
	second := EmbedText("Kimi 的 Responses API 支持函数调用和结构化输出。")
	require.Len(t, first, LocalEmbeddingDimension)
	assert.Equal(t, first, second)

	var magnitude float64
	for _, value := range first {
		magnitude += float64(value * value)
	}
	assert.InDelta(t, 1, math.Sqrt(magnitude), 1e-6)

	records := []VectorRecord{
		{ChunkID: "kimi", Vector: EmbedText("Kimi 的 Responses API 支持函数调用和结构化输出。")},
		{ChunkID: "local", Vector: EmbedText("Agent 使用本地词法索引检索知识库文档。")},
		{ChunkID: "english", Vector: EmbedText("The agent stores documents in a local lexical index.")},
	}
	chinese := TopK(records, EmbedText("Kimi 函数调用"), 1)
	require.Len(t, chinese, 1)
	assert.Equal(t, "kimi", chinese[0].ChunkID)
	english := TopK(records, EmbedText("local lexical index"), 1)
	require.Len(t, english, 1)
	assert.Equal(t, "english", english[0].ChunkID)
}

func TestFloat32RoundTripAndInvalidEncoding(t *testing.T) {
	want := []float32{1, -2.5, 0.125}
	got, err := DecodeFloat32(EncodeFloat32(want))
	require.NoError(t, err)
	assert.Equal(t, want, got)
	_, err = DecodeFloat32([]byte{1, 2, 3})
	assert.Error(t, err)
}

func TestCosineAndTopKStableOrdering(t *testing.T) {
	records := []VectorRecord{
		{ChunkID: "b", Vector: []float32{1, 0}},
		{ChunkID: "a", Vector: []float32{1, 0}},
		{ChunkID: "c", Vector: []float32{0, 1}},
	}
	got := TopK(records, []float32{1, 0}, 2)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].ChunkID)
	assert.Equal(t, "b", got[1].ChunkID)
	assert.InDelta(t, 1, got[0].Score, 1e-6)
	assert.Equal(t, float64(-1), Cosine([]float32{1}, []float32{1, 2}))
}

func TestCacheIsolationAndInvalidation(t *testing.T) {
	cache := NewCache()
	cache.Put("kb-a", []VectorRecord{{ChunkID: "a"}})
	cache.Put("kb-b", []VectorRecord{{ChunkID: "b"}})
	items, ok := cache.Get("kb-a")
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.Equal(t, "a", items[0].ChunkID)
	cache.Invalidate("kb-a")
	_, ok = cache.Get("kb-a")
	assert.False(t, ok)
	_, ok = cache.Get("kb-b")
	assert.True(t, ok)
}

func TestCacheExpires(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	cache := NewCache()
	cache.ttl = time.Minute
	cache.now = func() time.Time { return now }
	cache.Put("kb", []VectorRecord{{ChunkID: "chunk"}})

	_, ok := cache.Get("kb")
	require.True(t, ok)
	now = now.Add(time.Minute)
	_, ok = cache.Get("kb")
	assert.False(t, ok)
}

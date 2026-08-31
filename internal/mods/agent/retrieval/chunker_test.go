package retrieval

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunkTextMarkdownAndLines(t *testing.T) {
	raw := "# 标题\r\n第一段中文。\r\n\r\n## Second\r\nline four\r\nline five"
	chunks := ChunkText(raw, 24, 5)
	require.NotEmpty(t, chunks)
	for _, chunk := range chunks {
		assert.LessOrEqual(t, utf8.RuneCountInString(chunk.Content), 24)
		assert.GreaterOrEqual(t, chunk.LineStart, 1)
		assert.GreaterOrEqual(t, chunk.LineEnd, chunk.LineStart)
	}
	assert.Equal(t, 1, chunks[0].LineStart)
	assert.Contains(t, strings.Join(func() []string {
		out := make([]string, len(chunks))
		for i := range chunks {
			out[i] = chunks[i].Content
		}
		return out
	}(), "\n"), "## Second")
}

func TestChunkTextLongLineAndInvalidConfig(t *testing.T) {
	chunks := ChunkText(strings.Repeat("界", 45), 20, 4)
	require.GreaterOrEqual(t, len(chunks), 3)
	for _, chunk := range chunks {
		assert.LessOrEqual(t, utf8.RuneCountInString(chunk.Content), 20)
		assert.Equal(t, 1, chunk.LineStart)
		assert.Equal(t, 1, chunk.LineEnd)
	}
	assert.Nil(t, ChunkText("text", 10, 10))
	assert.Nil(t, ChunkText("text", 0, 0))
}

func TestChunkTextOverlap(t *testing.T) {
	chunks := ChunkText("one two three\nfour five six\nseven eight nine", 24, 8)
	require.GreaterOrEqual(t, len(chunks), 2)
	assert.NotEmpty(t, chunks[0].Content)
	assert.NotEmpty(t, chunks[1].Content)
}

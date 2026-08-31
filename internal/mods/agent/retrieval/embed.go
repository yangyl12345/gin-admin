package retrieval

import (
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

const LocalEmbeddingDimension = 512

// EmbedTexts creates deterministic local vectors for the lexical retrieval
// index. It keeps document indexing independent from remote embedding models.
func EmbedTexts(texts []string) [][]float32 {
	vectors := make([][]float32, len(texts))
	for i, text := range texts {
		vectors[i] = EmbedText(text)
	}
	return vectors
}

func EmbedText(text string) []float32 {
	vector := make([]float32, LocalEmbeddingDimension)
	tokens := lexicalTokens(text)
	for _, token := range tokens {
		addToken(vector, "u:"+token, 1)
	}
	for i := 0; i+1 < len(tokens); i++ {
		addToken(vector, "b:"+tokens[i]+"\\x00"+tokens[i+1], 0.75)
	}
	if len(tokens) == 0 && strings.TrimSpace(text) != "" {
		addToken(vector, "raw:"+strings.ToLower(strings.TrimSpace(text)), 1)
	}
	var magnitude float64
	for _, value := range vector {
		magnitude += float64(value * value)
	}
	if magnitude == 0 {
		return vector
	}
	scale := float32(1 / math.Sqrt(magnitude))
	for i := range vector {
		vector[i] *= scale
	}
	return vector
}

func lexicalTokens(text string) []string {
	var tokens []string
	var word strings.Builder
	flushWord := func() {
		if word.Len() > 0 {
			tokens = append(tokens, strings.ToLower(word.String()))
			word.Reset()
		}
	}
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			flushWord()
			tokens = append(tokens, string(r))
			continue
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			word.WriteRune(r)
			continue
		}
		flushWord()
	}
	flushWord()
	return tokens
}

func addToken(vector []float32, token string, weight float32) {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(token))
	value := hash.Sum64()
	index := value % LocalEmbeddingDimension
	if value>>63 == 0 {
		vector[index] += weight
		return
	}
	vector[index] -= weight
}

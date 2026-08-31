package retrieval

import (
	"encoding/binary"
	"errors"
	"math"
	"sort"
)

func EncodeFloat32(values []float32) []byte {
	buf := make([]byte, len(values)*4)
	for i, value := range values {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(value))
	}
	return buf
}

func DecodeFloat32(buf []byte) ([]float32, error) {
	if len(buf)%4 != 0 {
		return nil, errors.New("invalid float32 vector encoding")
	}
	values := make([]float32, len(buf)/4)
	for i := range values {
		values[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return values, nil
}

func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return -1
	}
	var dot, aa, bb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		aa += x * x
		bb += y * y
	}
	if aa == 0 || bb == 0 {
		return -1
	}
	return dot / (math.Sqrt(aa) * math.Sqrt(bb))
}

type VectorRecord struct {
	ChunkID      string
	DocumentID   string
	DocumentName string
	Content      string
	LineStart    int
	LineEnd      int
	Vector       []float32
}

type ScoredRecord struct {
	VectorRecord
	Score float64
}

func TopK(records []VectorRecord, query []float32, k int) []ScoredRecord {
	if k <= 0 {
		return []ScoredRecord{}
	}
	result := make([]ScoredRecord, 0, len(records))
	for _, record := range records {
		score := Cosine(record.Vector, query)
		if score >= -0.5 {
			result = append(result, ScoredRecord{VectorRecord: record, Score: score})
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			return result[i].ChunkID < result[j].ChunkID
		}
		return result[i].Score > result[j].Score
	})
	if len(result) > k {
		result = result[:k]
	}
	return result
}

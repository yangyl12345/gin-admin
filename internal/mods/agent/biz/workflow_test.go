package biz

import (
	"testing"

	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/schema"
	"github.com/stretchr/testify/assert"
)

func TestValidCitationIDsRejectsUnknownChunks(t *testing.T) {
	hits := []schema.RetrievalHit{{ChunkID: "chunk-a", LineStart: 1, LineEnd: 2}, {ChunkID: "chunk-b", LineStart: 3, LineEnd: 3}}
	assert.True(t, validCitationIDs([]string{"chunk-a", "chunk-b"}, hits))
	assert.True(t, validCitationIDs(nil, hits))
	assert.False(t, validCitationIDs([]string{"chunk-a", "invented"}, hits))
	assert.False(t, validCitationIDs([]string{"chunk-b"}, []schema.RetrievalHit{{ChunkID: "chunk-b", LineStart: 4, LineEnd: 3}}))
}

func TestWorkflowSchemasAreStrict(t *testing.T) {
	for _, outputSchema := range []map[string]any{supervisorSchema, answerSchema, reviewerSchema} {
		assert.Equal(t, false, outputSchema["additionalProperties"])
		assert.Equal(t, "object", outputSchema["type"])
		assert.NotEmpty(t, outputSchema["required"])
	}
}

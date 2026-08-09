package providers

import (
	"os"
	"path/filepath"
	"testing"

	"pentagi/pkg/database"
	resourcefiles "pentagi/pkg/resources"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vxcontrol/langchaingo/llms"
)

func TestBuildAssistantHumanMessageIncludesImageBytes(t *testing.T) {
	dataDir := t.TempDir()
	hash := "0123456789abcdef0123456789abcdef"
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	blobPath := resourcefiles.BlobPath(dataDir, hash)
	require.NoError(t, os.MkdirAll(filepath.Dir(blobPath), 0o755))
	require.NoError(t, os.WriteFile(blobPath, png, 0o600))

	message, err := buildAssistantHumanMessage(dataDir, "inspect this", []database.UserResource{
		{Hash: hash, Name: "evidence.png", Path: "evidence.png", Size: int64(len(png))},
	})
	require.NoError(t, err)
	require.Len(t, message.Parts, 2)
	assert.Equal(t, llms.ChatMessageTypeHuman, message.Role)
	assert.Equal(t, "inspect this", message.Parts[0].(llms.TextContent).Text)
	image := message.Parts[1].(llms.BinaryContent)
	assert.Equal(t, "image/png", image.MIMEType)
	assert.Equal(t, png, image.Data)
}

func TestBuildAssistantHumanMessageLeavesNonImagesAsFiles(t *testing.T) {
	message, err := buildAssistantHumanMessage(t.TempDir(), "read the report", []database.UserResource{
		{Name: "report.pdf", Path: "report.pdf", Size: 100},
	})
	require.NoError(t, err)
	require.Len(t, message.Parts, 1)
	assert.Equal(t, "read the report", message.Parts[0].(llms.TextContent).Text)
}

func TestBuildAssistantHumanMessageRejectsOversizedImage(t *testing.T) {
	_, err := buildAssistantHumanMessage(t.TempDir(), "inspect", []database.UserResource{
		{Name: "large.png", Path: "large.png", Size: maxAssistantImageSize + 1},
	})
	require.ErrorContains(t, err, "exceeds the 10 MiB limit")
}

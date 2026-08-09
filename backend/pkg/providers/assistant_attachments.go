package providers

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"pentagi/pkg/database"
	resourcefiles "pentagi/pkg/resources"

	"github.com/vxcontrol/langchaingo/llms"
)

const (
	maxAssistantImageSize       = 10 << 20
	maxAssistantImagesTotalSize = 20 << 20
)

var supportedAssistantImageTypes = map[string]struct{}{
	"image/gif":  {},
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
}

func buildAssistantHumanMessage(
	dataDir, prompt string,
	attachments []database.UserResource,
) (llms.MessageContent, error) {
	parts := []llms.ContentPart{llms.TextContent{Text: prompt}}
	totalSize := int64(0)

	for _, attachment := range attachments {
		if attachment.IsDir || !isAssistantImageName(attachment.Name) {
			continue
		}
		if attachment.Size > maxAssistantImageSize {
			return llms.MessageContent{}, fmt.Errorf(
				"image attachment %q exceeds the %d MiB limit",
				attachment.Name, maxAssistantImageSize>>20,
			)
		}
		if totalSize+attachment.Size > maxAssistantImagesTotalSize {
			return llms.MessageContent{}, fmt.Errorf(
				"image attachments exceed the %d MiB total limit",
				maxAssistantImagesTotalSize>>20,
			)
		}

		data, err := os.ReadFile(resourcefiles.BlobPath(dataDir, attachment.Hash))
		if err != nil {
			return llms.MessageContent{}, fmt.Errorf("failed to read image attachment %q: %w", attachment.Name, err)
		}
		actualSize := int64(len(data))
		if actualSize > maxAssistantImageSize {
			return llms.MessageContent{}, fmt.Errorf(
				"image attachment %q exceeds the %d MiB limit",
				attachment.Name, maxAssistantImageSize>>20,
			)
		}
		if totalSize+actualSize > maxAssistantImagesTotalSize {
			return llms.MessageContent{}, fmt.Errorf(
				"image attachments exceed the %d MiB total limit",
				maxAssistantImagesTotalSize>>20,
			)
		}
		mimeType := http.DetectContentType(data[:min(len(data), 512)])
		if _, supported := supportedAssistantImageTypes[mimeType]; !supported {
			return llms.MessageContent{}, fmt.Errorf(
				"image attachment %q has unsupported content type %q",
				attachment.Name, mimeType,
			)
		}

		parts = append(parts, llms.BinaryContent{MIMEType: mimeType, Data: data})
		totalSize += actualSize
	}

	return llms.MessageContent{Role: llms.ChatMessageTypeHuman, Parts: parts}, nil
}

func isAssistantImageName(name string) bool {
	lowerName := strings.ToLower(name)
	return strings.HasSuffix(lowerName, ".gif") ||
		strings.HasSuffix(lowerName, ".jpeg") ||
		strings.HasSuffix(lowerName, ".jpg") ||
		strings.HasSuffix(lowerName, ".png") ||
		strings.HasSuffix(lowerName, ".webp")
}

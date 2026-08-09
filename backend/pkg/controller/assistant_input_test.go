package controller

import (
	"testing"

	"pentagi/pkg/database"

	"github.com/stretchr/testify/assert"
)

func TestFormatAssistantInputLogRendersImages(t *testing.T) {
	message := formatAssistantInputLog("Please inspect this", []database.UserResource{
		{Name: "screen shot.png", Path: "evidence/screen shot.png"},
		{Name: "notes.pdf", Path: "notes.pdf"},
	})

	assert.Contains(t, message, "Please inspect this")
	assert.Contains(t, message, "![Attached image: screen shot.png]")
	assert.Contains(t, message, "inline=true&paths%5B%5D=evidence%2Fscreen+shot.png")
	assert.NotContains(t, message, "notes.pdf")
}

func TestFormatAssistantInputLogSupportsImageOnlyInput(t *testing.T) {
	message := formatAssistantInputLog("", []database.UserResource{
		{Name: "evidence.webp", Path: "evidence.webp"},
	})

	assert.Equal(t,
		"![Attached image: evidence.webp](/api/v1/resources/download?inline=true&paths%5B%5D=evidence.webp)",
		message,
	)
}

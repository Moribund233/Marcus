package agent

import (
	"strings"
	"testing"

	"Marcus/internal/model"
)

// TestContextCompressorShouldCompress 验证压缩触发判断。
func TestContextCompressorShouldCompress(t *testing.T) {
	c := NewContextCompressor(100, 0.8)

	short := []model.Message{
		{Role: model.RoleSystem, Content: "sys"},
		{Role: model.RoleUser, Content: "hi"},
	}
	if c.ShouldCompress(short) {
		t.Fatal("short history should not trigger compression")
	}

	long := []model.Message{
		{Role: model.RoleSystem, Content: strings.Repeat("a", 400)},
	}
	if !c.ShouldCompress(long) {
		t.Fatal("long history should trigger compression")
	}
}

// TestContextCompressorCompress 验证压缩保留最近消息。
func TestContextCompressorCompress(t *testing.T) {
	c := NewContextCompressor(100, 0.8)

	msgs := []model.Message{
		{Role: model.RoleSystem, Content: "system prompt"},
		{Role: model.RoleUser, Content: strings.Repeat("a", 100)},
		{Role: model.RoleAssistant, Content: strings.Repeat("b", 100)},
		{Role: model.RoleUser, Content: "latest question"},
	}

	compressed := c.Compress(msgs)
	if len(compressed) == 0 {
		t.Fatal("compressed should not be empty")
	}
	if compressed[0].Role != model.RoleSystem {
		t.Fatal("compressed should keep system prompt")
	}
	if compressed[len(compressed)-1].Content != "latest question" {
		t.Fatal("compressed should keep latest user message")
	}
}

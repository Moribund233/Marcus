# Chat UI Optimization Design

## Overview

Optimize the Marcus chat interface across four dimensions: message bubble layout + right-click context menu, streaming output debounce, LLM status indicators, and avatar alignment.

---

## 1. Chat Bubbles + Right-Click Menu

### Layout
- **User messages** → right-aligned, blue-tinted bubble (`bg-primary/10`), `User` icon avatar
- **Assistant messages** → left-aligned, muted bubble (`bg-muted/30`), app logo SVG avatar
- All messages render inside a styled "bubble" container with rounded corners

### Right-Click Context Menu
A popup menu appears on `contextmenu` event with three actions:

| Action | UI Effect | Backend Effect |
|--------|-----------|----------------|
| **Recall (撤回)** | Remove message + all subsequent messages from UI | Delete message + all later messages from DB; next LLM call uses truncated history |
| **Edit (编辑)** | Replace message text with an inline textarea; confirm saves | Update message content in DB; optionally re-trigger LLM to regenerate follow-up |
| **Delete (删除)** | Remove the single message from UI | Delete the single message from DB |

### Backend Additions to `internal/conversation/store.go`
- `UpdateMessage(id, content string)` — update a message's content
- `DeleteMessage(id string)` — delete a single message
- `RecallMessages(fromID string)` — delete a message and all messages after it (by created_at order)
- New Wails bindings in `internal/app/agent.go` and `main.go`

---

## 2. Streaming Output Debounce

### Root Cause Fix
`SendMessageStream` in `internal/app/agent.go` currently splits content by Go byte length (`len(content)`), which splits multi-byte UTF-8 characters (Chinese = 3 bytes). Fix:
```go
runes := []rune(content)
chunkSize := 4
for i := 0; i < len(runes); i += chunkSize {
    end := i + chunkSize
    if end > len(runes) { end = len(runes) }
    chunk := string(runes[i:end])
    // emit chunk
}
```

### Frontend Debounce
- Collect streaming chunks in a buffer ref
- Set a 150ms debounce timer on each chunk arrival
- On timer fire: flush buffer to display state
- On stream end: flush immediately

---

## 3. LLM Status Indicators

### Backend Streaming Phase Events
Emit typed events during agent execution instead of only after completion:

| Event Type | When Emitted | Frontend Display |
|-----------|-------------|-----------------|
| `thinking` | Agent starts reasoning | Pulsing dots + "Thinking..." |
| `tool_call` | Agent invokes a tool | Wrench icon + tool name + spinner |
| `code` | Agent outputs code block | Code block with language label |
| `fetch` | Agent fetches web URL | Globe icon + URL in status bubble |
| `text` | Normal text streaming | Standard text bubble |

### Event Payload
```typescript
{
  type: "thinking" | "tool_call" | "code" | "fetch" | "text",
  content: string,
  metadata?: { tool_name?: string, url?: string, language?: string }
}
```

The agent core (`internal/agent/core.go`) will emit these events through a callback that the Wails layer forwards as `chat:stream:phase` events.

---

## 4. Avatar

- Replace the `Bot` lucide icon in `ChatMessage.tsx` with the app's existing logo SVG component
- Locate the reusable logo component (likely in `frontend/src/components/common/` or embedded in `title-bar.tsx`)
- User messages keep the `User` lucide icon on the right side

---

## Files Changed

### Backend (Go)
| File | Change |
|------|--------|
| `internal/conversation/store.go` | Add `UpdateMessage`, `DeleteMessage`, `RecallMessages` |
| `internal/app/agent.go` | Fix UTF-8 split; add phase event callbacks; add new Wails bindings |
| `main.go` | Bind new methods |
| `internal/agent/core.go` | Add phase event emission during Run() |

### Frontend (React/TypeScript)
| File | Change |
|------|--------|
| `components/chat/ChatMessage.tsx` | Bubble layout, right-click menu, avatar swap |
| `components/chat/ChatPage.tsx` | Streaming phase handling, debounce |
| `hooks/useChat.ts` | Handle `chat:stream:phase` events |
| `locales/en-US.ts`, `locales/zh-CN.ts` | New i18n strings |
| (new) `components/chat/ChatContextMenu.tsx` | Reusable right-click menu |
| (new) `components/common/AppLogo.tsx` | Logo SVG component (if not existing) |

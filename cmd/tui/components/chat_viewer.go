package components

import (
	"fmt"
	"strings"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// ChatMessagesViewer displays session messages in a scrollable viewport.
type ChatMessagesViewer struct {
	viewport viewport.Model
	messages []claudeviewer.SessionMessageInfo
	width    int
	height   int
}

// NewChatMessagesViewer creates a new chat messages viewer component.
func NewChatMessagesViewer(width, height int) *ChatMessagesViewer {
	vp := viewport.New(width, height)
	vp.SetContent("")

	return &ChatMessagesViewer{
		viewport: vp,
		width:    width,
		height:   height,
		messages: []claudeviewer.SessionMessageInfo{},
	}
}

// SetMessages sets the messages to display.
func (c *ChatMessagesViewer) SetMessages(messages []claudeviewer.SessionMessageInfo) {
	c.messages = messages
	c.updateViewportContent()
}

// AppendMessage adds a new message to the end of the list.
func (c *ChatMessagesViewer) AppendMessage(msg claudeviewer.SessionMessageInfo) {
	c.messages = append(c.messages, msg)
	c.updateViewportContent()
	// Auto-scroll to bottom
	c.viewport.GotoBottom()
}

// AppendMessages adds multiple new messages to the end of the list.
func (c *ChatMessagesViewer) AppendMessages(messages []claudeviewer.SessionMessageInfo) {
	c.messages = append(c.messages, messages...)
	c.updateViewportContent()
	// Auto-scroll to bottom
	c.viewport.GotoBottom()
}

// SetSize updates the viewer dimensions.
func (c *ChatMessagesViewer) SetSize(width, height int) {
	c.width = width
	c.height = height
	c.viewport.Width = width
	c.viewport.Height = height
	c.updateViewportContent()
}

// Update handles messages for the viewport.
func (c *ChatMessagesViewer) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.viewport, cmd = c.viewport.Update(msg)
	return cmd
}

// View renders the chat messages viewer.
func (c *ChatMessagesViewer) View() string {
	return c.viewport.View()
}

// updateViewportContent refreshes the viewport with formatted messages.
func (c *ChatMessagesViewer) updateViewportContent() {
	if len(c.messages) == 0 {
		c.viewport.SetContent(styles.LoadingStyle.Render("No messages yet. Start chatting!"))
		return
	}

	var sb strings.Builder

	for i, msg := range c.messages {
		if i > 0 {
			sb.WriteString("\n")
		}

		// Format message based on role
		if msg.Role != nil && *msg.Role == "user" {
			// User message
			sb.WriteString(styles.AccentStyle.Render("You") + " " + styles.AccentStyle.Render(msg.Timestamp) + "\n")
			if msg.Content != nil {
				sb.WriteString(*msg.Content)
			}
		} else if msg.Role != nil && *msg.Role == "assistant" {
			// Assistant message
			sb.WriteString(styles.MetaStyle.Render("Claude") + " " + styles.MetaStyle.Render(msg.Timestamp) + "\n")
			if msg.Content != nil {
				sb.WriteString(*msg.Content)
			}
		} else {
			// System or other message
			sb.WriteString(styles.ActiveStyle.Render(fmt.Sprintf("[%s]", msg.MessageType)) + " " + styles.ActiveStyle.Render(msg.Timestamp) + "\n")
			if msg.Content != nil {
				sb.WriteString(styles.AccentStyle.Render(*msg.Content))
			}
		}

		sb.WriteString("\n")
	}

	c.viewport.SetContent(sb.String())
}

// ScrollPercent returns the current scroll percentage.
func (c *ChatMessagesViewer) ScrollPercent() float64 {
	return c.viewport.ScrollPercent()
}

// AtBottom returns true if scrolled to the bottom.
func (c *ChatMessagesViewer) AtBottom() bool {
	return c.viewport.AtBottom()
}

// GotoBottom scrolls to the bottom of the messages.
func (c *ChatMessagesViewer) GotoBottom() {
	c.viewport.GotoBottom()
}

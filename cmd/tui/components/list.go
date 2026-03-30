// Package components provides reusable TUI components.
package components

import (
	"fmt"
	"io"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ListItem represents an item in the list.
type ListItem struct {
	title       string
	description string
	data        interface{}
}

// NewListItem creates a new list item.
func NewListItem(title, description string, data interface{}) ListItem {
	return ListItem{
		title:       title,
		description: description,
		data:        data,
	}
}

// Title returns the item title (implements list.Item).
func (i ListItem) Title() string { return i.title }

// Description returns the item description (implements list.Item).
func (i ListItem) Description() string { return i.description }

// FilterValue returns the value to filter on (implements list.Item).
func (i ListItem) FilterValue() string { return i.title }

// Data returns the underlying data.
func (i ListItem) Data() interface{} { return i.data }

// ListItemDelegate is a custom delegate for rendering list items.
type ListItemDelegate struct{}

// NewListItemDelegate creates a new delegate with custom styling.
func NewListItemDelegate() ListItemDelegate {
	return ListItemDelegate{}
}

// Height returns the height of each item.
func (d ListItemDelegate) Height() int { return 1 }

// Spacing returns the spacing between items.
func (d ListItemDelegate) Spacing() int { return 0 }

// Update handles item updates.
func (d ListItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

// Render renders a list item.
func (d ListItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(ListItem)
	if !ok {
		return
	}

	var str string
	if index == m.Index() {
		str = styles.ActiveStyle.Render(fmt.Sprintf("❯ %s", i.Title()))
	} else {
		str = styles.InactiveStyle.Render(fmt.Sprintf("  %s", i.Title()))
	}

	fmt.Fprint(w, str)
}

// List wraps bubbles/list with custom styling and behavior.
type List struct {
	model  list.Model
	width  int
	height int
}

// NewList creates a new list component.
func NewList(items []ListItem, width, height int) *List {
	// Convert to list.Item slice.
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	delegate := NewListItemDelegate()
	l := list.New(listItems, delegate, width, height)

	// Customize the list appearance.
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	// Custom styles.
	l.Styles.NoItems = lipgloss.NewStyle().Foreground(styles.InactiveColor)

	return &List{
		model:  l,
		width:  width,
		height: height,
	}
}

// Update handles list updates.
func (l *List) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	l.model, cmd = l.model.Update(msg)
	return cmd
}

// View renders the list.
func (l *List) View() string {
	return l.model.View()
}

// SetSize updates the list dimensions.
func (l *List) SetSize(width, height int) {
	l.width = width
	l.height = height
	l.model.SetSize(width, height)
}

// SelectedItem returns the currently selected item, or nil if none.
func (l *List) SelectedItem() *ListItem {
	item := l.model.SelectedItem()
	if item == nil {
		return nil
	}
	if li, ok := item.(ListItem); ok {
		return &li
	}
	return nil
}

// SelectedIndex returns the index of the selected item.
func (l *List) SelectedIndex() int {
	return l.model.Index()
}

// SetItems replaces all items in the list.
func (l *List) SetItems(items []ListItem) tea.Cmd {
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}
	return l.model.SetItems(listItems)
}

// ItemCount returns the number of items.
func (l *List) ItemCount() int {
	return len(l.model.Items())
}

// Select moves selection to the given index.
func (l *List) Select(index int) {
	l.model.Select(index)
}

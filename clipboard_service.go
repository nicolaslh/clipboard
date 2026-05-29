package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.design/x/clipboard"
)

// ClipboardService monitors the system clipboard and provides management APIs.
type ClipboardService struct {
	store         *Store
	cancel        context.CancelFunc
	app           *application.App
	lastContent   string
	retentionDays int
}

// NewClipboardService creates a new ClipboardService.
func NewClipboardService(store *Store) *ClipboardService {
	return &ClipboardService{
		store:         store,
		retentionDays: 30, // default 30 days retention
	}
}

// ServiceStartup is called when the application starts.
func (s *ClipboardService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	err := clipboard.Init()
	if err != nil {
		return err
	}

	s.app = application.Get()

	// Read current clipboard content as baseline
	s.lastContent = string(clipboard.Read(clipboard.FmtText))

	// Clean expired items on startup
	deleted, err := s.store.CleanExpired(s.retentionDays)
	if err != nil {
		slog.Error("failed to clean expired items", "error", err)
	} else if deleted > 0 {
		slog.Info("cleaned expired clipboard items", "count", deleted)
	}

	// Start polling clipboard in background
	monitorCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	go s.pollClipboard(monitorCtx)
	go s.periodicCleanup(monitorCtx)

	return nil
}

// ServiceShutdown is called when the application is shutting down.
func (s *ClipboardService) ServiceShutdown() error {
	if s.cancel != nil {
		s.cancel()
	}
	return s.store.Close()
}

// pollClipboard checks the clipboard every 500ms for changes.
func (s *ClipboardService) pollClipboard(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			data := clipboard.Read(clipboard.FmtText)
			if len(data) == 0 {
				continue
			}
			content := string(data)
			if content == s.lastContent {
				continue
			}
			s.lastContent = content

			item, err := s.store.Save(content, "text")
			if err != nil {
				slog.Error("failed to save clipboard item", "error", err)
				continue
			}
			if item != nil {
				s.app.Event.Emit("clipboard:new", item)
			}
		}
	}
}

// periodicCleanup runs every hour to remove expired items.
func (s *ClipboardService) periodicCleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleted, err := s.store.CleanExpired(s.retentionDays)
			if err != nil {
				slog.Error("periodic cleanup failed", "error", err)
			} else if deleted > 0 {
				slog.Info("periodic cleanup removed items", "count", deleted)
				s.app.Event.Emit("clipboard:cleaned", deleted)
			}
		}
	}
}

// GetItems returns clipboard items with pagination.
func (s *ClipboardService) GetItems(limit, offset int) ([]ClipboardItem, error) {
	if limit <= 0 {
		limit = 50
	}
	items, err := s.store.List(limit, offset)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []ClipboardItem{}
	}
	return items, nil
}

// SearchItems searches clipboard history.
func (s *ClipboardService) SearchItems(query string) ([]ClipboardItem, error) {
	if query == "" {
		return s.GetItems(50, 0)
	}
	items, err := s.store.Search(query, 50)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []ClipboardItem{}
	}
	return items, nil
}

// FilterByCategory returns items of a specific category.
func (s *ClipboardService) FilterByCategory(category string) ([]ClipboardItem, error) {
	items, err := s.store.ListByCategory(category, 50)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []ClipboardItem{}
	}
	return items, nil
}

// GetGroups returns all snippet group names.
func (s *ClipboardService) GetGroups() ([]string, error) {
	groups, err := s.store.ListGroups()
	if err != nil {
		return nil, err
	}
	if groups == nil {
		groups = []string{}
	}
	return groups, nil
}

// GetGroupItems returns items in a specific group.
func (s *ClipboardService) GetGroupItems(groupName string) ([]ClipboardItem, error) {
	items, err := s.store.ListByGroup(groupName, 100)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []ClipboardItem{}
	}
	return items, nil
}

// SetItemGroup assigns an item to a snippet group.
func (s *ClipboardService) SetItemGroup(id int64, groupName string) error {
	return s.store.SetGroup(id, groupName)
}

// DeleteItem removes a clipboard item.
func (s *ClipboardService) DeleteItem(id int64) error {
	return s.store.Delete(id)
}

// TogglePin toggles the pinned state of an item.
func (s *ClipboardService) TogglePin(id int64) error {
	return s.store.TogglePin(id)
}

// ClearAll removes all non-pinned items.
func (s *ClipboardService) ClearAll() error {
	return s.store.Clear()
}

// CopyToClipboard writes content back to the system clipboard.
func (s *ClipboardService) CopyToClipboard(content string) {
	s.lastContent = content
	clipboard.Write(clipboard.FmtText, []byte(content))
}

// GetRetentionDays returns the current retention period.
func (s *ClipboardService) GetRetentionDays() int {
	return s.retentionDays
}

// SetRetentionDays updates the retention period and runs cleanup.
func (s *ClipboardService) SetRetentionDays(days int) (int64, error) {
	if days < 1 {
		days = 1
	}
	s.retentionDays = days
	return s.store.CleanExpired(days)
}

// GetStats returns basic statistics about clipboard history.
func (s *ClipboardService) GetStats() (map[string]interface{}, error) {
	var total int
	err := s.store.db.QueryRow("SELECT COUNT(*) FROM clipboard_items").Scan(&total)
	if err != nil {
		return nil, err
	}

	var pinned int
	err = s.store.db.QueryRow("SELECT COUNT(*) FROM clipboard_items WHERE pinned = 1").Scan(&pinned)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total":         total,
		"pinned":        pinned,
		"retentionDays": s.retentionDays,
	}, nil
}

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
	store  *Store
	cancel context.CancelFunc
	app    *application.App
}

// NewClipboardService creates a new ClipboardService.
func NewClipboardService(store *Store) *ClipboardService {
	return &ClipboardService{
		store: store,
	}
}

// ServiceStartup is called when the application starts.
func (s *ClipboardService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	err := clipboard.Init()
	if err != nil {
		return err
	}

	s.app = application.Get()

	// Start monitoring clipboard in background
	monitorCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	go s.watchClipboard(monitorCtx)

	return nil
}

// ServiceShutdown is called when the application is shutting down.
func (s *ClipboardService) ServiceShutdown() error {
	if s.cancel != nil {
		s.cancel()
	}
	return s.store.Close()
}

// watchClipboard polls the clipboard for changes.
func (s *ClipboardService) watchClipboard(ctx context.Context) {
	textCh := clipboard.Watch(ctx, clipboard.FmtText)

	for {
		select {
		case <-ctx.Done():
			return
		case data := <-textCh:
			if len(data) == 0 {
				continue
			}
			content := string(data)
			item, err := s.store.Save(content, "text")
			if err != nil {
				slog.Error("failed to save clipboard item", "error", err)
				continue
			}
			if item != nil {
				// Emit event to frontend
				s.app.Event.Emit("clipboard:new", item)
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
	clipboard.Write(clipboard.FmtText, []byte(content))
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

	var oldest string
	err = s.store.db.QueryRow("SELECT COALESCE(MIN(created_at), '') FROM clipboard_items").Scan(&oldest)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total":     total,
		"pinned":    pinned,
		"oldest":    oldest,
		"queryTime": time.Now().Format(time.RFC3339),
	}, nil
}

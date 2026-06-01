package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
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
	lastImageHash string
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
			// Check text clipboard
			textData := clipboard.Read(clipboard.FmtText)
			if len(textData) > 0 {
				content := string(textData)
				if content != s.lastContent {
					s.lastContent = content
					item, err := s.store.Save(content, "text")
					if err != nil {
						slog.Error("failed to save clipboard text", "error", err)
					} else if item != nil {
						s.app.Event.Emit("clipboard:new", item)
					}
					continue
				}
			}

			// Check image clipboard
			imgData := clipboard.Read(clipboard.FmtImage)
			if len(imgData) > 0 {
				hash := hashBytes(imgData)
				if hash != s.lastImageHash {
					s.lastImageHash = hash
					// Store as base64
					b64 := base64.StdEncoding.EncodeToString(imgData)
					content := "data:image/png;base64," + b64
					item, err := s.store.SaveImage(content, len(imgData))
					if err != nil {
						slog.Error("failed to save clipboard image", "error", err)
					} else if item != nil {
						s.app.Event.Emit("clipboard:new", item)
					}
				}
			}
		}
	}
}

func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:8]) // short hash is enough for dedup
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

// FilterByDate returns items from a specific date (YYYY-MM-DD format).
func (s *ClipboardService) FilterByDate(dateStr string) ([]ClipboardItem, error) {
	loc := time.Now().Location()
	start, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return nil, err
	}
	end := start.AddDate(0, 0, 1)
	items, err := s.store.ListByTimeRange(start.Format(time.RFC3339), end.Format(time.RFC3339), 100)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []ClipboardItem{}
	}
	return items, nil
}

// FilterByDateRange returns items between two dates (YYYY-MM-DD format).
func (s *ClipboardService) FilterByDateRange(startDate, endDate string) ([]ClipboardItem, error) {
	loc := time.Now().Location()
	start, err := time.ParseInLocation("2006-01-02", startDate, loc)
	if err != nil {
		return nil, err
	}
	end, err := time.ParseInLocation("2006-01-02", endDate, loc)
	if err != nil {
		return nil, err
	}
	end = end.AddDate(0, 0, 1) // include the end date fully
	items, err := s.store.ListByTimeRange(start.Format(time.RFC3339), end.Format(time.RFC3339), 100)
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

// CopyImageToClipboard writes image data back to the system clipboard.
func (s *ClipboardService) CopyImageToClipboard(id int64) error {
	item, err := s.store.GetByID(id)
	if err != nil {
		return err
	}
	if item == nil || item.Category != "image" {
		return fmt.Errorf("item is not an image")
	}
	// Decode base64 data URI
	data, err := decodeDataURI(item.Content)
	if err != nil {
		return err
	}
	s.lastImageHash = hashBytes(data)
	clipboard.Write(clipboard.FmtImage, data)
	return nil
}

// ServeHTTP serves image data for the frontend.
func (s *ClipboardService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	item, err := s.store.GetByID(id)
	if err != nil || item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if item.Category != "image" {
		http.Error(w, "not an image", http.StatusBadRequest)
		return
	}
	data, err := decodeDataURI(item.Content)
	if err != nil {
		http.Error(w, "decode error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Write(data)
}

func decodeDataURI(dataURI string) ([]byte, error) {
	// Format: data:image/png;base64,XXXX
	const prefix = "data:image/png;base64,"
	if len(dataURI) <= len(prefix) {
		return nil, fmt.Errorf("invalid data URI")
	}
	return base64.StdEncoding.DecodeString(dataURI[len(prefix):])
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

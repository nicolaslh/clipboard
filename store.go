package main

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ClipboardItem represents a single clipboard entry.
type ClipboardItem struct {
	ID        int64  `json:"id"`
	Content   string `json:"content"`
	Type      string `json:"type"` // "text" or "image"
	CreatedAt string `json:"createdAt"`
	Pinned    bool   `json:"pinned"`
}

// Store handles persistence of clipboard items using SQLite.
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store and initializes the database schema.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS clipboard_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'text',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			pinned INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_clipboard_items_created_at ON clipboard_items(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_clipboard_items_content ON clipboard_items(content);
	`)
	if err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

// Save inserts a new clipboard item, skipping duplicates of the most recent entry.
func (s *Store) Save(content string, contentType string) (*ClipboardItem, error) {
	// Check if the last item is the same content to avoid duplicates
	var lastContent string
	err := s.db.QueryRow("SELECT content FROM clipboard_items ORDER BY created_at DESC LIMIT 1").Scan(&lastContent)
	if err == nil && lastContent == content {
		return nil, nil // Skip duplicate
	}

	now := time.Now().Format(time.RFC3339)
	result, err := s.db.Exec(
		"INSERT INTO clipboard_items (content, type, created_at) VALUES (?, ?, ?)",
		content, contentType, now,
	)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &ClipboardItem{
		ID:        id,
		Content:   content,
		Type:      contentType,
		CreatedAt: now,
		Pinned:    false,
	}, nil
}

// List returns clipboard items with pagination.
func (s *Store) List(limit, offset int) ([]ClipboardItem, error) {
	rows, err := s.db.Query(
		"SELECT id, content, type, created_at, pinned FROM clipboard_items ORDER BY pinned DESC, created_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ClipboardItem
	for rows.Next() {
		var item ClipboardItem
		var pinned int
		err := rows.Scan(&item.ID, &item.Content, &item.Type, &item.CreatedAt, &pinned)
		if err != nil {
			return nil, err
		}
		item.Pinned = pinned == 1
		items = append(items, item)
	}
	return items, nil
}

// Search finds clipboard items matching the query.
// Supports time-based search with keywords like "今天", "昨天", "本周", "YYYY-MM-DD".
func (s *Store) Search(query string, limit int) ([]ClipboardItem, error) {
	var rows *sql.Rows
	var err error

	// Check if query is a time-based search
	timeFilter := parseTimeQuery(query)
	if timeFilter != "" {
		rows, err = s.db.Query(
			"SELECT id, content, type, created_at, pinned FROM clipboard_items WHERE created_at >= ? ORDER BY pinned DESC, created_at DESC LIMIT ?",
			timeFilter, limit,
		)
	} else {
		rows, err = s.db.Query(
			"SELECT id, content, type, created_at, pinned FROM clipboard_items WHERE content LIKE ? ORDER BY pinned DESC, created_at DESC LIMIT ?",
			"%"+query+"%", limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ClipboardItem
	for rows.Next() {
		var item ClipboardItem
		var pinned int
		err := rows.Scan(&item.ID, &item.Content, &item.Type, &item.CreatedAt, &pinned)
		if err != nil {
			return nil, err
		}
		item.Pinned = pinned == 1
		items = append(items, item)
	}
	return items, nil
}

// Delete removes a clipboard item by ID.
func (s *Store) Delete(id int64) error {
	_, err := s.db.Exec("DELETE FROM clipboard_items WHERE id = ?", id)
	return err
}

// TogglePin toggles the pinned state of a clipboard item.
func (s *Store) TogglePin(id int64) error {
	_, err := s.db.Exec("UPDATE clipboard_items SET pinned = CASE WHEN pinned = 0 THEN 1 ELSE 0 END WHERE id = ?", id)
	return err
}

// Clear removes all non-pinned clipboard items.
func (s *Store) Clear() error {
	_, err := s.db.Exec("DELETE FROM clipboard_items WHERE pinned = 0")
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// parseTimeQuery checks if the query is a time-based keyword and returns
// an RFC3339 timestamp string for filtering. Returns empty string if not a time query.
func parseTimeQuery(query string) string {
	now := time.Now()
	var t time.Time

	switch query {
	case "今天", "today":
		t = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "昨天", "yesterday":
		t = time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location())
	case "本周", "this week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		t = time.Date(now.Year(), now.Month(), now.Day()-(weekday-1), 0, 0, 0, 0, now.Location())
	case "本月", "this month":
		t = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "最近1小时", "last hour":
		t = now.Add(-1 * time.Hour)
	case "最近3天", "last 3 days":
		t = now.AddDate(0, 0, -3)
	case "最近7天", "last 7 days", "最近一周":
		t = now.AddDate(0, 0, -7)
	case "最近30天", "last 30 days", "最近一月":
		t = now.AddDate(0, 0, -30)
	default:
		// Try parsing as date format YYYY-MM-DD
		parsed, err := time.ParseInLocation("2006-01-02", query, now.Location())
		if err == nil {
			t = parsed
		} else {
			return ""
		}
	}

	return t.Format(time.RFC3339)
}

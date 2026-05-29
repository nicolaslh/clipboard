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
func (s *Store) Search(query string, limit int) ([]ClipboardItem, error) {
	rows, err := s.db.Query(
		"SELECT id, content, type, created_at, pinned FROM clipboard_items WHERE content LIKE ? ORDER BY pinned DESC, created_at DESC LIMIT ?",
		"%"+query+"%", limit,
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

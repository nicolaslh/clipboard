package main

import (
	"database/sql"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ClipboardItem represents a single clipboard entry.
type ClipboardItem struct {
	ID        int64  `json:"id"`
	Content   string `json:"content"`
	Type      string `json:"type"`     // "text", "url", "email", "code", "path"
	Category  string `json:"category"` // auto-detected category label
	CreatedAt string `json:"createdAt"`
	Pinned    bool   `json:"pinned"`
	GroupName string `json:"groupName"` // snippet group name (empty = ungrouped)
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
			category TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			pinned INTEGER NOT NULL DEFAULT 0,
			group_name TEXT NOT NULL DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS idx_clipboard_items_created_at ON clipboard_items(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_clipboard_items_content ON clipboard_items(content);
		CREATE INDEX IF NOT EXISTS idx_clipboard_items_category ON clipboard_items(category);
		CREATE INDEX IF NOT EXISTS idx_clipboard_items_group ON clipboard_items(group_name);
	`)
	if err != nil {
		return nil, err
	}

	// Migrate: add category column if missing
	db.Exec("ALTER TABLE clipboard_items ADD COLUMN category TEXT NOT NULL DEFAULT ''")
	// Migrate: add group_name column if missing
	db.Exec("ALTER TABLE clipboard_items ADD COLUMN group_name TEXT NOT NULL DEFAULT ''")

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

	// Auto-detect category
	category := detectCategory(content)
	if contentType == "text" {
		contentType = category
	}

	now := time.Now().Format(time.RFC3339)
	result, err := s.db.Exec(
		"INSERT INTO clipboard_items (content, type, category, created_at) VALUES (?, ?, ?, ?)",
		content, contentType, category, now,
	)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &ClipboardItem{
		ID:        id,
		Content:   content,
		Type:      contentType,
		Category:  category,
		CreatedAt: now,
		Pinned:    false,
	}, nil
}

// List returns clipboard items with pagination.
func (s *Store) List(limit, offset int) ([]ClipboardItem, error) {
	rows, err := s.db.Query(
		"SELECT id, content, type, COALESCE(category,''), created_at, pinned, COALESCE(group_name,'') FROM clipboard_items ORDER BY pinned DESC, created_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

// ListByCategory returns items filtered by category.
func (s *Store) ListByCategory(category string, limit int) ([]ClipboardItem, error) {
	rows, err := s.db.Query(
		"SELECT id, content, type, COALESCE(category,''), created_at, pinned, COALESCE(group_name,'') FROM clipboard_items WHERE category = ? ORDER BY pinned DESC, created_at DESC LIMIT ?",
		category, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

// ListByGroup returns items in a specific snippet group.
func (s *Store) ListByGroup(groupName string, limit int) ([]ClipboardItem, error) {
	rows, err := s.db.Query(
		"SELECT id, content, type, COALESCE(category,''), created_at, pinned, COALESCE(group_name,'') FROM clipboard_items WHERE group_name = ? ORDER BY created_at DESC LIMIT ?",
		groupName, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

// ListGroups returns all distinct group names.
func (s *Store) ListGroups() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT group_name FROM clipboard_items WHERE group_name != '' ORDER BY group_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		groups = append(groups, name)
	}
	return groups, nil
}

// SetGroup assigns a group name to an item.
func (s *Store) SetGroup(id int64, groupName string) error {
	_, err := s.db.Exec("UPDATE clipboard_items SET group_name = ? WHERE id = ?", groupName, id)
	return err
}

// Search finds clipboard items matching the query.
func (s *Store) Search(query string, limit int) ([]ClipboardItem, error) {
	var rows *sql.Rows
	var err error

	timeFilter := parseTimeQuery(query)
	if timeFilter != "" {
		rows, err = s.db.Query(
			"SELECT id, content, type, COALESCE(category,''), created_at, pinned, COALESCE(group_name,'') FROM clipboard_items WHERE created_at >= ? ORDER BY pinned DESC, created_at DESC LIMIT ?",
			timeFilter, limit,
		)
	} else {
		rows, err = s.db.Query(
			"SELECT id, content, type, COALESCE(category,''), created_at, pinned, COALESCE(group_name,'') FROM clipboard_items WHERE content LIKE ? ORDER BY pinned DESC, created_at DESC LIMIT ?",
			"%"+query+"%", limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
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

// CleanExpired removes non-pinned items older than the given duration.
func (s *Store) CleanExpired(retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	result, err := s.db.Exec("DELETE FROM clipboard_items WHERE pinned = 0 AND created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// scanItems scans rows into ClipboardItem slice.
func scanItems(rows *sql.Rows) ([]ClipboardItem, error) {
	var items []ClipboardItem
	for rows.Next() {
		var item ClipboardItem
		var pinned int
		err := rows.Scan(&item.ID, &item.Content, &item.Type, &item.Category, &item.CreatedAt, &pinned, &item.GroupName)
		if err != nil {
			return nil, err
		}
		item.Pinned = pinned == 1
		items = append(items, item)
	}
	return items, nil
}

// --- Content category detection ---

var (
	codePatterns = regexp.MustCompile(`(?m)(^(func |def |class |import |export |const |let |var |if |for |while |return |package |#include)|[{}\[\]];?\s*$|=>|->)`)
	pathPattern  = regexp.MustCompile(`^[/~][\w./-]+$|^[A-Z]:\\[\w.\\-]+$`)
)

func detectCategory(content string) string {
	trimmed := strings.TrimSpace(content)

	// URL
	if u, err := url.Parse(trimmed); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
		return "url"
	}

	// Email
	if _, err := mail.ParseAddress(trimmed); err == nil && strings.Contains(trimmed, "@") && !strings.Contains(trimmed, " ") {
		return "email"
	}

	// File path
	if pathPattern.MatchString(trimmed) && len(trimmed) < 500 {
		return "path"
	}

	// Code snippet
	if codePatterns.MatchString(content) {
		return "code"
	}

	return "text"
}

// --- Time query parsing ---

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
		parsed, err := time.ParseInLocation("2006-01-02", query, now.Location())
		if err == nil {
			t = parsed
		} else {
			return ""
		}
	}

	return t.Format(time.RFC3339)
}

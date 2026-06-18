package memory

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Conversation struct {
	ID        string
	Title     string
	CreatedAt string
	UpdatedAt string
	Starred   bool
}

type Message struct {
	ID             string
	ConversationID string
	Role           string
	Content        string
	Model          string
	CreatedAt      string
	ParentID       string
	VariantIndex   int
	ActiveVariant  bool
}

type ChatAttachment struct {
	ID             string
	ConversationID string
	UserMessageID  string
	FileName       string
	ContentType    string
	SizeBytes      int64
	Status         string
	Summary        string
	Preview        string
	SourceKind     string
	Sources        []string
	Retention      string
	Content        string
	CreatedAt      string
	ExpiresAt      string
}

type SearchResult struct {
	MessageID      string
	ConversationID string
	Role           string
	Snippet        string
	CreatedAt      string
}

type Memory struct {
	ID         string
	Kind       string
	Content    string
	Importance int
	Source     string
	Pinned     bool
	Disabled   bool
	CreatedAt  string
	UpdatedAt  string
}

type MemorySearchResult struct {
	ID         string
	Kind       string
	Content    string
	Snippet    string
	Importance int
	Source     string
	Pinned     bool
	CreatedAt  string
	UpdatedAt  string
}

type ToolRun struct {
	ID                 string
	ConversationID     string
	SessionID          string
	UserMessageID      string
	AssistantMessageID string
	ParentMessageID    string
	VariantIndex       int
	ToolName           string
	Input              any
	Output             any
	Status             string
	RiskLevel          string
	CreatedAt          string
	CompletedAt        string
}

type AgentTurn struct {
	ID                 string
	ConversationID     string
	SessionID          string
	UserMessageID      string
	AssistantMessageID string
	Status             string
	Trace              []string
	Sources            []string
	SourceKind         string
	Model              string
	CreatedAt          string
	UpdatedAt          string
	CompletedAt        string
}

type ConversationRouteState struct {
	ConversationID string
	StateJSON      string
	CreatedAt      string
	UpdatedAt      string
}

type ToolRunFilter struct {
	ConversationID string
	SessionID      string
	UserMessageID  string
	Limit          int
}

type ConversationStats struct {
	Total      int `json:"total"`
	WithTools  int `json:"withTools"`
	WithSkills int `json:"withSkills"`
}

type SkillUsed struct {
	ID             string
	ConversationID string
	SkillName      string
	SkillVersion   string
	CreatedAt      string
}

func Open(ctx context.Context, databasePath string) (*Store, error) {
	if databasePath == "" {
		return nil, fmt.Errorf("memory database path is empty")
	}

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	store := &Store{db: db}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("memory store is not open")
	}
	return s.db.PingContext(ctx)
}

func (s *Store) CreateConversation(ctx context.Context, title string) (Conversation, error) {
	now := timestamp()
	conversation := Conversation{
		ID:        newID("conv"),
		Title:     compactTitle(title),
		CreatedAt: now,
		UpdatedAt: now,
		Starred:   false,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO conversations (id, title, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, conversation.ID, conversation.Title, conversation.CreatedAt, conversation.UpdatedAt)
	if err != nil {
		return Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	return conversation, nil
}

func (s *Store) UpdateConversationTitle(ctx context.Context, conversationID string, title string) (Conversation, error) {
	conversationID = strings.TrimSpace(conversationID)
	title = compactTitle(title)
	if conversationID == "" {
		return Conversation{}, fmt.Errorf("conversation id is required")
	}
	now := timestamp()
	result, err := s.db.ExecContext(ctx, `
		UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?
	`, title, now, conversationID)
	if err != nil {
		return Conversation{}, fmt.Errorf("rename conversation: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Conversation{}, fmt.Errorf("conversation not found: %s", conversationID)
	}
	return s.GetConversation(ctx, conversationID)
}

func (s *Store) SetConversationStarred(ctx context.Context, conversationID string, starred bool) (Conversation, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return Conversation{}, fmt.Errorf("conversation id is required")
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE conversations SET starred = ? WHERE id = ?
	`, boolInt(starred), conversationID)
	if err != nil {
		return Conversation{}, fmt.Errorf("set conversation starred: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Conversation{}, fmt.Errorf("conversation not found: %s", conversationID)
	}
	return s.GetConversation(ctx, conversationID)
}

func (s *Store) DeleteConversation(ctx context.Context, conversationID string) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return fmt.Errorf("conversation id is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin conversation delete: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM message_fts WHERE conversation_id = ?`, conversationID); err != nil {
		return fmt.Errorf("delete conversation message index: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM tool_runs WHERE conversation_id = ?`, conversationID); err != nil {
		return fmt.Errorf("delete conversation tool runs: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM agent_turns WHERE conversation_id = ?`, conversationID); err != nil {
		return fmt.Errorf("delete conversation agent turns: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM conversation_route_state WHERE conversation_id = ?`, conversationID); err != nil {
		return fmt.Errorf("delete conversation route state: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chat_attachments WHERE conversation_id = ?`, conversationID); err != nil {
		return fmt.Errorf("delete conversation chat attachments: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM skills_used WHERE conversation_id = ?`, conversationID); err != nil {
		return fmt.Errorf("delete conversation skill usage: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM conversations WHERE id = ?`, conversationID)
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("conversation not found: %s", conversationID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit conversation delete: %w", err)
	}
	return nil
}

func (s *Store) SaveMessage(ctx context.Context, msg Message) (Message, error) {
	if msg.ID == "" {
		msg.ID = newID("msg")
	}
	if msg.CreatedAt == "" {
		msg.CreatedAt = timestamp()
	}
	msg.ParentID = strings.TrimSpace(msg.ParentID)
	if msg.Role == "user" && msg.ParentID != "" {
		rootID, err := s.userVariantRootID(ctx, msg.ConversationID, msg.ParentID)
		if err != nil {
			return Message{}, err
		}
		msg.ParentID = rootID
		if msg.VariantIndex <= 0 {
			if err := s.db.QueryRowContext(ctx, `
				SELECT COALESCE(MAX(variant_index), 0) + 1
				FROM messages
				WHERE conversation_id = ? AND role = 'user' AND (id = ? OR parent_id = ?)
			`, msg.ConversationID, msg.ParentID, msg.ParentID).Scan(&msg.VariantIndex); err != nil {
				return Message{}, fmt.Errorf("next prompt variant index: %w", err)
			}
		}
		msg.ActiveVariant = true
	} else if msg.Role == "assistant" && msg.ParentID != "" {
		if msg.VariantIndex <= 0 {
			if err := s.db.QueryRowContext(ctx, `
				SELECT COALESCE(MAX(variant_index), 0) + 1
				FROM messages
				WHERE conversation_id = ? AND role = 'assistant' AND parent_id = ?
			`, msg.ConversationID, msg.ParentID).Scan(&msg.VariantIndex); err != nil {
				return Message{}, fmt.Errorf("next response variant index: %w", err)
			}
		}
		msg.ActiveVariant = true
	} else if msg.VariantIndex <= 0 {
		msg.VariantIndex = 0
	}
	if (msg.Role != "assistant" && msg.Role != "user") || msg.ParentID == "" {
		msg.ActiveVariant = true
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Message{}, fmt.Errorf("begin message transaction: %w", err)
	}
	defer tx.Rollback()

	if msg.Role == "assistant" && msg.ParentID != "" && msg.ActiveVariant {
		if _, err := tx.ExecContext(ctx, `
			UPDATE messages SET active_variant = 0
			WHERE conversation_id = ? AND role = 'assistant' AND parent_id = ?
		`, msg.ConversationID, msg.ParentID); err != nil {
			return Message{}, fmt.Errorf("deactivate response variants: %w", err)
		}
	}
	if msg.Role == "user" && msg.ParentID != "" && msg.ActiveVariant {
		if _, err := tx.ExecContext(ctx, `
			UPDATE messages SET active_variant = 0
			WHERE conversation_id = ? AND role = 'user' AND (id = ? OR parent_id = ?)
		`, msg.ConversationID, msg.ParentID, msg.ParentID); err != nil {
			return Message{}, fmt.Errorf("deactivate prompt variants: %w", err)
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO messages (id, conversation_id, role, content, model, created_at, parent_id, variant_index, active_variant)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.ID, msg.ConversationID, msg.Role, msg.Content, msg.Model, msg.CreatedAt, msg.ParentID, msg.VariantIndex, boolInt(msg.ActiveVariant))
	if err != nil {
		return Message{}, fmt.Errorf("insert message: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO message_fts (content, message_id, conversation_id)
		VALUES (?, ?, ?)
	`, msg.Content, msg.ID, msg.ConversationID)
	if err != nil {
		return Message{}, fmt.Errorf("index message: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE conversations SET updated_at = ? WHERE id = ?
	`, msg.CreatedAt, msg.ConversationID)
	if err != nil {
		return Message{}, fmt.Errorf("update conversation timestamp: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Message{}, fmt.Errorf("commit message transaction: %w", err)
	}
	return msg, nil
}

func (s *Store) userVariantRootID(ctx context.Context, conversationID string, messageID string) (string, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return "", fmt.Errorf("prompt variant parent id is required")
	}
	var role string
	var parentID string
	var parentConversationID string
	if err := s.db.QueryRowContext(ctx, `
		SELECT conversation_id, role, parent_id
		FROM messages
		WHERE id = ?
	`, messageID).Scan(&parentConversationID, &role, &parentID); err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("prompt variant parent not found: %s", messageID)
		}
		return "", fmt.Errorf("find prompt variant parent: %w", err)
	}
	if parentConversationID != conversationID || role != "user" {
		return "", fmt.Errorf("prompt variant parent does not belong to this conversation")
	}
	parentID = strings.TrimSpace(parentID)
	if parentID != "" {
		return parentID, nil
	}
	return messageID, nil
}

func (s *Store) GetMessage(ctx context.Context, messageID string) (Message, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return Message{}, fmt.Errorf("message id is required")
	}
	var msg Message
	var active int
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, conversation_id, role, content, model, created_at, parent_id, variant_index, active_variant
		FROM messages
		WHERE id = ?
	`, messageID).Scan(
		&msg.ID,
		&msg.ConversationID,
		&msg.Role,
		&msg.Content,
		&msg.Model,
		&msg.CreatedAt,
		&msg.ParentID,
		&msg.VariantIndex,
		&active,
	); err != nil {
		if err == sql.ErrNoRows {
			return Message{}, fmt.Errorf("message not found: %s", messageID)
		}
		return Message{}, fmt.Errorf("get message: %w", err)
	}
	msg.ActiveVariant = active != 0
	return msg, nil
}

func (s *Store) CreateUserMessageVariant(ctx context.Context, messageID string, content string) (Message, error) {
	messageID = strings.TrimSpace(messageID)
	content = strings.TrimSpace(content)
	if messageID == "" {
		return Message{}, fmt.Errorf("message id is required")
	}
	if content == "" {
		return Message{}, fmt.Errorf("message content is required")
	}

	original, err := s.GetMessage(ctx, messageID)
	if err != nil {
		return Message{}, err
	}
	if original.Role != "user" {
		return Message{}, fmt.Errorf("editable user message not found: %s", messageID)
	}
	rootID := original.ID
	if strings.TrimSpace(original.ParentID) != "" {
		rootID = original.ParentID
	}
	return s.SaveMessage(ctx, Message{
		ConversationID: original.ConversationID,
		Role:           "user",
		Content:        content,
		Model:          original.Model,
		ParentID:       rootID,
		ActiveVariant:  true,
	})
}

func (s *Store) SearchMessages(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			m.id,
			m.conversation_id,
			m.role,
			snippet(message_fts, 0, '[', ']', ' ... ', 12),
			m.created_at
		FROM message_fts
		JOIN messages m ON message_fts.message_id = m.id
		WHERE message_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(
			&result.MessageID,
			&result.ConversationID,
			&result.Role,
			&result.Snippet,
			&result.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read search results: %w", err)
	}
	return results, nil
}

func (s *Store) CountConversationMessages(ctx context.Context, conversationID string) (int, error) {
	if strings.TrimSpace(conversationID) == "" {
		return 0, fmt.Errorf("conversation id is required")
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM messages WHERE conversation_id = ?
	`, conversationID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count conversation messages: %w", err)
	}
	return count, nil
}

func (s *Store) ListConversationMessages(ctx context.Context, conversationID string, limit int) ([]Message, error) {
	if strings.TrimSpace(conversationID) == "" {
		return nil, fmt.Errorf("conversation id is required")
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, conversation_id, role, content, model, created_at, parent_id, variant_index, active_variant
		FROM messages
		WHERE conversation_id = ?
		ORDER BY rowid DESC
		LIMIT ?
	`, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("list conversation messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var active int
		if err := rows.Scan(
			&msg.ID,
			&msg.ConversationID,
			&msg.Role,
			&msg.Content,
			&msg.Model,
			&msg.CreatedAt,
			&msg.ParentID,
			&msg.VariantIndex,
			&active,
		); err != nil {
			return nil, fmt.Errorf("scan conversation message: %w", err)
		}
		msg.ActiveVariant = active != 0
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read conversation messages: %w", err)
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

func (s *Store) SaveChatAttachment(ctx context.Context, attachment ChatAttachment) (ChatAttachment, error) {
	if err := s.pruneExpiredChatAttachments(ctx); err != nil {
		return ChatAttachment{}, err
	}
	if attachment.ID == "" {
		attachment.ID = newID("att")
	}
	if attachment.CreatedAt == "" {
		attachment.CreatedAt = timestamp()
	}
	attachment.FileName = strings.TrimSpace(filepath.Base(filepath.ToSlash(attachment.FileName)))
	if attachment.FileName == "" || attachment.FileName == "." || attachment.FileName == "/" {
		return ChatAttachment{}, fmt.Errorf("attachment file name is required")
	}
	attachment.Status = strings.TrimSpace(attachment.Status)
	if attachment.Status == "" {
		attachment.Status = "ready"
	}
	attachment.SourceKind = strings.TrimSpace(attachment.SourceKind)
	if attachment.SourceKind == "" {
		attachment.SourceKind = "attachment"
	}
	attachment.Retention = strings.TrimSpace(attachment.Retention)
	if attachment.Retention == "" {
		attachment.Retention = "conversation"
	}
	if attachment.ConversationID == "" && attachment.UserMessageID == "" && attachment.ExpiresAt == "" {
		attachment.ExpiresAt = time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	}
	attachment.FileName = sanitizeTextForStorage(attachment.FileName)
	attachment.Summary = sanitizeTextForStorage(attachment.Summary)
	attachment.Preview = sanitizeTextForStorage(attachment.Preview)
	attachment.Content = sanitizeTextForStorage(attachment.Content)
	attachment.Sources = sanitizeStringSliceForStorage(attachment.Sources)
	sourcesJSON, err := json.Marshal(sanitizeValueForStorage(attachment.Sources))
	if err != nil {
		return ChatAttachment{}, fmt.Errorf("encode attachment sources: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO chat_attachments (
			id, conversation_id, user_message_id, file_name, content_type, size_bytes, status,
			summary, preview, source_kind, sources_json, retention, content, created_at, expires_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, attachment.ID, attachment.ConversationID, attachment.UserMessageID, attachment.FileName, attachment.ContentType,
		attachment.SizeBytes, attachment.Status, attachment.Summary, attachment.Preview, attachment.SourceKind,
		string(sourcesJSON), attachment.Retention, attachment.Content, attachment.CreatedAt, attachment.ExpiresAt)
	if err != nil {
		return ChatAttachment{}, fmt.Errorf("save chat attachment: %w", err)
	}
	return attachment, nil
}

func (s *Store) GetChatAttachment(ctx context.Context, id string) (ChatAttachment, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ChatAttachment{}, fmt.Errorf("attachment id is required")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, conversation_id, user_message_id, file_name, content_type, size_bytes, status,
			summary, preview, source_kind, sources_json, retention, content, created_at, expires_at
		FROM chat_attachments
		WHERE id = ?
	`, id)
	if err != nil {
		return ChatAttachment{}, fmt.Errorf("get chat attachment: %w", err)
	}
	defer rows.Close()
	items, err := scanChatAttachments(rows)
	if err != nil {
		return ChatAttachment{}, err
	}
	if len(items) == 0 {
		return ChatAttachment{}, fmt.Errorf("attachment not found: %s", id)
	}
	return items[0], nil
}

func (s *Store) LinkChatAttachments(ctx context.Context, conversationID string, userMessageID string, ids []string) ([]ChatAttachment, error) {
	conversationID = strings.TrimSpace(conversationID)
	userMessageID = strings.TrimSpace(userMessageID)
	if conversationID == "" {
		return nil, fmt.Errorf("conversation id is required")
	}
	if userMessageID == "" {
		return nil, fmt.Errorf("user message id is required")
	}
	cleanIDs := uniqueNonEmpty(ids)
	if len(cleanIDs) == 0 {
		return nil, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin attachment link: %w", err)
	}
	defer tx.Rollback()
	for _, id := range cleanIDs {
		var existingConversation string
		if err := tx.QueryRowContext(ctx, `SELECT conversation_id FROM chat_attachments WHERE id = ?`, id).Scan(&existingConversation); err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("attachment not found: %s", id)
			}
			return nil, fmt.Errorf("read attachment %s: %w", id, err)
		}
		if strings.TrimSpace(existingConversation) != "" && existingConversation != conversationID {
			return nil, fmt.Errorf("attachment %s belongs to another conversation", id)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE chat_attachments
			SET conversation_id = ?, user_message_id = ?, expires_at = ''
			WHERE id = ?
		`, conversationID, userMessageID, id); err != nil {
			return nil, fmt.Errorf("link attachment %s: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit attachment link: %w", err)
	}
	return s.ChatAttachmentsByIDs(ctx, conversationID, cleanIDs)
}

func (s *Store) ChatAttachmentsByIDs(ctx context.Context, conversationID string, ids []string) ([]ChatAttachment, error) {
	conversationID = strings.TrimSpace(conversationID)
	cleanIDs := uniqueNonEmpty(ids)
	if len(cleanIDs) == 0 {
		return nil, nil
	}
	items := make([]ChatAttachment, 0, len(cleanIDs))
	for _, id := range cleanIDs {
		item, err := s.GetChatAttachment(ctx, id)
		if err != nil {
			return nil, err
		}
		if item.ConversationID != "" && conversationID != "" && item.ConversationID != conversationID {
			return nil, fmt.Errorf("attachment %s belongs to another conversation", id)
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) ListChatAttachmentsForMessages(ctx context.Context, messageIDs []string) (map[string][]ChatAttachment, error) {
	cleanIDs := uniqueNonEmpty(messageIDs)
	out := make(map[string][]ChatAttachment, len(cleanIDs))
	for _, id := range cleanIDs {
		rows, err := s.db.QueryContext(ctx, `
			SELECT id, conversation_id, user_message_id, file_name, content_type, size_bytes, status,
				summary, preview, source_kind, sources_json, retention, content, created_at, expires_at
			FROM chat_attachments
			WHERE user_message_id = ?
			ORDER BY rowid ASC
		`, id)
		if err != nil {
			return nil, fmt.Errorf("list message attachments: %w", err)
		}
		items, scanErr := scanChatAttachments(rows)
		closeErr := rows.Close()
		if scanErr != nil {
			return nil, scanErr
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close message attachments: %w", closeErr)
		}
		if len(items) > 0 {
			out[id] = items
		}
	}
	return out, nil
}

func (s *Store) pruneExpiredChatAttachments(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM chat_attachments
		WHERE expires_at IS NOT NULL AND expires_at != '' AND expires_at <= ?
	`, timestamp()); err != nil {
		return fmt.Errorf("prune expired chat attachments: %w", err)
	}
	return nil
}

func (s *Store) GetConversation(ctx context.Context, conversationID string) (Conversation, error) {
	if strings.TrimSpace(conversationID) == "" {
		return Conversation{}, fmt.Errorf("conversation id is required")
	}
	var conversation Conversation
	var starred int
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, title, created_at, updated_at, starred
		FROM conversations
		WHERE id = ?
	`, conversationID).Scan(&conversation.ID, &conversation.Title, &conversation.CreatedAt, &conversation.UpdatedAt, &starred); err != nil {
		if err == sql.ErrNoRows {
			return Conversation{}, fmt.Errorf("conversation not found: %s", conversationID)
		}
		return Conversation{}, fmt.Errorf("get conversation: %w", err)
	}
	conversation.Starred = starred != 0
	return conversation, nil
}

func (s *Store) ListConversations(ctx context.Context, limit int) ([]Conversation, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, created_at, updated_at, starred
		FROM conversations
		ORDER BY starred DESC, updated_at DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()
	conversations := []Conversation{}
	for rows.Next() {
		var conversation Conversation
		var starred int
		if err := rows.Scan(&conversation.ID, &conversation.Title, &conversation.CreatedAt, &conversation.UpdatedAt, &starred); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		conversation.Starred = starred != 0
		conversations = append(conversations, conversation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read conversations: %w", err)
	}
	return conversations, nil
}

func (s *Store) SaveMemory(ctx context.Context, item Memory) (Memory, error) {
	item.Kind = strings.TrimSpace(item.Kind)
	item.Content = sanitizeTextForStorage(strings.TrimSpace(item.Content))
	if item.Kind == "" {
		return Memory{}, fmt.Errorf("memory kind is required")
	}
	if item.Content == "" {
		return Memory{}, fmt.Errorf("memory content is required")
	}
	if item.ID == "" {
		item.ID = newID("mem")
	}
	if item.Importance <= 0 {
		item.Importance = 1
	}
	if item.Importance > 5 {
		item.Importance = 5
	}
	if item.CreatedAt == "" {
		item.CreatedAt = timestamp()
	}
	if item.UpdatedAt == "" {
		item.UpdatedAt = item.CreatedAt
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Memory{}, fmt.Errorf("begin memory transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO memories (
			id, kind, content, importance, source, pinned, disabled, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.Kind, item.Content, item.Importance, item.Source, boolInt(item.Pinned), boolInt(item.Disabled), item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return Memory{}, fmt.Errorf("insert memory: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO memory_fts (content, memory_id)
		VALUES (?, ?)
	`, item.Content, item.ID)
	if err != nil {
		return Memory{}, fmt.Errorf("index memory: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Memory{}, fmt.Errorf("commit memory transaction: %w", err)
	}
	return item, nil
}

func (s *Store) FindMemoryByKindSource(ctx context.Context, kind string, source string) (Memory, bool, error) {
	kind = strings.TrimSpace(kind)
	source = strings.TrimSpace(source)
	if kind == "" || source == "" {
		return Memory{}, false, fmt.Errorf("kind and source are required")
	}
	var item Memory
	var pinned int
	var disabled int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, kind, content, importance, source, pinned, disabled, created_at, updated_at
		FROM memories
		WHERE kind = ? AND source = ? AND disabled = 0
		ORDER BY updated_at DESC
		LIMIT 1
	`, kind, source).Scan(
		&item.ID,
		&item.Kind,
		&item.Content,
		&item.Importance,
		&item.Source,
		&pinned,
		&disabled,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Memory{}, false, nil
	}
	if err != nil {
		return Memory{}, false, fmt.Errorf("find memory: %w", err)
	}
	item.Pinned = pinned != 0
	item.Disabled = disabled != 0
	return item, true, nil
}

func (s *Store) GetMemory(ctx context.Context, id string) (Memory, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Memory{}, fmt.Errorf("memory id is required")
	}
	var item Memory
	var pinned int
	var disabled int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, kind, content, importance, source, pinned, disabled, created_at, updated_at
		FROM memories
		WHERE id = ?
		LIMIT 1
	`, id).Scan(
		&item.ID,
		&item.Kind,
		&item.Content,
		&item.Importance,
		&item.Source,
		&pinned,
		&disabled,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Memory{}, fmt.Errorf("memory not found: %s", id)
	}
	if err != nil {
		return Memory{}, fmt.Errorf("get memory: %w", err)
	}
	item.Pinned = pinned != 0
	item.Disabled = disabled != 0
	return item, nil
}

func (s *Store) UpdateMemory(ctx context.Context, item Memory) (Memory, error) {
	item.ID = strings.TrimSpace(item.ID)
	item.Kind = strings.TrimSpace(item.Kind)
	item.Content = sanitizeTextForStorage(strings.TrimSpace(item.Content))
	if item.ID == "" {
		return Memory{}, fmt.Errorf("memory id is required")
	}
	if item.Kind == "" {
		return Memory{}, fmt.Errorf("memory kind is required")
	}
	if item.Content == "" {
		return Memory{}, fmt.Errorf("memory content is required")
	}
	if item.Importance <= 0 {
		item.Importance = 1
	}
	if item.Importance > 5 {
		item.Importance = 5
	}
	existing, err := s.GetMemory(ctx, item.ID)
	if err != nil {
		return Memory{}, err
	}
	if item.CreatedAt == "" {
		item.CreatedAt = existing.CreatedAt
	}
	item.UpdatedAt = timestamp()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Memory{}, fmt.Errorf("begin memory update: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE memories
		SET kind = ?, content = ?, importance = ?, source = ?, pinned = ?, disabled = ?, updated_at = ?
		WHERE id = ?
	`, item.Kind, item.Content, item.Importance, item.Source, boolInt(item.Pinned), boolInt(item.Disabled), item.UpdatedAt, item.ID)
	if err != nil {
		return Memory{}, fmt.Errorf("update memory: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Memory{}, fmt.Errorf("memory not found: %s", item.ID)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_fts WHERE memory_id = ?`, item.ID); err != nil {
		return Memory{}, fmt.Errorf("clear memory index: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO memory_fts (content, memory_id)
		VALUES (?, ?)
	`, item.Content, item.ID); err != nil {
		return Memory{}, fmt.Errorf("index memory: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Memory{}, fmt.Errorf("commit memory update: %w", err)
	}
	return item, nil
}

func (s *Store) SaveConversationSummary(ctx context.Context, conversationID string, content string) (Memory, error) {
	conversationID = strings.TrimSpace(conversationID)
	content = sanitizeTextForStorage(strings.TrimSpace(content))
	if conversationID == "" {
		return Memory{}, fmt.Errorf("conversation id is required")
	}
	if content == "" {
		return Memory{}, fmt.Errorf("summary content is required")
	}
	source := "conversation:" + conversationID
	now := timestamp()

	var existing Memory
	var pinned int
	var disabled int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, kind, content, importance, source, pinned, disabled, created_at, updated_at
		FROM memories
		WHERE kind = 'summary' AND source = ?
		LIMIT 1
	`, source).Scan(
		&existing.ID,
		&existing.Kind,
		&existing.Content,
		&existing.Importance,
		&existing.Source,
		&pinned,
		&disabled,
		&existing.CreatedAt,
		&existing.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return s.SaveMemory(ctx, Memory{
			Kind:       "summary",
			Content:    content,
			Importance: 3,
			Source:     source,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}
	if err != nil {
		return Memory{}, fmt.Errorf("find conversation summary: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Memory{}, fmt.Errorf("begin summary transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		UPDATE memories
		SET content = ?, importance = ?, disabled = 0, updated_at = ?
		WHERE id = ?
	`, content, 3, now, existing.ID)
	if err != nil {
		return Memory{}, fmt.Errorf("update conversation summary: %w", err)
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM memory_fts WHERE memory_id = ?`, existing.ID)
	if err != nil {
		return Memory{}, fmt.Errorf("clear summary index: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO memory_fts (content, memory_id)
		VALUES (?, ?)
	`, content, existing.ID)
	if err != nil {
		return Memory{}, fmt.Errorf("index summary: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Memory{}, fmt.Errorf("commit summary transaction: %w", err)
	}

	return Memory{
		ID:         existing.ID,
		Kind:       "summary",
		Content:    content,
		Importance: 3,
		Source:     source,
		Pinned:     pinned != 0,
		Disabled:   false,
		CreatedAt:  existing.CreatedAt,
		UpdatedAt:  now,
	}, nil
}

func (s *Store) SearchMemories(ctx context.Context, query string, limit int) ([]MemorySearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			m.id,
			m.kind,
			m.content,
			snippet(memory_fts, 0, '[', ']', ' ... ', 12),
			m.importance,
			m.source,
			m.pinned,
			m.created_at,
			m.updated_at
		FROM memory_fts
		JOIN memories m ON memory_fts.memory_id = m.id
		WHERE memory_fts MATCH ? AND m.disabled = 0
		ORDER BY m.pinned DESC, m.importance DESC, rank, m.updated_at DESC
		LIMIT ?
	`, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer rows.Close()
	return scanMemorySearchResults(rows)
}

func (s *Store) ListMemories(ctx context.Context, limit int) ([]MemorySearchResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			kind,
			content,
			content,
			importance,
			source,
			pinned,
			created_at,
			updated_at
		FROM memories
		WHERE disabled = 0
		ORDER BY pinned DESC, importance DESC, updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()
	return scanMemorySearchResults(rows)
}

func (s *Store) PinMemory(ctx context.Context, id string, pinned bool) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("memory id is required")
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE memories
		SET pinned = ?, updated_at = ?
		WHERE id = ? AND disabled = 0
	`, boolInt(pinned), timestamp(), id)
	if err != nil {
		return fmt.Errorf("pin memory: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return fmt.Errorf("memory not found: %s", id)
	}
	return nil
}

func (s *Store) DisableMemory(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("memory id is required")
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE memories
		SET disabled = 1, updated_at = ?
		WHERE id = ? AND disabled = 0
	`, timestamp(), id)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return fmt.Errorf("memory not found: %s", id)
	}
	return nil
}

func (s *Store) PruneDuplicateMemories(ctx context.Context) (int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, content
		FROM memories
		WHERE disabled = 0
		ORDER BY pinned DESC, importance DESC, updated_at DESC, id DESC
	`)
	if err != nil {
		return 0, fmt.Errorf("list memories for pruning: %w", err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	var duplicates []string
	for rows.Next() {
		var id string
		var kind string
		var content string
		if err := rows.Scan(&id, &kind, &content); err != nil {
			return 0, fmt.Errorf("scan memory for pruning: %w", err)
		}
		key := kind + "\x00" + normalizeMemoryContent(content)
		if seen[key] {
			duplicates = append(duplicates, id)
			continue
		}
		seen[key] = true
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read memories for pruning: %w", err)
	}
	if len(duplicates) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin prune transaction: %w", err)
	}
	defer tx.Rollback()
	for _, id := range duplicates {
		if _, err := tx.ExecContext(ctx, `
			UPDATE memories
			SET disabled = 1, updated_at = ?
			WHERE id = ?
		`, timestamp(), id); err != nil {
			return 0, fmt.Errorf("disable duplicate memory: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit prune transaction: %w", err)
	}
	return len(duplicates), nil
}

func (s *Store) SaveToolRun(ctx context.Context, run ToolRun) (ToolRun, error) {
	if run.ID == "" {
		run.ID = newID("tool")
	}
	if run.CreatedAt == "" {
		run.CreatedAt = timestamp()
	}
	if run.CompletedAt == "" {
		run.CompletedAt = timestamp()
	}
	run.Input = sanitizeValueForStorage(run.Input)
	run.Output = sanitizeValueForStorage(run.Output)
	inputJSON, err := json.Marshal(run.Input)
	if err != nil {
		return ToolRun{}, fmt.Errorf("encode tool input: %w", err)
	}
	outputJSON, err := json.Marshal(run.Output)
	if err != nil {
		return ToolRun{}, fmt.Errorf("encode tool output: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO tool_runs (
			id, conversation_id, session_id, user_message_id, assistant_message_id,
			parent_message_id, variant_index, tool_name, input_json, output_json,
			status, risk_level, created_at, completed_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, run.ID, run.ConversationID, run.SessionID, run.UserMessageID, run.AssistantMessageID, run.ParentMessageID, run.VariantIndex, run.ToolName, string(inputJSON), string(outputJSON), run.Status, run.RiskLevel, run.CreatedAt, run.CompletedAt)
	if err != nil {
		return ToolRun{}, fmt.Errorf("save tool run: %w", err)
	}
	return run, nil
}

func (s *Store) SaveAgentTurn(ctx context.Context, turn AgentTurn) (AgentTurn, error) {
	if turn.ID == "" {
		turn.ID = newID("turn")
	}
	now := timestamp()
	if turn.CreatedAt == "" {
		turn.CreatedAt = now
	}
	if turn.UpdatedAt == "" {
		turn.UpdatedAt = now
	}
	if strings.TrimSpace(turn.Status) == "" {
		turn.Status = "running"
	}
	traceJSON, err := json.Marshal(sanitizeValueForStorage(turn.Trace))
	if err != nil {
		return AgentTurn{}, fmt.Errorf("encode agent turn trace: %w", err)
	}
	sourcesJSON, err := json.Marshal(sanitizeValueForStorage(turn.Sources))
	if err != nil {
		return AgentTurn{}, fmt.Errorf("encode agent turn sources: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agent_turns (
			id, conversation_id, session_id, user_message_id, assistant_message_id,
			status, trace_json, sources_json, source_kind, model,
			created_at, updated_at, completed_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			conversation_id = excluded.conversation_id,
			session_id = excluded.session_id,
			user_message_id = excluded.user_message_id,
			assistant_message_id = excluded.assistant_message_id,
			status = excluded.status,
			trace_json = excluded.trace_json,
			sources_json = excluded.sources_json,
			source_kind = excluded.source_kind,
			model = excluded.model,
			updated_at = excluded.updated_at,
			completed_at = excluded.completed_at
	`, turn.ID, turn.ConversationID, turn.SessionID, turn.UserMessageID, turn.AssistantMessageID, turn.Status, string(traceJSON), string(sourcesJSON), turn.SourceKind, turn.Model, turn.CreatedAt, turn.UpdatedAt, turn.CompletedAt)
	if err != nil {
		return AgentTurn{}, fmt.Errorf("save agent turn: %w", err)
	}
	return turn, nil
}

func (s *Store) SaveConversationRouteState(ctx context.Context, state ConversationRouteState) (ConversationRouteState, error) {
	state.ConversationID = strings.TrimSpace(state.ConversationID)
	if state.ConversationID == "" {
		return ConversationRouteState{}, fmt.Errorf("conversation id is required")
	}
	if strings.TrimSpace(state.StateJSON) == "" {
		state.StateJSON = "{}"
	}
	now := timestamp()
	if state.CreatedAt == "" {
		state.CreatedAt = now
	}
	state.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO conversation_route_state (
			conversation_id, state_json, created_at, updated_at
		)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(conversation_id) DO UPDATE SET
			state_json = excluded.state_json,
			updated_at = excluded.updated_at
	`, state.ConversationID, state.StateJSON, state.CreatedAt, state.UpdatedAt)
	if err != nil {
		return ConversationRouteState{}, fmt.Errorf("save conversation route state: %w", err)
	}
	return state, nil
}

func (s *Store) GetConversationRouteState(ctx context.Context, conversationID string) (ConversationRouteState, bool, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return ConversationRouteState{}, false, fmt.Errorf("conversation id is required")
	}
	var state ConversationRouteState
	err := s.db.QueryRowContext(ctx, `
		SELECT conversation_id, state_json, created_at, updated_at
		FROM conversation_route_state
		WHERE conversation_id = ?
	`, conversationID).Scan(&state.ConversationID, &state.StateJSON, &state.CreatedAt, &state.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return ConversationRouteState{}, false, nil
		}
		return ConversationRouteState{}, false, fmt.Errorf("get conversation route state: %w", err)
	}
	return state, true, nil
}

func (s *Store) DeleteConversationRouteState(ctx context.Context, conversationID string) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return fmt.Errorf("conversation id is required")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM conversation_route_state WHERE conversation_id = ?`, conversationID); err != nil {
		return fmt.Errorf("delete conversation route state: %w", err)
	}
	return nil
}

func (s *Store) GetAgentTurn(ctx context.Context, turnID string) (AgentTurn, error) {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return AgentTurn{}, fmt.Errorf("agent turn id is required")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id, conversation_id, session_id, user_message_id, assistant_message_id,
			status, trace_json, sources_json, source_kind, model,
			created_at, updated_at, completed_at
		FROM agent_turns
		WHERE id = ?
	`, turnID)
	if err != nil {
		return AgentTurn{}, fmt.Errorf("get agent turn: %w", err)
	}
	defer rows.Close()
	turns, err := scanAgentTurns(rows)
	if err != nil {
		return AgentTurn{}, err
	}
	if len(turns) == 0 {
		return AgentTurn{}, fmt.Errorf("agent turn not found: %s", turnID)
	}
	return turns[0], nil
}

func (s *Store) ListAgentTurnsForConversation(ctx context.Context, conversationID string, limit int) ([]AgentTurn, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, fmt.Errorf("conversation id is required")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 300 {
		limit = 300
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id, conversation_id, session_id, user_message_id, assistant_message_id,
			status, trace_json, sources_json, source_kind, model,
			created_at, updated_at, completed_at
		FROM agent_turns
		WHERE conversation_id = ?
		ORDER BY created_at ASC, id ASC
		LIMIT ?
	`, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("list conversation agent turns: %w", err)
	}
	defer rows.Close()
	return scanAgentTurns(rows)
}

func (s *Store) ListToolRuns(ctx context.Context, limit int) ([]ToolRun, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id, conversation_id, session_id, user_message_id, assistant_message_id,
			parent_message_id, variant_index, tool_name, input_json, output_json,
			status, risk_level, created_at, completed_at
		FROM tool_runs
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list tool runs: %w", err)
	}
	defer rows.Close()

	var runs []ToolRun
	for rows.Next() {
		var run ToolRun
		var inputJSON string
		var outputJSON string
		if err := rows.Scan(
			&run.ID,
			&run.ConversationID,
			&run.SessionID,
			&run.UserMessageID,
			&run.AssistantMessageID,
			&run.ParentMessageID,
			&run.VariantIndex,
			&run.ToolName,
			&inputJSON,
			&outputJSON,
			&run.Status,
			&run.RiskLevel,
			&run.CreatedAt,
			&run.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan tool run: %w", err)
		}
		run.Input = decodeJSONValue(inputJSON)
		run.Output = decodeJSONValue(outputJSON)
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tool runs: %w", err)
	}
	return runs, nil
}

func (s *Store) ListToolRunsFiltered(ctx context.Context, filter ToolRunFilter) ([]ToolRun, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var clauses []string
	var args []any
	if value := strings.TrimSpace(filter.ConversationID); value != "" {
		clauses = append(clauses, "conversation_id = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.SessionID); value != "" {
		clauses = append(clauses, "session_id = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.UserMessageID); value != "" {
		clauses = append(clauses, "user_message_id = ?")
		args = append(args, value)
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id, conversation_id, session_id, user_message_id, assistant_message_id,
			parent_message_id, variant_index, tool_name, input_json, output_json,
			status, risk_level, created_at, completed_at
		FROM tool_runs
		`+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("list filtered tool runs: %w", err)
	}
	defer rows.Close()
	return scanToolRuns(rows)
}

func (s *Store) ListToolRunsForConversation(ctx context.Context, conversationID string, limit int) ([]ToolRun, error) {
	if strings.TrimSpace(conversationID) == "" {
		return nil, fmt.Errorf("conversation id is required")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id, conversation_id, session_id, user_message_id, assistant_message_id,
			parent_message_id, variant_index, tool_name, input_json, output_json,
			status, risk_level, created_at, completed_at
		FROM tool_runs
		WHERE conversation_id = ?
		ORDER BY created_at ASC, id ASC
		LIMIT ?
	`, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("list conversation tool runs: %w", err)
	}
	defer rows.Close()

	runs, err := scanToolRuns(rows)
	if err != nil {
		return nil, err
	}
	return runs, nil
}

func (s *Store) SaveSkillUsed(ctx context.Context, skill SkillUsed) (SkillUsed, error) {
	if skill.ID == "" {
		skill.ID = newID("skill")
	}
	if skill.CreatedAt == "" {
		skill.CreatedAt = timestamp()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO skills_used (
			id, conversation_id, skill_name, skill_version, created_at
		)
		VALUES (?, ?, ?, ?, ?)
	`, skill.ID, skill.ConversationID, skill.SkillName, skill.SkillVersion, skill.CreatedAt)
	if err != nil {
		return SkillUsed{}, fmt.Errorf("save skill usage: %w", err)
	}
	return skill, nil
}

func (s *Store) migrate(ctx context.Context) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			title TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			starred INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			model TEXT,
			created_at TEXT NOT NULL,
			parent_id TEXT NOT NULL DEFAULT '',
			variant_index INTEGER NOT NULL DEFAULT 0,
			active_variant INTEGER NOT NULL DEFAULT 1,
			FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
		);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS message_fts USING fts5(
			content,
			message_id UNINDEXED,
			conversation_id UNINDEXED
		);`,
		`CREATE TABLE IF NOT EXISTS tool_runs (
			id TEXT PRIMARY KEY,
			conversation_id TEXT,
			session_id TEXT NOT NULL DEFAULT '',
			user_message_id TEXT NOT NULL DEFAULT '',
			assistant_message_id TEXT NOT NULL DEFAULT '',
			parent_message_id TEXT NOT NULL DEFAULT '',
			variant_index INTEGER NOT NULL DEFAULT 0,
			tool_name TEXT NOT NULL,
			input_json TEXT NOT NULL,
			output_json TEXT NOT NULL,
			status TEXT NOT NULL,
			risk_level TEXT,
			created_at TEXT NOT NULL,
			completed_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS agent_turns (
			id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL,
			session_id TEXT NOT NULL DEFAULT '',
			user_message_id TEXT NOT NULL,
			assistant_message_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			trace_json TEXT NOT NULL DEFAULT '[]',
			sources_json TEXT NOT NULL DEFAULT '[]',
			source_kind TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			completed_at TEXT NOT NULL DEFAULT '',
			FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS conversation_route_state (
			conversation_id TEXT PRIMARY KEY,
			state_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS chat_attachments (
			id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL DEFAULT '',
			user_message_id TEXT NOT NULL DEFAULT '',
			file_name TEXT NOT NULL,
			content_type TEXT NOT NULL DEFAULT '',
			size_bytes INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'ready',
			summary TEXT NOT NULL DEFAULT '',
			preview TEXT NOT NULL DEFAULT '',
			source_kind TEXT NOT NULL DEFAULT 'attachment',
			sources_json TEXT NOT NULL DEFAULT '[]',
			retention TEXT NOT NULL DEFAULT 'conversation',
			content TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS chat_attachments_conversation_idx
			ON chat_attachments(conversation_id, created_at);`,
		`CREATE INDEX IF NOT EXISTS chat_attachments_user_message_idx
			ON chat_attachments(user_message_id, created_at);`,
		`CREATE TABLE IF NOT EXISTS skills_used (
			id TEXT PRIMARY KEY,
			conversation_id TEXT,
			skill_name TEXT NOT NULL,
			skill_version TEXT,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			content TEXT NOT NULL,
			importance INTEGER NOT NULL DEFAULT 1,
			source TEXT,
			pinned INTEGER NOT NULL DEFAULT 0,
			disabled INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
			content,
			memory_id UNINDEXED
		);`,
	}

	for index, statement := range migrations {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply migration %d: %w", index+1, err)
		}
	}
	if err := s.ensureColumn(ctx, "conversations", "starred", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "messages", "parent_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "messages", "variant_index", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "messages", "active_variant", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "tool_runs", "session_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "tool_runs", "user_message_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "tool_runs", "assistant_message_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "tool_runs", "parent_message_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "tool_runs", "variant_index", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureColumn(ctx context.Context, table string, column string, definition string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return fmt.Errorf("inspect %s columns: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan %s column: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read %s columns: %w", table, err)
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition); err != nil {
		return fmt.Errorf("add %s.%s column: %w", table, column, err)
	}
	return nil
}

func decodeJSONValue(input string) any {
	var value any
	if err := json.Unmarshal([]byte(input), &value); err != nil {
		return input
	}
	return value
}

func decodeJSONStringSlice(input string) []string {
	var values []string
	if err := json.Unmarshal([]byte(input), &values); err == nil {
		return values
	}
	raw := decodeJSONValue(input)
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(fmt.Sprint(item))
		if text != "" && text != "<nil>" {
			out = append(out, text)
		}
	}
	return out
}

func scanChatAttachments(rows *sql.Rows) ([]ChatAttachment, error) {
	var items []ChatAttachment
	for rows.Next() {
		var item ChatAttachment
		var sourcesJSON string
		if err := rows.Scan(
			&item.ID,
			&item.ConversationID,
			&item.UserMessageID,
			&item.FileName,
			&item.ContentType,
			&item.SizeBytes,
			&item.Status,
			&item.Summary,
			&item.Preview,
			&item.SourceKind,
			&sourcesJSON,
			&item.Retention,
			&item.Content,
			&item.CreatedAt,
			&item.ExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("scan chat attachment: %w", err)
		}
		item.Sources = decodeJSONStringSlice(sourcesJSON)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read chat attachments: %w", err)
	}
	return items, nil
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func scanAgentTurns(rows *sql.Rows) ([]AgentTurn, error) {
	var turns []AgentTurn
	for rows.Next() {
		var turn AgentTurn
		var traceJSON string
		var sourcesJSON string
		if err := rows.Scan(
			&turn.ID,
			&turn.ConversationID,
			&turn.SessionID,
			&turn.UserMessageID,
			&turn.AssistantMessageID,
			&turn.Status,
			&traceJSON,
			&sourcesJSON,
			&turn.SourceKind,
			&turn.Model,
			&turn.CreatedAt,
			&turn.UpdatedAt,
			&turn.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent turn: %w", err)
		}
		turn.Trace = decodeJSONStringSlice(traceJSON)
		turn.Sources = decodeJSONStringSlice(sourcesJSON)
		turns = append(turns, turn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read agent turns: %w", err)
	}
	return turns, nil
}

func scanMemorySearchResults(rows *sql.Rows) ([]MemorySearchResult, error) {
	var results []MemorySearchResult
	for rows.Next() {
		var result MemorySearchResult
		var pinned int
		if err := rows.Scan(
			&result.ID,
			&result.Kind,
			&result.Content,
			&result.Snippet,
			&result.Importance,
			&result.Source,
			&pinned,
			&result.CreatedAt,
			&result.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan memory result: %w", err)
		}
		result.Pinned = pinned != 0
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read memory results: %w", err)
	}
	return results, nil
}

func scanToolRuns(rows *sql.Rows) ([]ToolRun, error) {
	var runs []ToolRun
	for rows.Next() {
		var run ToolRun
		var inputJSON string
		var outputJSON string
		if err := rows.Scan(
			&run.ID,
			&run.ConversationID,
			&run.SessionID,
			&run.UserMessageID,
			&run.AssistantMessageID,
			&run.ParentMessageID,
			&run.VariantIndex,
			&run.ToolName,
			&inputJSON,
			&outputJSON,
			&run.Status,
			&run.RiskLevel,
			&run.CreatedAt,
			&run.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan tool run: %w", err)
		}
		run.Input = decodeJSONValue(inputJSON)
		run.Output = decodeJSONValue(outputJSON)
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tool runs: %w", err)
	}
	return runs, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func BuildConversationSummary(messages []Message, maxChars int) string {
	if maxChars <= 0 {
		maxChars = 1000
	}
	messages = ActiveConversationMessages(messages)
	var lines []string
	used := 0
	for _, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			role = "message"
		}
		line := role + ": " + compactSummaryText(sanitizeTextForStorage(msg.Content), 240)
		if strings.TrimSpace(line) == role+":" {
			continue
		}
		if used+len(line) > maxChars {
			remaining := maxChars - used
			if remaining > len(role)+8 {
				lines = append(lines, compactSummaryText(line, remaining))
			}
			break
		}
		lines = append(lines, line)
		used += len(line) + 1
	}
	return strings.Join(lines, "\n")
}

func ActiveConversationMessages(messages []Message) []Message {
	if len(messages) == 0 {
		return []Message{}
	}
	messages = WithInferredResponseParents(messages)
	userActive := make(map[string]bool)
	for _, msg := range messages {
		if msg.Role == "user" && strings.TrimSpace(msg.ID) != "" {
			userActive[msg.ID] = msg.ActiveVariant
		}
	}
	out := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if !msg.ActiveVariant {
			continue
		}
		if msg.Role == "assistant" {
			if active, ok := userActive[strings.TrimSpace(msg.ParentID)]; ok && !active {
				continue
			}
		}
		out = append(out, msg)
	}
	return out
}

func compactSummaryText(input string, maxChars int) string {
	text := strings.Join(strings.Fields(input), " ")
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	if maxChars <= 3 {
		return text[:maxChars]
	}
	return strings.TrimSpace(text[:maxChars-3]) + "..."
}

var (
	storageSecretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(api[_-]?key|token|password|secret)\b\s*[:=]\s*["']?[^"'\s,}]+`),
		regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{12,}\b`),
		regexp.MustCompile(`\b[A-Za-z0-9_-]{24,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{20,}\b`),
	}
	storageUserPathPattern = regexp.MustCompile(`/Users/[^/\s]+/`)
)

func sanitizeValueForStorage(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return sanitizeTextForStorage(typed)
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return typed
	case map[string]any:
		out := map[string]any{}
		for key, item := range typed {
			out[key] = sanitizeValueForStorage(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, sanitizeValueForStorage(item))
		}
		return out
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return sanitizeTextForStorage(fmt.Sprint(typed))
		}
		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			return sanitizeTextForStorage(fmt.Sprint(typed))
		}
		switch decoded.(type) {
		case string, map[string]any, []any:
			return sanitizeValueForStorage(decoded)
		default:
			return decoded
		}
	}
}

func sanitizeTextForStorage(input string) string {
	output := input
	for _, pattern := range storageSecretPatterns {
		output = pattern.ReplaceAllStringFunc(output, redactSecretForStorage)
	}
	output = storageUserPathPattern.ReplaceAllString(output, "~/")
	return output
}

func sanitizeStringSliceForStorage(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(sanitizeTextForStorage(value))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func redactSecretForStorage(input string) string {
	if key, _, ok := strings.Cut(input, "="); ok {
		return strings.TrimSpace(key) + "=[REDACTED]"
	}
	if key, _, ok := strings.Cut(input, ":"); ok {
		return strings.TrimSpace(key) + ": [REDACTED]"
	}
	return "[REDACTED]"
}

func normalizeMemoryContent(input string) string {
	return strings.ToLower(strings.Join(strings.Fields(input), " "))
}

func buildFTSQuery(query string) string {
	terms := strings.FieldsFunc(query, func(r rune) bool {
		return !(r == '_' || unicode.IsDigit(r) || unicode.IsLetter(r))
	})
	for i, term := range terms {
		terms[i] = strings.Trim(term, "_")
	}
	clean := terms[:0]
	for _, term := range terms {
		if term != "" {
			clean = append(clean, quoteFTSTerm(term))
		}
	}
	return strings.Join(clean, " ")
}

func quoteFTSTerm(term string) string {
	return `"` + strings.ReplaceAll(term, `"`, `""`) + `"`
}

func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func compactTitle(input string) string {
	title := strings.TrimSpace(input)
	title = strings.ReplaceAll(title, "\n", " ")
	if title == "" {
		return "New conversation"
	}
	if len(title) > 80 {
		return title[:80]
	}
	return title
}

func newID(prefix string) string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		now := time.Now().UTC().UnixNano()
		return fmt.Sprintf("%s_%x", prefix, now)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return prefix + "_" + hex.EncodeToString(bytes[:])
}

func BaseName(path string) string {
	return filepath.Base(path)
}

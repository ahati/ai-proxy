// Package conversation provides in-memory storage for multi-turn conversations.
// It implements an LRU (Least Recently Used) cache with TTL-based expiration
// to store conversation history for the Responses API's previous_response_id feature.
package conversation

import (
	"container/list"
	"sync"
	"time"

	"ai-proxy/types"
)

// Conversation represents a stored conversation with its history.
// It stores both the input items from the request and the output items
// from the response, allowing the full conversation to be reconstructed.
type Conversation struct {
	// ID is the unique identifier for this conversation (response_id).
	ID string
	// PreviousResponseID points to the parent conversation in the chain.
	// This enables linked-list traversal for multi-turn conversations.
	PreviousResponseID string
	// Input contains the original input items from the request.
	Input []types.InputItem
	// Output contains the response output items.
	Output []types.OutputItem
	// ReasoningItemID is the ID of the reasoning output item, if any.
	// This is passed to the upstream LLM on continuation to enable
	// reasoning continuity across turns.
	ReasoningItemID string
	// EncryptedReasoning stores the encrypted blob in ZDR mode.
	// When store:false, this contains the encrypted reasoning data.
	EncryptedReasoning string
	// Persisted indicates whether this conversation should be visible via
	// the Responses API CRUD endpoints. Controlled by the OpenAI `store` flag.
	// When false (store:false), the conversation is kept only for bridge
	// operation and is cleaned up when the WebSocket connection closes.
	Persisted bool
	// UserID is the owner of this conversation.
	// Used for access control to prevent cross-user access.
	UserID string
	// OrgID is the organization ID for this conversation.
	// Used for organization-level access control.
	OrgID string
	// CreatedAt is the timestamp when the conversation was created.
	CreatedAt time.Time
	// ExpiresAt is the timestamp when the conversation should be expired.
	ExpiresAt time.Time
}

// Config holds configuration for the conversation store.
type Config struct {
	// MaxSize is the maximum number of conversations to store.
	// When the limit is reached, the least recently used conversations
	// are evicted. Default: 1000.
	MaxSize int
	// TTL is the time-to-live for conversations.
	// Conversations older than this are automatically expired.
	// Default: 24 hours.
	TTL time.Duration
}

// Store provides thread-safe LRU storage for conversations.
// It uses a combination of a map for O(1) lookups and a doubly-linked
// list for O(1) LRU ordering operations.
type Store struct {
	mu     sync.RWMutex
	config Config
	data   map[string]*list.Element // response_id -> list element
	lru    *list.List               // LRU order (front = most recent)
}

// entry represents an element in the LRU list.
type entry struct {
	key          string
	conversation *Conversation
}

// NewStore creates a new conversation store with the given configuration.
// If maxSize is 0, it defaults to 1000.
// If TTL is 0, it defaults to 24 hours.
func NewStore(config Config) *Store {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}
	if config.TTL <= 0 {
		config.TTL = 24 * time.Hour
	}
	return &Store{
		config: config,
		data:   make(map[string]*list.Element),
		lru:    list.New(),
	}
}

// Get retrieves a conversation by ID.
// Returns nil if the conversation is not found or has expired.
// Accessing a conversation moves it to the front of the LRU list.
func (s *Store) Get(id string) *Conversation {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up expired entries lazily
	s.cleanupExpired()

	if elem, ok := s.data[id]; ok {
		conv := elem.Value.(*entry).conversation
		// Check if expired
		if time.Now().After(conv.ExpiresAt) {
			s.deleteElement(elem)
			return nil
		}
		// Move to front (most recently used)
		s.lru.MoveToFront(elem)
		return conv
	}
	return nil
}

// WalkChain walks the linked-list chain of conversations backward from the given ID.
// It returns all conversations in chronological order (oldest first).
// The chain traversal follows PreviousResponseID pointers until it reaches
// a conversation with no parent or a missing conversation.
func (s *Store) WalkChain(id string) []*Conversation {
	var chain []*Conversation
	cursor := id
	for cursor != "" {
		conv := s.Get(cursor)
		if conv == nil {
			break
		}
		// Append to slice (O(1) amortized)
		chain = append(chain, conv)
		cursor = conv.PreviousResponseID
	}
	// Reverse to get chronological order (oldest first)
	// This is O(n) instead of O(n²) prepending
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// OwnershipError indicates that a conversation chain access was denied
// due to ownership mismatch.
type OwnershipError struct {
	ResponseID string
	UserID     string
	ExpectedID string
}

func (e *OwnershipError) Error() string {
	return "previous_response_id does not belong to user"
}

// WalkChainWithOwnership walks the conversation chain and validates ownership.
// Returns OwnershipError if the root conversation belongs to a different user.
// If userID is empty, ownership check is skipped (for backwards compatibility).
func (s *Store) WalkChainWithOwnership(id string, userID string) ([]*Conversation, error) {
	chain := s.WalkChain(id)
	if len(chain) == 0 {
		return nil, nil
	}

	// Skip ownership check if no userID provided (backwards compatibility)
	if userID == "" {
		return chain, nil
	}

	// Validate ownership of root conversation
	root := chain[0]
	if root.UserID != "" && root.UserID != userID {
		return nil, &OwnershipError{
			ResponseID: id,
			UserID:     userID,
			ExpectedID: root.UserID,
		}
	}

	return chain, nil
}

// Store saves a conversation, evicting the oldest if at capacity.
// If a conversation with the same ID already exists, it is replaced.
func (s *Store) Store(conv *Conversation) {
	if conv == nil || conv.ID == "" {
		return
	}

	// Set expiration time if not set
	if conv.ExpiresAt.IsZero() {
		conv.ExpiresAt = time.Now().Add(s.config.TTL)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up expired entries lazily
	s.cleanupExpired()

	// If already exists, remove old entry
	if elem, ok := s.data[conv.ID]; ok {
		s.deleteElement(elem)
	}

	// Check capacity and evict if necessary
	for s.lru.Len() >= s.config.MaxSize {
		s.evictOldest()
	}

	// Add new entry at front
	elem := s.lru.PushFront(&entry{
		key:          conv.ID,
		conversation: conv,
	})
	s.data[conv.ID] = elem
}

// Delete removes a conversation by ID.
func (s *Store) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if elem, ok := s.data[id]; ok {
		s.deleteElement(elem)
	}
}

// deleteElement removes an element from both the map and the list.
// Must be called with lock held.
func (s *Store) deleteElement(elem *list.Element) {
	if elem == nil {
		return
	}
	ent := elem.Value.(*entry)
	delete(s.data, ent.key)
	s.lru.Remove(elem)
}

// evictOldest removes the least recently used conversation.
// Must be called with lock held.
func (s *Store) evictOldest() {
	elem := s.lru.Back()
	if elem != nil {
		s.deleteElement(elem)
	}
}

// cleanupExpired removes all expired conversations.
// Must be called with lock held.
func (s *Store) cleanupExpired() {
	now := time.Now()
	// Iterate from back (oldest) to front (newest)
	for elem := s.lru.Back(); elem != nil; {
		next := elem.Prev()
		conv := elem.Value.(*entry).conversation
		if now.After(conv.ExpiresAt) {
			s.deleteElement(elem)
		}
		elem = next
	}
}

// DefaultStore is the global conversation store instance.
// It is initialized by the main package at startup.
var DefaultStore *Store

// InitDefaultStore initializes the global conversation store.
func InitDefaultStore(config Config) {
	DefaultStore = NewStore(config)
}

// GetFromDefault retrieves a conversation from the default store.
// Returns nil if the default store is not initialized.
func GetFromDefault(id string) *Conversation {
	if DefaultStore == nil {
		return nil
	}
	return DefaultStore.Get(id)
}

// WalkChainFromDefaultWithOwnership walks the conversation chain with ownership validation.
// Returns OwnershipError if the conversation belongs to a different user.
func WalkChainFromDefaultWithOwnership(id string, userID string) ([]*Conversation, error) {
	if DefaultStore == nil {
		return nil, nil
	}
	return DefaultStore.WalkChainWithOwnership(id, userID)
}

// StoreInDefault saves a conversation to the default store.
// Does nothing if the default store is not initialized.
func StoreInDefault(conv *Conversation) {
	if DefaultStore == nil {
		return
	}
	DefaultStore.Store(conv)
}

// DeleteFromDefault removes a conversation from the default store by ID.
// Does nothing if the default store is not initialized or the ID is empty.
func DeleteFromDefault(id string) {
	if DefaultStore == nil || id == "" {
		return
	}
	DefaultStore.Delete(id)
}

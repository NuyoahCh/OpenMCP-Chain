package auth

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
)

// MemoryStore 是基于内存的身份认证存储实现，适用于测试和轻量级场景。
type MemoryStore struct {
	mu     sync.RWMutex
	users  map[string]*User
	byID   map[int64]*Subject
	nextID int64
}

// NewMemoryStore 使用初始种子数据创建 MemoryStore 实例。
func NewMemoryStore(seeds []Seed) (*MemoryStore, error) {
	// 初始化存储结构。
	store := &MemoryStore{
		users:  make(map[string]*User),
		byID:   make(map[int64]*Subject),
		nextID: 1,
	}
	// 应用种子数据。
	for _, seed := range seeds {
		if strings.TrimSpace(seed.Username) == "" {
			continue
		}
		if _, exists := store.users[seed.Username]; exists {
			continue
		}
		hashed, err := HashPassword(seed.Password)
		if err != nil {
			return nil, err
		}
		subject := &Subject{
			ID:          store.nextID,
			Username:    seed.Username,
			Roles:       dedupeStrings(seed.Roles),
			Permissions: dedupeStrings(seed.Permissions),
			Disabled:    seed.Disabled,
		}
		subject.normalise()
		user := &User{
			ID:           subject.ID,
			Username:     seed.Username,
			PasswordHash: hashed,
			Disabled:     seed.Disabled,
		}
		store.users[seed.Username] = user
		store.byID[subject.ID] = subject
		store.nextID++
	}
	return store, nil
}

// ApplySeed 应用单个种子数据以添加或更新用户记录。
func (s *MemoryStore) ApplySeed(_ context.Context, seed Seed) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.users == nil {
		s.users = make(map[string]*User)
	}
	if s.byID == nil {
		s.byID = make(map[int64]*Subject)
	}
	username := strings.TrimSpace(seed.Username)
	if username == "" {
		return errors.New("seed username cannot be empty")
	}
	hashed, err := HashPassword(seed.Password)
	if err != nil {
		return err
	}
	user, ok := s.users[username]
	if !ok {
		if s.nextID == 0 {
			s.nextID = 1
		}
		user = &User{ID: s.nextID}
		s.nextID++
	}
	user.Username = username
	user.PasswordHash = hashed
	user.Disabled = seed.Disabled
	s.users[username] = user
	subject := &Subject{
		ID:          user.ID,
		Username:    username,
		Roles:       dedupeStrings(seed.Roles),
		Permissions: dedupeStrings(seed.Permissions),
		Disabled:    seed.Disabled,
	}
	subject.normalise()
	s.byID[user.ID] = subject
	return nil
}

// FindUserByUsername 根据用户名查找用户记录。
func (s *MemoryStore) FindUserByUsername(_ context.Context, username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if user, ok := s.users[strings.TrimSpace(username)]; ok {
		clone := *user
		return &clone, nil
	}
	return nil, errors.New("user not found")
}

// LoadSubject 根据用户 ID 加载主体信息。
func (s *MemoryStore) LoadSubject(_ context.Context, userID int64) (*Subject, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if subject, ok := s.byID[userID]; ok {
		clone := subject.Clone()
		return clone, nil
	}
	return nil, errors.New("subject not found")
}

// dedupeStrings 去重并规范化字符串切片。
func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		seen[strings.ToLower(value)] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for key := range seen {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

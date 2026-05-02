package state

import (
	"sync"
	"time"
)

type StateType int

const (
	StateIdle StateType = iota
	StateAwaitingCardSecrets
	StateAwaitingFulfillSecrets
)

type ConversationState struct {
	Type      StateType
	Data      map[string]interface{}
	ExpiresAt time.Time
}

func (s *ConversationState) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

type Manager struct {
	mu       sync.RWMutex
	states   map[int64]*ConversationState
	ttl      time.Duration
	stopCh   chan struct{}
}

func NewManager(ttl time.Duration) *Manager {
	m := &Manager{
		states: make(map[int64]*ConversationState),
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}
	go m.cleanupLoop()
	return m
}

func (m *Manager) Set(chatID int64, stateType StateType, data map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[chatID] = &ConversationState{
		Type:      stateType,
		Data:      data,
		ExpiresAt: time.Now().Add(m.ttl),
	}
}

func (m *Manager) Get(chatID int64) (*ConversationState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.states[chatID]
	if !ok || s.IsExpired() {
		return nil, false
	}
	return s, true
}

func (m *Manager) Clear(chatID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, chatID)
}

func (m *Manager) Stop() {
	close(m.stopCh)
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.states {
		if s.IsExpired() {
			delete(m.states, id)
		}
	}
}

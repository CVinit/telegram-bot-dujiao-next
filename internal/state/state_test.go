package state

import (
	"testing"
	"time"
)

func TestManagerSetGet(t *testing.T) {
	m := NewManager(5 * time.Minute)

	m.Set(123, StateAwaitingFulfillSecrets, map[string]interface{}{
		"product_name": "测试商品",
		"total_qty":    3,
	})

	s, ok := m.Get(123)
	if !ok {
		t.Fatal("state not found")
	}
	if s.Type != StateAwaitingFulfillSecrets {
		t.Errorf("Type = %d, want %d", s.Type, StateAwaitingFulfillSecrets)
	}
	name, _ := s.Data["product_name"].(string)
	if name != "测试商品" {
		t.Errorf("product_name = %q, want %q", name, "测试商品")
	}
	qty, _ := s.Data["total_qty"].(int)
	if qty != 3 {
		t.Errorf("total_qty = %d, want 3", qty)
	}
}

func TestManagerExpired(t *testing.T) {
	m := NewManager(50 * time.Millisecond)

	m.Set(456, StateAwaitingCardSecrets, map[string]interface{}{
		"product_id": 1,
	})

	s, ok := m.Get(456)
	if !ok {
		t.Fatal("state not found immediately after set")
	}
	_ = s

	time.Sleep(100 * time.Millisecond)

	_, ok = m.Get(456)
	if ok {
		t.Error("expected state to be expired after 100ms")
	}
}

func TestManagerClear(t *testing.T) {
	m := NewManager(5 * time.Minute)

	m.Set(789, StateAwaitingFulfillSecrets, map[string]interface{}{})
	_, ok := m.Get(789)
	if !ok {
		t.Fatal("state not found after set")
	}

	m.Clear(789)
	_, ok = m.Get(789)
	if ok {
		t.Error("expected state to be cleared")
	}
}

func TestManagerOverwrite(t *testing.T) {
	m := NewManager(5 * time.Minute)

	m.Set(111, StateAwaitingCardSecrets, map[string]interface{}{
		"key": "first",
	})
	m.Set(111, StateAwaitingFulfillSecrets, map[string]interface{}{
		"key": "second",
	})

	s, ok := m.Get(111)
	if !ok {
		t.Fatal("state not found")
	}
	if s.Type != StateAwaitingFulfillSecrets {
		t.Errorf("Type = %d, want %d", s.Type, StateAwaitingFulfillSecrets)
	}
	val, _ := s.Data["key"].(string)
	if val != "second" {
		t.Errorf("key = %q, want %q", val, "second")
	}
}

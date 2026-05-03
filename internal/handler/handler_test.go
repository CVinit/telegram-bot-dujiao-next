package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/v/telegram-bot-dujiao-next/internal/model"
)

func TestParseSecrets(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"single line", "key1", 1},
		{"multi line", "key1\nkey2\nkey3", 3},
		{"trailing newline", "key1\nkey2\n", 2},
		{"blank lines", "key1\n\nkey2\n\n", 2},
		{"whitespace trimmed", "  key1  \n  key2  ", 2},
		{"empty input", "", 0},
		{"only whitespace", "  \n  \n  ", 0},
		{"mixed", "key1\n\n  key2  \n\nkey3", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSecrets(tt.input)
			if len(got) != tt.want {
				t.Errorf("parseSecrets(%q) = %v, want %d items", tt.input, got, tt.want)
			}
		})
	}
}

func TestIntFromIface(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  int
	}{
		{"int", 42, 42},
		{"int64", int64(42), 42},
		{"float64", float64(42), 42},
		{"json.Number", json.Number("42"), 42},
		{"nil", nil, 0},
		{"string", "not a number", 0},
		{"zero int", 0, 0},
		{"negative", -1, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intFromIface(tt.input)
			if got != tt.want {
				t.Errorf("intFromIface(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestUintFromIface(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  uint
	}{
		{"uint", uint(42), 42},
		{"int", 42, 42},
		{"int64", int64(42), 42},
		{"float64", float64(42), 42},
		{"json.Number", json.Number("42"), 42},
		{"nil", nil, 0},
		{"string", "not a number", 0},
		{"zero", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uintFromIface(tt.input)
			if got != tt.want {
				t.Errorf("uintFromIface(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestCallbackDataParsing(t *testing.T) {
	tests := []struct {
		name       string
		rawData    string
		wantPrefix string
		wantSuffix string
	}{
		{"sales today", "\fsales|today", "sales", "today"},
		{"sales month", "\fsales|month", "sales", "month"},
		{"cards product", "\fcards|1", "cards", "1"},
		{"cards_sku", "\fcards_sku|5", "cards_sku", "5"},
		{"fulfill chinese", "\ffulfill|土耳其Apple ID", "fulfill", "土耳其Apple ID"},
		{"no prefix char", "fulfill|土耳其Apple ID", "fulfill", "土耳其Apple ID"},
		{"no suffix", "\fsales", "sales", ""},
		{"empty data", "", "", ""},
		{"suffix with pipes", "\ffulfill|name|with|pipes", "fulfill", "name|with|pipes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.rawData
			if len(data) > 0 && data[0] == '\f' {
				data = data[1:]
			}

			parts := strings.SplitN(data, "|", 2)
			prefix := parts[0]
			suffix := ""
			if len(parts) > 1 {
				suffix = parts[1]
			}

			if prefix != tt.wantPrefix || suffix != tt.wantSuffix {
				t.Errorf("parse(%q) = prefix=%q suffix=%q, want prefix=%q suffix=%q",
					tt.rawData, prefix, suffix, tt.wantPrefix, tt.wantSuffix)
			}
		})
	}
}

func TestLeafOrderResolution(t *testing.T) {
	orders := []model.Order{
		{
			ID:     35,
			Status: "partially_delivered",
			Children: []model.Order{
				{ID: 36, Status: "delivered"},
				{ID: 37, Status: "fulfilling"},
				{ID: 38, Status: "fulfilling"},
			},
		},
		{
			ID:     39,
			Status: "partially_delivered",
			Children: []model.Order{
				{ID: 40, Status: "delivered"},
				{ID: 41, Status: "fulfilling"},
			},
		},
		{
			ID:     50,
			Status: "fulfilling",
		},
	}

	var leafOrders []model.Order
	for _, o := range orders {
		if len(o.Children) > 0 {
			for _, ch := range o.Children {
				if ch.Status == "fulfilling" || ch.Status == "paid" {
					leafOrders = append(leafOrders, ch)
				}
			}
		} else {
			leafOrders = append(leafOrders, o)
		}
	}

	// Should get: 37, 38, 41, 50 (not 36 or 40 which are delivered)
	wantIDs := []uint{37, 38, 41, 50}
	if len(leafOrders) != len(wantIDs) {
		t.Fatalf("got %d leaf orders, want %d", len(leafOrders), len(wantIDs))
	}
	for i, want := range wantIDs {
		if leafOrders[i].ID != want {
			t.Errorf("leafOrders[%d].ID = %d, want %d", i, leafOrders[i].ID, want)
		}
	}
}

func TestLeafOrderResolutionAllDelivered(t *testing.T) {
	orders := []model.Order{
		{
			ID:     35,
			Status: "partially_delivered",
			Children: []model.Order{
				{ID: 36, Status: "delivered"},
				{ID: 37, Status: "delivered"},
			},
		},
	}

	var leafOrders []model.Order
	for _, o := range orders {
		if len(o.Children) > 0 {
			for _, ch := range o.Children {
				if ch.Status == "fulfilling" || ch.Status == "paid" {
					leafOrders = append(leafOrders, ch)
				}
			}
		} else {
			leafOrders = append(leafOrders, o)
		}
	}

	if len(leafOrders) != 0 {
		t.Errorf("expected 0 leaf orders when all children delivered, got %d", len(leafOrders))
	}
}

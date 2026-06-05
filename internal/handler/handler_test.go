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

func TestOrderItemSummaryIncludesSKUName(t *testing.T) {
	order := model.Order{
		Items: []model.OrderItem{{
			Title:    map[string]interface{}{"zh-CN": "ChatGPT Plus"},
			Quantity: 2,
			SKUSnapshot: map[string]interface{}{
				"sku_code":    "PLUS-MONTH",
				"spec_values": map[string]interface{}{"zh-CN": "月付"},
			},
		}},
	}

	got := orderItemSummary(order)
	for _, part := range []string{"ChatGPT Plus / 月付", "x2"} {
		if !strings.Contains(got, part) {
			t.Errorf("orderItemSummary() = %q, want %q", got, part)
		}
	}
}

func TestBuildFulfillGroupsSeparatesSKUs(t *testing.T) {
	orders := []model.Order{
		{
			ID: 1,
			Items: []model.OrderItem{{
				ProductID: 10,
				SKUID:     100,
				Title:     map[string]interface{}{"zh-CN": "ChatGPT Plus"},
				SKUSnapshot: map[string]interface{}{
					"sku_code":    "MONTH",
					"spec_values": map[string]interface{}{"zh-CN": "月付"},
				},
				Quantity: 1,
			}},
		},
		{
			ID: 2,
			Items: []model.OrderItem{{
				ProductID: 10,
				SKUID:     200,
				Title:     map[string]interface{}{"zh-CN": "ChatGPT Plus"},
				SKUSnapshot: map[string]interface{}{
					"sku_code":    "YEAR",
					"spec_values": map[string]interface{}{"zh-CN": "年付"},
				},
				Quantity: 3,
			}},
		},
	}

	groups := buildFulfillGroups(orders)
	if len(groups) != 2 {
		t.Fatalf("buildFulfillGroups() returned %d groups, want 2", len(groups))
	}

	got := map[string]fulfillGroup{}
	for _, group := range groups {
		got[group.Key] = group
	}
	if got["10:100"].Name != "ChatGPT Plus / 月付" || got["10:100"].TotalQty != 1 {
		t.Errorf("monthly group = %+v", got["10:100"])
	}
	if got["10:200"].Name != "ChatGPT Plus / 年付" || got["10:200"].TotalQty != 3 {
		t.Errorf("yearly group = %+v", got["10:200"])
	}
}

func TestFormatStockOverviewShowsSKULevelStockAndOfflineMarker(t *testing.T) {
	active := true
	inactive := false
	products := []model.Product{
		{
			Title:              map[string]interface{}{"zh-CN": "土耳其区Apple ID/账号"},
			AutoStockAvailable: 37,
			IsActive:           &active,
			SKUs: []model.SKU{
				{
					ID:                 1,
					SKUCode:            "TURKEY-2FA",
					SpecValues:         map[string]interface{}{"zh-CN": "双重认证登录"},
					AutoStockAvailable: 20,
					IsActive:           &active,
				},
				{
					ID:                 2,
					SKUCode:            "TURKEY-SECURITY",
					SpecValues:         map[string]interface{}{"zh-CN": "密保问题登录"},
					AutoStockAvailable: 6,
					IsActive:           &active,
				},
			},
		},
		{
			Title:              map[string]interface{}{"zh-CN": "土耳其Apple ID 带密保"},
			AutoStockAvailable: 6,
			IsActive:           &inactive,
			SKUs: []model.SKU{{
				ID:                 3,
				SKUCode:            "TURKEY-QUESTION",
				SpecValues:         map[string]interface{}{"zh-CN": "密保账号"},
				AutoStockAvailable: 6,
				IsActive:           &active,
			}},
		},
	}

	got := formatStockOverview(products, 10)
	for _, part := range []string{
		"📦 土耳其区Apple ID/账号",
		"✅ 总库存：37",
		"✅ SKU: 双重认证登录 库存：20",
		"⚠️ SKU: 密保问题登录 库存：6 (低库存!)",
		"📦 土耳其Apple ID 带密保（已下架）",
		"⚠️ SKU: 密保账号 库存：6 (低库存!)",
	} {
		if !strings.Contains(got, part) {
			t.Errorf("formatStockOverview() = %q, want substring %q", got, part)
		}
	}
}

func TestFormatStockOverviewDoesNotMarkMissingActiveFlagOffline(t *testing.T) {
	got := formatStockOverview([]model.Product{{
		Title:              map[string]interface{}{"zh-CN": "未返回状态商品"},
		AutoStockAvailable: 1,
	}}, 10)

	if strings.Contains(got, "已下架") {
		t.Errorf("formatStockOverview() = %q, should not mark missing is_active as offline", got)
	}
	if !strings.Contains(got, "⚠️ 库存：1 (低库存!)") {
		t.Errorf("formatStockOverview() = %q, should keep product stock fallback for products without SKUs", got)
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

func TestCollectNewPaidOrders(t *testing.T) {
	paidAt := "2026-05-22T13:00:00Z"
	orders := []model.Order{
		{ID: 1, OrderNo: "PENDING", Status: "pending_payment"},
		{ID: 2, OrderNo: "PAID-SEEN", Status: "paid", PaidAt: &paidAt},
		{ID: 3, OrderNo: "AUTO-DONE", Status: "completed", PaidAt: &paidAt},
	}
	seen := map[uint]struct{}{2: {}}

	got := collectNewPaidOrders(seen, orders)
	if len(got) != 1 || got[0].ID != 3 {
		t.Fatalf("collectNewPaidOrders() = %+v, want only order 3", got)
	}
	if _, ok := seen[3]; !ok {
		t.Fatal("collectNewPaidOrders() did not mark new paid order as seen")
	}
	if _, ok := seen[1]; ok {
		t.Fatal("collectNewPaidOrders() marked unpaid order as seen")
	}
}

func TestSeedPaidOrderIDs(t *testing.T) {
	paidAt := "2026-05-22T13:00:00Z"
	emptyPaidAt := " "
	seen := map[uint]struct{}{}

	seedPaidOrderIDs(seen, []model.Order{
		{ID: 1, Status: "paid", PaidAt: &paidAt},
		{ID: 2, Status: "completed", PaidAt: &emptyPaidAt},
		{ID: 3, Status: "fulfilling"},
	})

	if len(seen) != 1 {
		t.Fatalf("seedPaidOrderIDs() marked %d orders, want 1", len(seen))
	}
	if _, ok := seen[1]; !ok {
		t.Fatal("seedPaidOrderIDs() did not mark paid order")
	}
}

func TestFormatAlertLineUsesSKUSpecValues(t *testing.T) {
	got := formatAlertLine(model.InventoryAlert{
		ProductTitle:   map[string]interface{}{"zh-CN": "ChatGPT Plus"},
		SKUCode:        "PLUS-HK",
		SKUSpecValues:  map[string]interface{}{"zh-CN": "香港区"},
		AvailableStock: 1,
		TotalStock:     10,
	})

	for _, part := range []string{"ChatGPT Plus", "香港区", "可用 1 / 总计 10"} {
		if !strings.Contains(got, part) {
			t.Errorf("formatAlertLine() = %q, want %q", got, part)
		}
	}
	if strings.Contains(got, "PLUS-HK") {
		t.Errorf("formatAlertLine() = %q, should prefer SKU spec values over sku_code", got)
	}
}

func TestFormatPaidOrderAlertLine(t *testing.T) {
	got := formatPaidOrderAlertLine(model.Order{
		OrderNo:     "DJ-PAID-001",
		Status:      "completed",
		TotalAmount: "19.90",
		Items: []model.OrderItem{{
			Title:    map[string]interface{}{"zh-CN": "自动卡密商品"},
			Quantity: 2,
		}},
	})

	for _, part := range []string{"DJ-PAID-001", "自动卡密商品 x2", "19.90", "completed"} {
		if !strings.Contains(got, part) {
			t.Errorf("formatPaidOrderAlertLine() = %q, want %q", got, part)
		}
	}
}

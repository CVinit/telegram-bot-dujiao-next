package model

import (
	"encoding/json"
	"testing"
)

func TestGetProductName(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"plain string", "hello", "hello"},
		{"zh-CN map", map[string]interface{}{"zh-CN": "苹果ID", "en-US": "Apple ID"}, "苹果ID"},
		{"zh only", map[string]interface{}{"zh": "中文名"}, "中文名"},
		{"en fallback", map[string]interface{}{"en-US": "English Name"}, "English Name"},
		{"en fallback no country", map[string]interface{}{"en": "English"}, "English"},
		{"zh-TW fallback", map[string]interface{}{"zh-TW": "繁體名"}, "繁體名"},
		{"empty map", map[string]interface{}{}, "未知商品"},
		{"nil", nil, "未知商品"},
		{"int type", 42, "未知商品"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetProductName(tt.input)
			if got != tt.want {
				t.Errorf("GetProductName(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResponseUnmarshal(t *testing.T) {
	raw := `{"status_code":0,"msg":"success","data":{"id":1,"title":"test"}}`
	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0", resp.StatusCode)
	}
	if resp.Msg != "success" {
		t.Errorf("Msg = %q, want %q", resp.Msg, "success")
	}
}

func TestPageResponseUnmarshal(t *testing.T) {
	raw := `{"status_code":0,"msg":"success","data":[],"pagination":{"page":1,"page_size":20,"total":100,"total_page":5}}`
	var resp PageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Pagination.Total != 100 {
		t.Errorf("Total = %d, want 100", resp.Pagination.Total)
	}
	if resp.Pagination.Page != 1 {
		t.Errorf("Page = %d, want 1", resp.Pagination.Page)
	}
}

func TestOrderUnmarshal(t *testing.T) {
	raw := `{
		"id": 28,
		"order_no": "DJ-01",
		"status": "fulfilling",
		"total_amount": "24.00",
		"created_at": "2026-05-03T10:00:00Z",
		"items": [{"id":1,"order_id":28,"product_id":1,"sku_id":1,"quantity":2,"title":{"zh-CN":"土耳其Apple ID"}}],
		"children": []
	}`
	var order Order
	if err := json.Unmarshal([]byte(raw), &order); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if order.ID != 28 {
		t.Errorf("ID = %d, want 28", order.ID)
	}
	if order.Status != "fulfilling" {
		t.Errorf("Status = %q, want %q", order.Status, "fulfilling")
	}
	if len(order.Items) != 1 || order.Items[0].Quantity != 2 {
		t.Errorf("Items = %v, want 1 item qty=2", order.Items)
	}
	name := GetProductName(order.Items[0].Title)
	if name != "土耳其Apple ID" {
		t.Errorf("Item title name = %q, want %q", name, "土耳其Apple ID")
	}
}

func TestDashboardKPIUnmarshal(t *testing.T) {
	raw := `{
		"range": "7d",
		"from": "2026-04-26",
		"to": "2026-05-03",
		"timezone": "UTC",
		"currency": "CNY",
		"kpi": {
			"orders_total": 14,
			"paid_orders": 6,
			"completed_orders": 1,
			"gmv_paid": "84.00",
			"total_profit": "42.00",
			"profit_margin": "50.00"
		},
		"funnel": {
			"orders_created": 14,
			"orders_paid": 6,
			"orders_completed": 1
		},
		"alerts": [{"type":"payments_failed","level":"warning","value":20}]
	}`
	var overview DashboardOverview
	if err := json.Unmarshal([]byte(raw), &overview); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if overview.KPI.GMVPaid != "84.00" {
		t.Errorf("GMVPaid = %q, want %q", overview.KPI.GMVPaid, "84.00")
	}
	if overview.KPI.PaidOrders != 6 {
		t.Errorf("PaidOrders = %d, want 6", overview.KPI.PaidOrders)
	}
	if len(overview.Alerts) != 1 || overview.Alerts[0].Type != "payments_failed" {
		t.Errorf("Alerts = %v, want 1 alert of type payments_failed", overview.Alerts)
	}
}

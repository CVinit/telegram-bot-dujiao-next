package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/v/telegram-bot-dujiao-next/internal/model"
)

func TestListRecentOrdersSortsByUpdatedAtAndScansPages(t *testing.T) {
	var requestedPages []int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/admin/orders" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("sort_by"); got != "updated_at" {
			t.Errorf("sort_by = %q, want updated_at", got)
		}
		if got := r.URL.Query().Get("sort_order"); got != "desc" {
			t.Errorf("sort_order = %q, want desc", got)
		}
		if got := r.URL.Query().Get("page_size"); got != "2" {
			t.Errorf("page_size = %q, want 2", got)
		}

		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil {
			t.Fatalf("invalid page query: %v", err)
		}
		requestedPages = append(requestedPages, page)

		orders := []model.Order{{ID: uint(page), OrderNo: "ORD-" + strconv.Itoa(page)}}
		resp := struct {
			StatusCode int              `json:"status_code"`
			Msg        string           `json:"msg"`
			Data       []model.Order    `json:"data"`
			Pagination model.Pagination `json:"pagination"`
		}{
			StatusCode: 0,
			Msg:        "success",
			Data:       orders,
			Pagination: model.Pagination{Page: page, PageSize: 2, Total: 3, TotalPage: 2},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: server.Client(),
		token:      "test-token",
		expiresAt:  time.Now().Add(time.Hour),
	}

	orders, err := client.ListRecentOrders(context.Background(), 2, 3)
	if err != nil {
		t.Fatalf("ListRecentOrders() error = %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("ListRecentOrders() returned %d orders, want 2", len(orders))
	}
	if len(requestedPages) != 2 || requestedPages[0] != 1 || requestedPages[1] != 2 {
		t.Fatalf("requested pages = %v, want [1 2]", requestedPages)
	}
}

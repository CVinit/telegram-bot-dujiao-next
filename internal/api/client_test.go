package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

func TestListFulfillingOrdersIncludesPaidAndScansEveryPage(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/admin/orders" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		status := r.URL.Query().Get("status")
		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil {
			t.Fatalf("invalid page: %v", err)
		}
		requests = append(requests, status+":"+strconv.Itoa(page))

		pageCount := 1
		if status == "paid" {
			pageCount = 2
		}
		orders := []model.Order{{ID: uint(len(requests)), Status: status}}
		if page > pageCount {
			orders = nil
		}
		_ = json.NewEncoder(w).Encode(struct {
			StatusCode int              `json:"status_code"`
			Msg        string           `json:"msg"`
			Data       []model.Order    `json:"data"`
			Pagination model.Pagination `json:"pagination"`
		}{0, "success", orders, model.Pagination{Page: page, PageSize: 100, Total: int64(pageCount), TotalPage: int64(pageCount)}})
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client(), token: "test-token", expiresAt: time.Now().Add(time.Hour)}
	orders, err := client.ListFulfillingOrders(context.Background())
	if err != nil {
		t.Fatalf("ListFulfillingOrders() error = %v", err)
	}
	if len(orders) != 4 {
		t.Fatalf("got %d orders, want 4", len(orders))
	}
	wantRequests := []string{"paid:1", "paid:2", "fulfilling:1", "partially_delivered:1"}
	if strings.Join(requests, ",") != strings.Join(wantRequests, ",") {
		t.Fatalf("requests = %v, want %v", requests, wantRequests)
	}
}

func TestLoginReturnsTOTPChallengeErrorWithoutToken(t *testing.T) {
	expiresAt := "2026-09-07T12:00:00Z"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/admin/login" {
			t.Fatalf("unexpected login request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status_code": 0,
			"msg":         "success",
			"data": map[string]interface{}{
				"requires_totp":        true,
				"challenge_token":      "challenge-token",
				"challenge_expires_at": expiresAt,
			},
		})
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client(), username: "admin", password: "password", token: "stale-token", expiresAt: time.Now().Add(time.Hour)}
	err := client.login(context.Background())
	var challengeErr *TOTPChallengeError
	if !errors.As(err, &challengeErr) {
		t.Fatalf("login() error = %v, want TOTPChallengeError", err)
	}
	if challengeErr.ChallengeToken != "challenge-token" || challengeErr.ExpiresAt == nil || *challengeErr.ExpiresAt != expiresAt {
		t.Fatalf("challenge error = %+v", challengeErr)
	}
	if client.token != "" || !client.expiresAt.IsZero() {
		t.Fatalf("credential state = token %q, expires_at %v; want cleared", client.token, client.expiresAt)
	}
}

func TestAdminAPIContractsForFulfillmentStatusAndCardBatch(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		calls = append(calls, r.Method+" "+r.URL.Path)
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		switch r.URL.Path {
		case "/api/v1/admin/fulfillments":
			if body["order_id"] != float64(7) || body["payload"] != "SECRET" {
				t.Errorf("fulfillment body = %#v", body)
			}
		case "/api/v1/admin/orders/7":
			if body["status"] != "completed" {
				t.Errorf("status body = %#v", body)
			}
		case "/api/v1/admin/card-secrets/batch":
			if body["deduplicate"] != true || body["sku_id"] != float64(9) {
				t.Errorf("card batch body = %#v", body)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status_code": 0, "msg": "success", "data": map[string]interface{}{"created": 1}})
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client(), token: "test-token", expiresAt: time.Now().Add(time.Hour)}
	deduplicate := true
	if _, err := client.CreateFulfillment(context.Background(), model.CreateFulfillmentRequest{OrderID: 7, Payload: "SECRET"}); err != nil {
		t.Fatalf("CreateFulfillment() error = %v", err)
	}
	if err := client.UpdateOrderStatus(context.Background(), 7, "completed"); err != nil {
		t.Fatalf("UpdateOrderStatus() error = %v", err)
	}
	if _, err := client.CreateCardSecretBatch(context.Background(), model.CreateCardSecretBatchRequest{ProductID: 3, SKUID: 9, Secrets: []string{"SECRET"}, Deduplicate: &deduplicate}); err != nil {
		t.Fatalf("CreateCardSecretBatch() error = %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("calls = %v, want 3 calls", calls)
	}
}

func TestUpdateOrderStatusPropagatesBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status_code": 409,
			"msg":         "invalid status transition",
			"data":        nil,
		})
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client(), token: "test-token", expiresAt: time.Now().Add(time.Hour)}
	err := client.UpdateOrderStatus(context.Background(), 7, "completed")
	if err == nil || !strings.Contains(err.Error(), "invalid status transition") {
		t.Fatalf("UpdateOrderStatus() error = %v, want business error", err)
	}
}

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/v/telegram-bot-dujiao-next/internal/config"
	"github.com/v/telegram-bot-dujiao-next/internal/model"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.RWMutex
	token      string
	expiresAt  time.Time

	username string
	password string
}

// TOTPChallengeError marks a successful password check that requires a
// separate interactive 2FA verification step. The bot deliberately does not
// collect or generate TOTP codes, so callers can present this boundary clearly.
type TOTPChallengeError struct {
	ChallengeToken string
	ExpiresAt      *string
}

func (e *TOTPChallengeError) Error() string {
	if e.ExpiresAt != nil && *e.ExpiresAt != "" {
		return fmt.Sprintf("账号需要两步验证，请完成 2FA challenge（有效期至 %s）后重试", *e.ExpiresAt)
	}
	return "账号需要两步验证，请完成 2FA challenge 后重试"
}

func NewClient(cfg config.DujiaoConfig) *Client {
	return &Client{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		username: cfg.AdminUsername,
		password: cfg.AdminPassword,
	}
}

func (c *Client) StartRefreshLoop(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.login(ctx); err != nil {
					fmt.Printf("JWT refresh failed: %v\n", err)
				}
			}
		}
	}()
}

func (c *Client) EnsureToken(ctx context.Context) error {
	c.mu.RLock()
	valid := c.token != "" && time.Now().Before(c.expiresAt.Add(-5*time.Minute))
	c.mu.RUnlock()
	if valid {
		return nil
	}
	return c.login(ctx)
}

func (c *Client) login(ctx context.Context) error {
	reqBody := model.LoginRequest{
		Username: c.username,
		Password: c.password,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/admin/login", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// dujiao-next always returns HTTP 200; check business status_code
	var envelope model.Response
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("login: parse response: %w", err)
	}
	if envelope.StatusCode != 0 {
		return fmt.Errorf("login failed: %s", envelope.Msg)
	}

	var loginData model.LoginResponseData
	if err := json.Unmarshal(envelope.Data, &loginData); err != nil {
		return fmt.Errorf("login: parse data: %w", err)
	}

	if loginData.RequiresTOTP {
		// A challenge is not an authenticated session. Clear any stale token so
		// a refresh attempt cannot leave callers using the previous credential.
		c.mu.Lock()
		c.token = ""
		c.expiresAt = time.Time{}
		c.mu.Unlock()
		return &TOTPChallengeError{
			ChallengeToken: loginData.ChallengeToken,
			ExpiresAt:      loginData.ChallengeExpiresAt,
		}
	}
	if loginData.Token == "" {
		return fmt.Errorf("login: 响应中没有 token")
	}

	c.mu.Lock()
	c.token = loginData.Token
	if loginData.ExpiresAt != nil {
		if t, err := time.Parse(time.RFC3339, *loginData.ExpiresAt); err == nil {
			c.expiresAt = t
		} else {
			c.expiresAt = time.Now().Add(24 * time.Hour)
		}
	} else {
		c.expiresAt = time.Now().Add(24 * time.Hour)
	}
	c.mu.Unlock()

	return nil
}

// doRequest makes an authenticated request, unwraps the response envelope,
// and returns the "data" payload on success.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (json.RawMessage, error) {
	if err := c.EnsureToken(ctx); err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var envelope model.Response
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if envelope.StatusCode != 0 {
		return nil, fmt.Errorf("API 错误 %d: %s", envelope.StatusCode, envelope.Msg)
	}

	return envelope.Data, nil
}

// doPageRequest makes a paginated authenticated request.
func (c *Client) doPageRequest(ctx context.Context, method, path string) (json.RawMessage, model.Pagination, error) {
	if err := c.EnsureToken(ctx); err != nil {
		return nil, model.Pagination{}, fmt.Errorf("auth: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, model.Pagination{}, err
	}

	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, model.Pagination{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, model.Pagination{}, err
	}

	var pageResp model.PageResponse
	if err := json.Unmarshal(respBody, &pageResp); err != nil {
		return nil, model.Pagination{}, fmt.Errorf("parse response: %w", err)
	}
	if pageResp.StatusCode != 0 {
		return nil, model.Pagination{}, fmt.Errorf("API 错误 %d: %s", pageResp.StatusCode, pageResp.Msg)
	}

	return pageResp.Data, pageResp.Pagination, nil
}

// --- Admin API Methods ---

func (c *Client) ListOrders(ctx context.Context, status string, page, pageSize int) ([]model.Order, model.Pagination, error) {
	return c.listOrders(ctx, status, page, pageSize, "", "")
}

func (c *Client) listOrders(ctx context.Context, status string, page, pageSize int, sortBy, sortOrder string) ([]model.Order, model.Pagination, error) {
	values := url.Values{}
	values.Set("page", fmt.Sprintf("%d", page))
	values.Set("page_size", fmt.Sprintf("%d", pageSize))
	if status != "" {
		values.Set("status", status)
	}
	if sortBy != "" {
		values.Set("sort_by", sortBy)
	}
	if sortOrder != "" {
		values.Set("sort_order", sortOrder)
	}

	path := "/api/v1/admin/orders?" + values.Encode()
	data, pagination, err := c.doPageRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, model.Pagination{}, err
	}
	var orders []model.Order
	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, model.Pagination{}, err
	}
	return orders, pagination, nil
}

// ListRecentOrders fetches the newest admin orders by update time.
//
// Payment callbacks and manual payment recovery can update older pending
// orders, so paid-order notification polling should not rely on creation order.
func (c *Client) ListRecentOrders(ctx context.Context, pageSize, maxPages int) ([]model.Order, error) {
	if pageSize <= 0 {
		pageSize = 100
	}
	if maxPages <= 0 {
		maxPages = 1
	}

	var allOrders []model.Order
	for page := 1; page <= maxPages; page++ {
		orders, pagination, err := c.listOrders(ctx, "", page, pageSize, "updated_at", "desc")
		if err != nil {
			return nil, err
		}
		allOrders = append(allOrders, orders...)
		if len(orders) == 0 ||
			(pagination.Total > 0 && len(allOrders) >= int(pagination.Total)) ||
			(pagination.TotalPage > 0 && int64(page) >= pagination.TotalPage) {
			break
		}
	}
	return allOrders, nil
}

// ListFulfillingOrders fetches orders that need manual fulfillment.
// It queries all statuses that can still receive manual fulfillment.
func (c *Client) ListFulfillingOrders(ctx context.Context) ([]model.Order, error) {
	var allOrders []model.Order
	for _, status := range []string{"paid", "fulfilling", "partially_delivered"} {
		statusOrderCount := 0
		for page := 1; ; page++ {
			orders, pagination, err := c.ListOrders(ctx, status, page, 100)
			if err != nil {
				return nil, fmt.Errorf("list %s orders page %d: %w", status, page, err)
			}
			allOrders = append(allOrders, orders...)
			statusOrderCount += len(orders)
			if len(orders) == 0 ||
				(pagination.Total > 0 && int64(statusOrderCount) >= pagination.Total) ||
				(pagination.TotalPage > 0 && int64(page) >= pagination.TotalPage) ||
				(pagination.TotalPage == 0 && len(orders) < 100) {
				break
			}
		}
	}
	return allOrders, nil
}

func (c *Client) CreateFulfillment(ctx context.Context, req model.CreateFulfillmentRequest) (*model.FulfillmentResponse, error) {
	data, err := c.doRequest(ctx, http.MethodPost, "/api/v1/admin/fulfillments", req)
	if err != nil {
		return nil, err
	}
	var result model.FulfillmentResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateOrderStatus(ctx context.Context, orderID uint, status string) error {
	_, err := c.doRequest(ctx, http.MethodPatch, fmt.Sprintf("/api/v1/admin/orders/%d", orderID), map[string]string{
		"status": status,
	})
	return err
}

func (c *Client) GetOrder(ctx context.Context, orderID uint) (*model.Order, error) {
	data, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/admin/orders/%d", orderID), nil)
	if err != nil {
		return nil, err
	}
	var order model.Order
	if err := json.Unmarshal(data, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (c *Client) CreateCardSecretBatch(ctx context.Context, req model.CreateCardSecretBatchRequest) (*model.CreateCardSecretBatchResponse, error) {
	data, err := c.doRequest(ctx, http.MethodPost, "/api/v1/admin/card-secrets/batch", req)
	if err != nil {
		return nil, err
	}
	var result model.CreateCardSecretBatchResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListProducts(ctx context.Context, page, pageSize int) ([]model.Product, model.Pagination, error) {
	path := fmt.Sprintf("/api/v1/admin/products?page=%d&page_size=%d", page, pageSize)
	data, pagination, err := c.doPageRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, model.Pagination{}, err
	}
	var products []model.Product
	if err := json.Unmarshal(data, &products); err != nil {
		return nil, model.Pagination{}, err
	}
	return products, pagination, nil
}

func (c *Client) GetDashboardOverview(ctx context.Context, query string) (*model.DashboardOverview, error) {
	path := "/api/v1/admin/dashboard/overview"
	if query != "" {
		path += "?" + query
	}
	data, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var overview model.DashboardOverview
	if err := json.Unmarshal(data, &overview); err != nil {
		return nil, err
	}
	return &overview, nil
}

func (c *Client) GetInventoryAlerts(ctx context.Context) ([]model.InventoryAlert, error) {
	data, err := c.doRequest(ctx, http.MethodGet, "/api/v1/admin/dashboard/inventory-alerts", nil)
	if err != nil {
		return nil, err
	}
	var alerts []model.InventoryAlert
	if err := json.Unmarshal(data, &alerts); err != nil {
		return nil, err
	}
	return alerts, nil
}

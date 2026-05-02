package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		return fmt.Errorf("账号启用了两步验证，Bot 暂不支持")
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
	path := fmt.Sprintf("/api/v1/admin/orders?page=%d&page_size=%d", page, pageSize)
	if status != "" {
		path += "&status=" + status
	}
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

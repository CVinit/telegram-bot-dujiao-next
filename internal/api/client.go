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
	if c.token != "" && time.Now().Before(c.expiresAt.Add(-5*time.Minute)) {
		c.mu.RUnlock()
		return nil
	}
	c.mu.RUnlock()
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

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed: status %d, body: %s", resp.StatusCode, respBody)
	}

	var loginResp model.LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return err
	}

	c.mu.Lock()
	c.token = loginResp.Token
	c.expiresAt = time.Now().Add(24 * time.Hour)
	c.mu.Unlock()

	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, int, error) {
	if err := c.EnsureToken(ctx); err != nil {
		return nil, 0, fmt.Errorf("auth: %w", err)
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, 0, err
	}

	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return respBody, resp.StatusCode, nil
}

// Admin API methods

func (c *Client) ListOrders(ctx context.Context, orderStatus string, page, pageSize int) (*model.OrderListResponse, error) {
	path := fmt.Sprintf("/api/v1/admin/orders?page=%d&page_size=%d", page, pageSize)
	if orderStatus != "" {
		path += "&status=" + orderStatus
	}
	body, code, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("list orders failed: status %d, body: %s", code, body)
	}
	var result model.OrderListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateFulfillment(ctx context.Context, req model.CreateFulfillmentRequest) error {
	body, code, err := c.doRequest(ctx, http.MethodPost, "/api/v1/admin/fulfillments", req)
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return fmt.Errorf("create fulfillment failed: status %d, body: %s", code, body)
	}
	return nil
}

func (c *Client) AddCardSecrets(ctx context.Context, req model.AddCardSecretsRequest) (*model.AddCardSecretsResponse, error) {
	body, code, err := c.doRequest(ctx, http.MethodPost, "/api/v1/admin/card-secrets", req)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return nil, fmt.Errorf("add card secrets failed: status %d, body: %s", code, body)
	}
	var result model.AddCardSecretsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListProducts(ctx context.Context, page, pageSize int) (*model.ProductListResponse, error) {
	path := fmt.Sprintf("/api/v1/admin/products?page=%d&page_size=%d", page, pageSize)
	body, code, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("list products failed: status %d, body: %s", code, body)
	}
	var result model.ProductListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetSalesStats(ctx context.Context, period string) (*model.SalesStats, error) {
	path := fmt.Sprintf("/api/v1/admin/orders/stats?period=%s", period)
	body, code, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("get sales stats failed: status %d, body: %s", code, body)
	}
	var result model.SalesStats
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

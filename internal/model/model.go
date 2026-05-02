package model

import "encoding/json"

// Response envelope

type Response struct {
	StatusCode int             `json:"status_code"`
	Msg        string          `json:"msg"`
	Data       json.RawMessage `json:"data"`
}

type PageResponse struct {
	StatusCode int             `json:"status_code"`
	Msg        string          `json:"msg"`
	Data       json.RawMessage `json:"data"`
	Pagination Pagination      `json:"pagination"`
}

type Pagination struct {
	Page      int   `json:"page"`
	PageSize  int   `json:"page_size"`
	Total     int64 `json:"total"`
	TotalPage int64 `json:"total_page"`
}

// Admin login

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponseData struct {
	RequiresTOTP    bool       `json:"requires_totp"`
	Token           string     `json:"token"`
	ExpiresAt       *string    `json:"expires_at"`
	ChallengeToken  string     `json:"challenge_token,omitempty"`
	User            *AdminUser `json:"user,omitempty"`
}

type AdminUser struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

// Order

type Order struct {
	ID                   uint           `json:"id"`
	OrderNo              string         `json:"order_no"`
	ParentID             *uint          `json:"parent_id"`
	UserID               uint           `json:"user_id"`
	Status               string         `json:"status"`
	Currency             string         `json:"currency"`
	OriginalAmount       string         `json:"original_amount"`
	DiscountAmount       string         `json:"discount_amount"`
	TotalAmount          string         `json:"total_amount"`
	WalletPaidAmount     string         `json:"wallet_paid_amount"`
	OnlinePaidAmount     string         `json:"online_paid_amount"`
	RefundedAmount       string         `json:"refunded_amount"`
	PaidAt               *string        `json:"paid_at"`
	CanceledAt           *string        `json:"canceled_at"`
	CreatedAt            string         `json:"created_at"`
	UpdatedAt            string         `json:"updated_at"`
	Items                []OrderItem    `json:"items,omitempty"`
	Children             []Order        `json:"children,omitempty"`
	UserEmail            string         `json:"user_email,omitempty"`
	UserDisplayName      string         `json:"user_display_name,omitempty"`
	GuestEmail           string         `json:"guest_email,omitempty"`
}

type OrderItem struct {
	ID              uint   `json:"id"`
	OrderID         uint   `json:"order_id"`
	ProductID       uint   `json:"product_id"`
	SKUID           uint   `json:"sku_id"`
	Quantity        int    `json:"quantity"`
	UnitPrice       string `json:"unit_price"`
	Subtotal        string `json:"subtotal"`
	FulfillmentType string `json:"fulfillment_type"`
	Title           interface{} `json:"title"` // i18n map or string
	SKUName         string `json:"sku_name,omitempty"`
	CardSecrets     string `json:"card_secrets,omitempty"`
}

// Fulfillment

type CreateFulfillmentRequest struct {
	OrderID      uint          `json:"order_id"`
	Payload      string        `json:"payload"`
	DeliveryData interface{}   `json:"delivery_data,omitempty"`
}

type FulfillmentResponse struct {
	ID               uint       `json:"id"`
	OrderID          uint       `json:"order_id"`
	Type             string     `json:"type"`
	Status           string     `json:"status"`
	Payload          string     `json:"payload"`
	PayloadLineCount int        `json:"payload_line_count"`
	DeliveryData     interface{} `json:"delivery_data"`
	DeliveredBy      *uint      `json:"delivered_by,omitempty"`
	DeliveredAt      *string    `json:"delivered_at,omitempty"`
	CreatedAt        string     `json:"created_at"`
}

// Card secret batch

type CreateCardSecretBatchRequest struct {
	ProductID uint     `json:"product_id"`
	SKUID     uint     `json:"sku_id"`
	Secrets   []string `json:"secrets"`
	BatchNo   string   `json:"batch_no,omitempty"`
	Note      string   `json:"note,omitempty"`
}

type CreateCardSecretBatchResponse struct {
	Created  int    `json:"created"`
	BatchID  uint   `json:"batch_id"`
	BatchNo  string `json:"batch_no"`
}

// Product

type Product struct {
	ID                  uint        `json:"id"`
	CategoryID          uint        `json:"category_id"`
	Slug                string      `json:"slug"`
	Title               interface{} `json:"title"` // i18n map: {"en": "...", "zh": "..."}
	Description         interface{} `json:"description"`
	PriceAmount         string      `json:"price_amount"`
	CostPriceAmount     string      `json:"cost_price_amount"`
	FulfillmentType     string      `json:"fulfillment_type"`
	ManualStockTotal    int         `json:"manual_stock_total"`
	ManualStockLocked   int         `json:"manual_stock_locked"`
	ManualStockSold     int         `json:"manual_stock_sold"`
	AutoStockAvailable  int         `json:"auto_stock_available"`
	AutoStockTotal      int         `json:"auto_stock_total"`
	AutoStockLocked     int         `json:"auto_stock_locked"`
	AutoStockSold       int         `json:"auto_stock_sold"`
	IsActive            bool        `json:"is_active"`
	SKUs                []SKU       `json:"skus,omitempty"`
	CreatedAt           string      `json:"created_at"`
	UpdatedAt           string      `json:"updated_at"`
}

type SKU struct {
	ID              uint   `json:"id"`
	SKUCode         string `json:"sku_code"`
	Name            interface{} `json:"name"` // i18n map
	PriceAmount     string `json:"price_amount"`
	FulfillmentType string `json:"fulfillment_type"`
	StockCount      int    `json:"stock_count,omitempty"`
}

// Dashboard

type DashboardOverview struct {
	TotalRevenue     string                 `json:"total_revenue"`
	TotalOrders      int64                  `json:"total_orders"`
	TotalUsers       int64                  `json:"total_users"`
	RevenueChange    float64                `json:"revenue_change"`
	OrdersChange     float64                `json:"orders_change"`
	UsersChange      float64                `json:"users_change"`
	RecentOrders     []Order                `json:"recent_orders,omitempty"`
	TopProducts      []TopProduct           `json:"top_products,omitempty"`
	InventoryAlerts  []InventoryAlert       `json:"inventory_alerts,omitempty"`
}

type TopProduct struct {
	ProductID   uint   `json:"product_id"`
	Title       interface{} `json:"title"`
	SoldCount   int64  `json:"sold_count"`
	Revenue     string `json:"revenue"`
}

type InventoryAlert struct {
	ProductID       uint        `json:"product_id"`
	ProductTitle    interface{} `json:"product_title"`
	SKUID           uint        `json:"sku_id"`
	SKUName         interface{} `json:"sku_name"`
	AvailableStock  int         `json:"available_stock"`
	TotalStock      int         `json:"total_stock"`
}

type DashboardTrends struct {
	Data []TrendPoint `json:"data"`
}

type TrendPoint struct {
	Date    string `json:"date"`
	Revenue string `json:"revenue"`
	Orders  int64  `json:"orders"`
}

// Helper to extract product display name from i18n title
func GetProductName(title interface{}) string {
	switch v := title.(type) {
	case string:
		return v
	case map[string]interface{}:
		for _, key := range []string{"zh", "zh-CN", "zh-TW", "en", "en-US"} {
			if name, ok := v[key].(string); ok && name != "" {
				return name
			}
		}
		for _, val := range v {
			if name, ok := val.(string); ok && name != "" {
				return name
			}
		}
	}
	return "未知商品"
}

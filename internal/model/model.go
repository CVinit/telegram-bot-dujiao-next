package model

// Admin login

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

// Order

type Order struct {
	ID              uint          `json:"id"`
	OrderNo         string        `json:"order_no"`
	ParentID        *uint         `json:"parent_id"`
	Status          string        `json:"status"`
	FulfillmentType string        `json:"fulfillment_type"`
	Quantity        int           `json:"quantity"`
	Amount          string        `json:"amount"`
	ProductName     string        `json:"product_name"`
	CreatedAt       string        `json:"created_at"`
	Children        []Order       `json:"children,omitempty"`
	Items           []OrderItem   `json:"items,omitempty"`
}

type OrderItem struct {
	ID              uint   `json:"id"`
	ProductID       uint   `json:"product_id"`
	ProductName     string `json:"product_name"`
	SKUID           uint   `json:"sku_id"`
	SKUName         string `json:"sku_name"`
	Quantity        int    `json:"quantity"`
	FulfillmentType string `json:"fulfillment_type"`
	CardSecrets     string `json:"card_secrets,omitempty"`
}

type OrderListResponse struct {
	Items []Order `json:"items"`
	Total int64   `json:"total"`
}

// Fulfillment

type CreateFulfillmentRequest struct {
	OrderID      uint              `json:"order_id"`
	Payload      string            `json:"payload"`
	DeliveryData *DeliveryData     `json:"delivery_data,omitempty"`
}

type DeliveryData struct {
	Note    string           `json:"note,omitempty"`
	Entries []KeyValueEntry `json:"entries,omitempty"`
	Extra   []KeyValueEntry `json:"extra,omitempty"`
}

type KeyValueEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Card secret

type CardSecret struct {
	ID        uint   `json:"id"`
	ProductID uint   `json:"product_id"`
	SKUID     uint   `json:"sku_id"`
	Secret    string `json:"secret"`
	Status    string `json:"status"`
	BatchID   string `json:"batch_id"`
	CreatedAt string `json:"created_at"`
}

type AddCardSecretsRequest struct {
	ProductID uint     `json:"product_id"`
	SKUID     uint     `json:"sku_id"`
	Secrets   []string `json:"secrets"`
	BatchID   string   `json:"batch_id,omitempty"`
}

type AddCardSecretsResponse struct {
	Count int `json:"count"`
}

// Product

type Product struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      int    `json:"status"`
	CreatedAt   string `json:"created_at"`
	SKUs        []SKU  `json:"skus,omitempty"`
}

type SKU struct {
	ID              uint   `json:"id"`
	Name            string `json:"name"`
	Price           string `json:"price"`
	FulfillmentType string `json:"fulfillment_type"`
	StockCount      int    `json:"stock_count"`
}

type ProductListResponse struct {
	Items []Product `json:"items"`
	Total int64     `json:"total"`
}

// Sales stats

type SalesStats struct {
	OrderCount int64  `json:"order_count"`
	TotalAmount string `json:"total_amount"`
}
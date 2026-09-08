package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
	RequiresTOTP       bool       `json:"requires_totp"`
	Token              string     `json:"token"`
	ExpiresAt          *string    `json:"expires_at"`
	ChallengeToken     string     `json:"challenge_token"`
	ChallengeExpiresAt *string    `json:"challenge_expires_at"`
	User               *AdminUser `json:"user,omitempty"`
}

type AdminUser struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

// Order

type Order struct {
	ID                       uint                 `json:"id"`
	ParentID                 *uint                `json:"parent_id"`
	OrderNo                  string               `json:"order_no"`
	Status                   string               `json:"status"`
	Currency                 string               `json:"currency"`
	OriginalAmount           string               `json:"original_amount"`
	DiscountAmount           string               `json:"discount_amount"`
	MemberDiscountAmount     string               `json:"member_discount_amount"`
	PromotionDiscountAmount  string               `json:"promotion_discount_amount"`
	WholesaleDiscountAmount  string               `json:"wholesale_discount_amount"`
	TotalAmount              string               `json:"total_amount"`
	WalletPaidAmount         string               `json:"wallet_paid_amount"`
	OnlinePaidAmount         string               `json:"online_paid_amount"`
	RefundedAmount           string               `json:"refunded_amount"`
	GuestEmail               string               `json:"guest_email"`
	GuestLocale              string               `json:"guest_locale"`
	ClientIP                 string               `json:"client_ip"`
	ExpiresAt                *string              `json:"expires_at"`
	PaidAt                   *string              `json:"paid_at"`
	CanceledAt               *string              `json:"canceled_at"`
	CreatedAt                string               `json:"created_at"`
	UpdatedAt                string               `json:"updated_at"`
	Items                    []OrderItem          `json:"items"`
	Fulfillment              *FulfillmentResponse `json:"fulfillment"`
	RefundRecords            []RefundRecord       `json:"refund_records"`
	AllowedPaymentChannelIDs []uint               `json:"allowed_payment_channel_ids"`
	Children                 []Order              `json:"children"`
}

type OrderItem struct {
	ID                       uint        `json:"id"`
	OrderID                  uint        `json:"order_id"`
	ProductID                uint        `json:"product_id"`
	SKUID                    uint        `json:"sku_id"`
	Title                    interface{} `json:"title"`
	SKUSnapshot              interface{} `json:"sku_snapshot"`
	Tags                     interface{} `json:"tags"`
	UnitPrice                string      `json:"unit_price"`
	OriginalUnitPrice        string      `json:"original_unit_price"`
	CostPrice                string      `json:"cost_price"`
	Quantity                 int         `json:"quantity"`
	TotalPrice               string      `json:"total_price"`
	OriginalTotalPrice       string      `json:"original_total_price"`
	CouponDiscountAmount     string      `json:"coupon_discount_amount"`
	MemberDiscountAmount     string      `json:"member_discount_amount"`
	PromotionDiscountAmount  string      `json:"promotion_discount_amount"`
	WholesaleDiscountAmount  string      `json:"wholesale_discount_amount"`
	PromotionID              *uint       `json:"promotion_id"`
	PromotionName            interface{} `json:"promotion_name"`
	FulfillmentType          string      `json:"fulfillment_type"`
	ManualFormSchemaSnapshot interface{} `json:"manual_form_schema_snapshot"`
	ManualFormSubmission     interface{} `json:"manual_form_submission"`
	Instructions             interface{} `json:"instructions"`
	CreatedAt                string      `json:"created_at"`
	UpdatedAt                string      `json:"updated_at"`
}

// Fulfillment

type CreateFulfillmentRequest struct {
	OrderID      uint        `json:"order_id"`
	Payload      string      `json:"payload"`
	DeliveryData interface{} `json:"delivery_data,omitempty"`
}

type FulfillmentResponse struct {
	ID               uint        `json:"id"`
	OrderID          uint        `json:"order_id"`
	Type             string      `json:"type"`
	Status           string      `json:"status"`
	Payload          string      `json:"payload"`
	PayloadLineCount int         `json:"payload_line_count"`
	DeliveryData     interface{} `json:"delivery_data"`
	DeliveredBy      *uint       `json:"delivered_by,omitempty"`
	DeliveredAt      *string     `json:"delivered_at,omitempty"`
	CreatedAt        string      `json:"created_at"`
}

type RefundRecord struct {
	ID        uint   `json:"id"`
	OrderID   uint   `json:"order_id"`
	UserID    uint   `json:"user_id"`
	Type      string `json:"type"`
	Amount    string `json:"amount"`
	Currency  string `json:"currency"`
	Remark    string `json:"remark"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Card secret batch

type CreateCardSecretBatchRequest struct {
	ProductID   uint     `json:"product_id"`
	SKUID       uint     `json:"sku_id"`
	Secrets     []string `json:"secrets"`
	BatchNo     string   `json:"batch_no,omitempty"`
	Note        string   `json:"note,omitempty"`
	Deduplicate *bool    `json:"deduplicate,omitempty"`
}

type CreateCardSecretBatchResponse struct {
	Created int    `json:"created"`
	BatchID uint   `json:"batch_id"`
	BatchNo string `json:"batch_no"`
}

// Product

type Product struct {
	ID                   uint               `json:"id"`
	CategoryID           uint               `json:"category_id"`
	Slug                 string             `json:"slug"`
	SEOMeta              interface{}        `json:"seo_meta"`
	Title                interface{}        `json:"title"`
	Description          interface{}        `json:"description"`
	Content              interface{}        `json:"content"`
	Instructions         interface{}        `json:"instructions"`
	PriceAmount          string             `json:"price_amount"`
	WholesalePrices      []WholesalePrice   `json:"wholesale_prices,omitempty"`
	Images               []string           `json:"images"`
	Tags                 []string           `json:"tags"`
	PurchaseType         string             `json:"purchase_type"`
	MinPurchaseQuantity  int                `json:"min_purchase_quantity"`
	MaxPurchaseQuantity  int                `json:"max_purchase_quantity"`
	CostPriceAmount      string             `json:"cost_price_amount"`
	FulfillmentType      string             `json:"fulfillment_type"`
	ManualFormSchema     interface{}        `json:"manual_form_schema"`
	ManualStockAvailable int                `json:"manual_stock_available"`
	ManualStockTotal     int                `json:"manual_stock_total"`
	ManualStockLocked    int                `json:"manual_stock_locked"`
	ManualStockSold      int                `json:"manual_stock_sold"`
	AutoStockAvailable   int64              `json:"auto_stock_available"`
	AutoStockTotal       int                `json:"auto_stock_total"`
	AutoStockLocked      int                `json:"auto_stock_locked"`
	AutoStockSold        int                `json:"auto_stock_sold"`
	IsActive             *bool              `json:"is_active"`
	StockStatus          string             `json:"stock_status"`
	IsSoldOut            *bool              `json:"is_sold_out"`
	StockDisplayMode     string             `json:"stock_display_mode"`
	StockDisplay         interface{}        `json:"stock_display"`
	StockRangeMin        *int               `json:"stock_range_min"`
	StockRangeMax        *int               `json:"stock_range_max"`
	StockQuantityHidden  *bool              `json:"stock_quantity_hidden"`
	PaymentChannelIDs    []uint             `json:"payment_channel_ids"`
	PromotionID          *uint              `json:"promotion_id,omitempty"`
	PromotionName        string             `json:"promotion_name,omitempty"`
	PromotionType        string             `json:"promotion_type,omitempty"`
	PromotionPriceAmount *string            `json:"promotion_price_amount,omitempty"`
	MemberPrices         []MemberLevelPrice `json:"member_prices,omitempty"`
	SKUs                 []SKU              `json:"skus,omitempty"`
	CreatedAt            string             `json:"created_at"`
	UpdatedAt            string             `json:"updated_at"`
}

type SKU struct {
	ID                   uint        `json:"id"`
	ProductID            uint        `json:"product_id"`
	SKUCode              string      `json:"sku_code"`
	SpecValues           interface{} `json:"spec_values"`
	PriceAmount          string      `json:"price_amount"`
	CostPriceAmount      string      `json:"cost_price_amount"`
	ManualStockTotal     int         `json:"manual_stock_total"`
	ManualStockLocked    int         `json:"manual_stock_locked"`
	ManualStockSold      int         `json:"manual_stock_sold"`
	AutoStockAvailable   int64       `json:"auto_stock_available"`
	AutoStockTotal       int         `json:"auto_stock_total"`
	AutoStockLocked      int         `json:"auto_stock_locked"`
	AutoStockSold        int         `json:"auto_stock_sold"`
	UpstreamStock        int         `json:"upstream_stock"`
	IsActive             *bool       `json:"is_active"`
	StockStatus          string      `json:"stock_status"`
	IsSoldOut            *bool       `json:"is_sold_out"`
	StockDisplayMode     string      `json:"stock_display_mode"`
	StockDisplay         interface{} `json:"stock_display"`
	StockRangeMin        *int        `json:"stock_range_min"`
	StockRangeMax        *int        `json:"stock_range_max"`
	StockQuantityHidden  *bool       `json:"stock_quantity_hidden"`
	PromotionPriceAmount *string     `json:"promotion_price_amount,omitempty"`
	MemberPriceAmount    *string     `json:"member_price_amount,omitempty"`
	CreatedAt            string      `json:"created_at"`
	UpdatedAt            string      `json:"updated_at"`
}

type WholesalePrice struct {
	SKUID       uint   `json:"sku_id,omitempty"`
	SKUCode     string `json:"sku_code,omitempty"`
	MinQuantity int    `json:"min_quantity"`
	UnitPrice   string `json:"unit_price"`
}

type MemberLevelPrice struct {
	MemberLevelID uint   `json:"member_level_id"`
	SKUID         uint   `json:"sku_id"`
	PriceAmount   string `json:"price_amount"`
}

// Dashboard overview — matches actual API response structure.

type DashboardOverview struct {
	Range    string           `json:"range"`
	From     string           `json:"from"`
	To       string           `json:"to"`
	Timezone string           `json:"timezone"`
	Currency string           `json:"currency"`
	KPI      DashboardKPI     `json:"kpi"`
	Funnel   DashboardFunnel  `json:"funnel"`
	Alerts   []DashboardAlert `json:"alerts"`
}

type DashboardKPI struct {
	OrdersTotal          int    `json:"orders_total"`
	PaidOrders           int    `json:"paid_orders"`
	CompletedOrders      int    `json:"completed_orders"`
	PendingPaymentOrders int    `json:"pending_payment_orders"`
	GMVPaid              string `json:"gmv_paid"`
	TotalCost            string `json:"total_cost"`
	TotalProfit          string `json:"total_profit"`
	ProfitMargin         string `json:"profit_margin"`
	PaymentsTotal        int    `json:"payments_total"`
	PaymentsSuccess      int    `json:"payments_success"`
	PaymentsFailed       int    `json:"payments_failed"`
	PaymentSuccessRate   string `json:"payment_success_rate"`
	NewUsers             int    `json:"new_users"`
	ActiveProducts       int    `json:"active_products"`
	OutOfStockProducts   int    `json:"out_of_stock_products"`
	LowStockProducts     int    `json:"low_stock_products"`
	OutOfStockSKUs       int    `json:"out_of_stock_skus"`
	LowStockSKUs         int    `json:"low_stock_skus"`
	AutoAvailableSecrets int    `json:"auto_available_secrets"`
	ManualAvailableUnits int    `json:"manual_available_units"`
	TotalUserBalance     string `json:"total_user_balance"`
}

type DashboardFunnel struct {
	OrdersCreated         int    `json:"orders_created"`
	PaymentsCreated       int    `json:"payments_created"`
	PaymentsSuccess       int    `json:"payments_success"`
	OrdersPaid            int    `json:"orders_paid"`
	OrdersCompleted       int    `json:"orders_completed"`
	PaymentConversionRate string `json:"payment_conversion_rate"`
	CompletionRate        string `json:"completion_rate"`
}

type DashboardAlert struct {
	Type  string `json:"type"`
	Level string `json:"level"`
	Value int    `json:"value"`
}

// Inventory alert from dashboard/inventory-alerts endpoint

type InventoryAlert struct {
	ProductID           uint        `json:"product_id"`
	ProductTitle        interface{} `json:"product_title"`
	SKUID               uint        `json:"sku_id"`
	SKUCode             string      `json:"sku_code"`
	SKUName             interface{} `json:"sku_name"`
	SKUSpecValues       interface{} `json:"sku_spec_values"`
	AvailableStock      int         `json:"available_stock"`
	TotalStock          int         `json:"total_stock"`
	StockStatus         string      `json:"stock_status"`
	IsSoldOut           *bool       `json:"is_sold_out"`
	StockDisplay        interface{} `json:"stock_display"`
	StockRangeMin       *int        `json:"stock_range_min"`
	StockRangeMax       *int        `json:"stock_range_max"`
	StockQuantityHidden *bool       `json:"stock_quantity_hidden"`
}

func IsFulfillableOrderStatus(status string) bool {
	switch status {
	case "paid", "fulfilling":
		return true
	default:
		return false
	}
}

// Dashboard trends

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
	if name := localizedText(title); name != "" {
		return name
	}
	return "未知商品"
}

func GetSKUName(snapshot interface{}) string {
	data := objectMap(snapshot)
	if len(data) == 0 {
		return ""
	}
	if name := localizedText(data["spec_values"]); name != "" {
		return name
	}
	code := cleanText(data["sku_code"])
	if code == "" {
		return ""
	}
	if code == "DEFAULT" {
		return "默认规格"
	}
	return code
}

func GetProductSKUName(sku SKU) string {
	return GetSKUName(map[string]interface{}{
		"sku_code":    sku.SKUCode,
		"spec_values": sku.SpecValues,
	})
}

func GetProductSKUDisplayName(productTitle interface{}, sku SKU) string {
	productName := GetProductName(productTitle)
	skuName := GetProductSKUName(sku)
	if skuName == "" {
		return productName
	}
	return productName + " / " + skuName
}

func GetInventoryAlertSKUName(alert InventoryAlert) string {
	if name := localizedText(alert.SKUSpecValues); name != "" {
		return name
	}
	if name := localizedText(alert.SKUName); name != "" {
		return name
	}
	code := cleanString(alert.SKUCode)
	if code == "" {
		return "默认规格"
	}
	if code == "DEFAULT" {
		return "默认规格"
	}
	return code
}

func GetOrderItemDisplayName(item OrderItem) string {
	productName := GetProductName(item.Title)
	skuName := GetSKUName(item.SKUSnapshot)
	if skuName == "" {
		return productName
	}
	return productName + " / " + skuName
}

func GetOrderItemGroupKey(item OrderItem) string {
	if item.ProductID != 0 || item.SKUID != 0 {
		return fmt.Sprintf("%d:%d", item.ProductID, item.SKUID)
	}
	return GetOrderItemDisplayName(item)
}

func localizedText(value interface{}) string {
	switch v := value.(type) {
	case string:
		return cleanString(v)
	case map[string]interface{}:
		for _, key := range []string{"zh-CN", "zh", "zh-TW", "en-US", "en"} {
			if name := cleanText(v[key]); name != "" {
				return name
			}
		}
		for _, val := range v {
			if name := cleanText(val); name != "" {
				return name
			}
		}
	case map[string]string:
		for _, key := range []string{"zh-CN", "zh", "zh-TW", "en-US", "en"} {
			if name := cleanString(v[key]); name != "" {
				return name
			}
		}
		for _, val := range v {
			if name := cleanString(val); name != "" {
				return name
			}
		}
	case json.RawMessage:
		return localizedTextFromJSON(v)
	case []byte:
		return localizedTextFromJSON(v)
	}
	return ""
}

func localizedTextFromJSON(raw []byte) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return cleanString(text)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err == nil {
		return localizedText(data)
	}
	return ""
}

func objectMap(value interface{}) map[string]interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return v
	case map[string]string:
		data := make(map[string]interface{}, len(v))
		for key, val := range v {
			data[key] = val
		}
		return data
	case json.RawMessage:
		return objectMapFromJSON(v)
	case []byte:
		return objectMapFromJSON(v)
	}
	return nil
}

func objectMapFromJSON(raw []byte) map[string]interface{} {
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil
	}
	return data
}

func cleanText(value interface{}) string {
	if value == nil {
		return ""
	}
	return cleanString(fmt.Sprintf("%v", value))
}

func cleanString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" {
		return ""
	}
	return value
}

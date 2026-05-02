package handler

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/v/telegram-bot-dujiao-next/internal/api"
	"github.com/v/telegram-bot-dujiao-next/internal/config"
	"github.com/v/telegram-bot-dujiao-next/internal/model"
	"github.com/v/telegram-bot-dujiao-next/internal/state"
)

type Handler struct {
	api   *api.Client
	state *state.Manager
	cfg   *config.Config
}

func New(apiClient *api.Client, stateMgr *state.Manager, cfg *config.Config) *Handler {
	return &Handler{
		api:   apiClient,
		state: stateMgr,
		cfg:   cfg,
	}
}

// /start
func (h *Handler) OnStart(c tele.Context) error {
	return c.Reply(
		"独角数卡管理 Bot\n\n" +
			"/sales - 查看销量\n" +
			"/orders - 待处理订单\n" +
			"/cards - 补充卡密\n" +
			"/fulfill - 批量发货\n" +
			"/stock - 库存概况\n" +
			"/cancel - 取消当前操作",
	)
}

// /sales
func (h *Handler) OnSales(c tele.Context) error {
	selector := &tele.ReplyMarkup{}
	selector.Inline(
		selector.Row(
			selector.Data("今天", "sales", "today"),
			selector.Data("昨天", "sales", "yesterday"),
		),
		selector.Row(
			selector.Data("本周", "sales", "week"),
			selector.Data("本月", "sales", "month"),
		),
	)
	return c.Reply("选择时间维度：", selector)
}

// /orders
func (h *Handler) OnOrders(c tele.Context) error {
	ctx := context.Background()
	resp, err := h.api.ListOrders(ctx, "fulfilling", 1, 50)
	if err != nil {
		return c.Reply(fmt.Sprintf("查询失败：%v", err))
	}

	if len(resp.Items) == 0 {
		return c.Reply("没有待处理的订单")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("待处理订单（共 %d 个）：\n\n", len(resp.Items)))
	for _, o := range resp.Items {
		sb.WriteString(fmt.Sprintf("📦 %s\n", o.OrderNo))
		sb.WriteString(fmt.Sprintf("   商品：%s | 数量：%d | 金额：%s\n", o.ProductName, o.Quantity, o.Amount))
		sb.WriteString(fmt.Sprintf("   时间：%s\n\n", o.CreatedAt))
	}
	return c.Reply(sb.String())
}

// /cards
func (h *Handler) OnCards(c tele.Context) error {
	ctx := context.Background()
	products, err := h.loadAllProducts(ctx)
	if err != nil {
		return c.Reply(fmt.Sprintf("查询商品失败：%v", err))
	}

	if len(products) == 0 {
		return c.Reply("没有可用的商品")
	}

	selector := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, p := range products {
		btn := selector.Data(p.Name, "cards", fmt.Sprintf("%d", p.ID))
		rows = append(rows, selector.Row(btn))
	}
	selector.Inline(rows...)

	h.state.Clear(c.Chat().ID)
	h.state.Set(c.Chat().ID, state.StateAwaitingCardSecrets, map[string]interface{}{
		"products": products,
	})

	return c.Reply("选择要补充卡密的商品：", selector)
}

// /fulfill
func (h *Handler) OnFulfill(c tele.Context) error {
	ctx := context.Background()
	resp, err := h.api.ListOrders(ctx, "fulfilling", 1, 100)
	if err != nil {
		return c.Reply(fmt.Sprintf("查询订单失败：%v", err))
	}

	if len(resp.Items) == 0 {
		return c.Reply("没有待发货的订单")
	}

	// Aggregate by product name
	type productAgg struct {
		Name      string
		Orders    []model.Order
		TotalQty  int
	}
	aggMap := make(map[string]*productAgg)
	var productNames []string
	for _, o := range resp.Items {
		name := o.ProductName
		if name == "" {
			name = "未知商品"
		}
		if _, ok := aggMap[name]; !ok {
			aggMap[name] = &productAgg{Name: name}
			productNames = append(productNames, name)
		}
		aggMap[name].Orders = append(aggMap[name].Orders, o)
		aggMap[name].TotalQty += o.Quantity
	}
	sort.Strings(productNames)

	selector := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, name := range productNames {
		a := aggMap[name]
		label := fmt.Sprintf("%s (%d单, %d个)", a.Name, len(a.Orders), a.TotalQty)
		// Encode product name in callback data (limit 64 bytes)
		data := fmt.Sprintf("fulfill|%s", a.Name)
		btn := selector.Data(label, "fulfill", data)
		rows = append(rows, selector.Row(btn))
	}
	selector.Inline(rows...)

	// Store aggregated data in state
	ordersData := make(map[string]interface{})
	for _, name := range productNames {
		ordersData[name] = aggMap[name].Orders
	}
	h.state.Set(c.Chat().ID, state.StateAwaitingFulfillSecrets, map[string]interface{}{
		"agg_orders": ordersData,
	})

	return c.Reply("选择要发货的商品：", selector)
}

// /stock
func (h *Handler) OnStock(c tele.Context) error {
	ctx := context.Background()
	products, err := h.loadAllProducts(ctx)
	if err != nil {
		return c.Reply(fmt.Sprintf("查询库存失败：%v", err))
	}

	if len(products) == 0 {
		return c.Reply("没有商品数据")
	}

	var sb strings.Builder
	sb.WriteString("库存概况：\n\n")
	for _, p := range products {
		sb.WriteString(fmt.Sprintf("📦 %s\n", p.Name))
		if len(p.SKUs) > 0 {
			for _, sku := range p.SKUs {
				status := "✅"
				if sku.StockCount <= h.cfg.StockAlert.Threshold {
					status = "⚠️"
				}
				sb.WriteString(fmt.Sprintf("   %s %s 库存：%d %s\n", status, sku.Name, sku.StockCount,
					map[bool]string{true: "(低库存!)", false: ""}[sku.StockCount <= h.cfg.StockAlert.Threshold]))
			}
		}
		sb.WriteString("\n")
	}
	return c.Reply(sb.String())
}

// /cancel
func (h *Handler) OnCancel(c tele.Context) error {
	h.state.Clear(c.Chat().ID)
	return c.Reply("已取消当前操作")
}

// OnCallback handles inline keyboard callbacks
func (h *Handler) OnCallback(c tele.Context) error {
	callback := c.Callback()

	// Parse callback prefix
	parts := strings.SplitN(callback.Data, "|", 2)
	prefix := parts[0]
	suffix := ""
	if len(parts) > 1 {
		suffix = parts[1]
	}

	switch prefix {
	case "sales":
		return h.handleSalesCallback(c, suffix)
	case "cards":
		return h.handleCardsCallback(c, suffix)
	case "fulfill":
		return h.handleFulfillCallback(c, suffix)
	default:
		return c.Respond()
	}
}

func (h *Handler) handleSalesCallback(c tele.Context, period string) error {
	ctx := context.Background()
	stats, err := h.api.GetSalesStats(ctx, period)
	if err != nil {
		return c.Reply(fmt.Sprintf("查询失败：%v", err))
	}

	periodLabel := map[string]string{
		"today":     "今天",
		"yesterday": "昨天",
		"week":      "本周",
		"month":     "本月",
	}[period]

	return c.Reply(fmt.Sprintf("%s销量统计：\n订单数：%d\n总金额：%s", periodLabel, stats.OrderCount, stats.TotalAmount))
}

func (h *Handler) handleCardsCallback(c tele.Context, productIDStr string) error {
	chatID := c.Chat().ID
	s, ok := h.state.Get(chatID)
	if !ok {
		return c.Reply("会话已过期，请重新 /cards")
	}

	products, _ := s.Data["products"].([]model.Product)
	var selected *model.Product
	for i := range products {
		if fmt.Sprintf("%d", products[i].ID) == productIDStr {
			selected = &products[i]
			break
		}
	}
	if selected == nil {
		return c.Reply("商品未找到")
	}

	h.state.Set(chatID, state.StateAwaitingCardSecrets, map[string]interface{}{
		"product_id": selected.ID,
		"product_name": selected.Name,
	})

	if len(selected.SKUs) > 0 {
		selector := &tele.ReplyMarkup{}
		var rows []tele.Row
		for _, sku := range selected.SKUs {
			btn := selector.Data(sku.Name, "cards_sku", fmt.Sprintf("%d", sku.ID))
			rows = append(rows, selector.Row(btn))
		}
		selector.Inline(rows...)
		return c.Reply(fmt.Sprintf("已选择商品：%s\n请选择 SKU：", selected.Name), selector)
	}

	return c.Reply(fmt.Sprintf("已选择商品：%s\n请发送卡密（每行一个）或上传 txt/csv 文件：", selected.Name))
}

func (h *Handler) handleCardsSKUCallback(c tele.Context, skuIDStr string) error {
	chatID := c.Chat().ID
	s, ok := h.state.Get(chatID)
	if !ok {
		return c.Reply("会话已过期，请重新 /cards")
	}

	s.Data["sku_id"] = skuIDStr
	return c.Reply("请发送卡密（每行一个）或上传 txt/csv 文件：")
}

func (h *Handler) handleFulfillCallback(c tele.Context, productName string) error {
	chatID := c.Chat().ID
	s, ok := h.state.Get(chatID)
	if !ok {
		return c.Reply("会话已过期，请重新 /fulfill")
	}

	aggOrders, _ := s.Data["agg_orders"].(map[string]interface{})
	ordersRaw, ok := aggOrders[productName]
	if !ok {
		return c.Reply("商品未找到")
	}

	orders, ok := ordersRaw.([]model.Order)
	if !ok {
		return c.Reply("数据异常，请重新 /fulfill")
	}

	// Sort orders by creation time (FIFO)
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].CreatedAt < orders[j].CreatedAt
	})

	totalQty := 0
	for _, o := range orders {
		totalQty += o.Quantity
	}

	h.state.Set(chatID, state.StateAwaitingFulfillSecrets, map[string]interface{}{
		"product_name": productName,
		"orders":       orders,
		"total_qty":    totalQty,
	})

	return c.Reply(fmt.Sprintf("商品：%s\n待发货订单：%d 个\n需要卡密总数：%d 个\n\n请发送卡密（每行一个）或上传 txt/csv 文件：", productName, len(orders), totalQty))
}

// OnText handles text messages (for card secret input)
func (h *Handler) OnText(c tele.Context) error {
	chatID := c.Chat().ID
	s, ok := h.state.Get(chatID)
	if !ok {
		return nil
	}

	secrets := parseSecrets(c.Text())
	if len(secrets) == 0 {
		return c.Reply("未检测到有效卡密，请每行一个发送")
	}

	switch s.Type {
	case state.StateAwaitingCardSecrets:
		return h.processCardSecrets(c, secrets, s)
	case state.StateAwaitingFulfillSecrets:
		return h.processFulfillSecrets(c, secrets, s)
	default:
		return nil
	}
}

// OnDocument handles file uploads (txt/csv for card secrets)
func (h *Handler) OnDocument(c tele.Context) error {
	chatID := c.Chat().ID
	s, ok := h.state.Get(chatID)
	if !ok {
		return nil
	}

	doc := c.Message().Document
	if doc == nil {
		return nil
	}

	file := doc.File
	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", h.cfg.Telegram.BotToken, file.FilePath)

	secrets, err := downloadAndParseFile(fileURL)
	if err != nil {
		return c.Reply(fmt.Sprintf("解析文件失败：%v", err))
	}

	if len(secrets) == 0 {
		return c.Reply("文件中未检测到有效卡密")
	}

	switch s.Type {
	case state.StateAwaitingCardSecrets:
		return h.processCardSecrets(c, secrets, s)
	case state.StateAwaitingFulfillSecrets:
		return h.processFulfillSecrets(c, secrets, s)
	default:
		return nil
	}
}

func (h *Handler) processCardSecrets(c tele.Context, secrets []string, s *state.ConversationState) error {
	ctx := context.Background()
	productID, _ := s.Data["product_id"].(int)
	skuIDStr, _ := s.Data["sku_id"].(string)
	var skuID uint
	if skuIDStr != "" {
		fmt.Sscanf(skuIDStr, "%d", &skuID)
	}

	productName, _ := s.Data["product_name"].(string)

	req := model.AddCardSecretsRequest{
		ProductID: uint(productID),
		SKUID:     skuID,
		Secrets:   secrets,
	}

	result, err := h.api.AddCardSecrets(ctx, req)
	if err != nil {
		h.state.Clear(c.Chat().ID)
		return c.Reply(fmt.Sprintf("补充卡密失败：%v", err))
	}

	h.state.Clear(c.Chat().ID)
	return c.Reply(fmt.Sprintf("✅ 商品 %s 成功补充 %d 个卡密", productName, result.Count))
}

func (h *Handler) processFulfillSecrets(c tele.Context, secrets []string, s *state.ConversationState) error {
	ctx := context.Background()
	productName, _ := s.Data["product_name"].(string)
	ordersRaw, _ := s.Data["orders"].([]model.Order)
	totalQty, _ := s.Data["total_qty"].(int)

	if len(secrets) < totalQty {
		return c.Reply(fmt.Sprintf("需要 %d 个卡密，但只收到 %d 个，请继续发送剩余卡密：", totalQty, len(secrets)))
	}

	successCount := 0
	failCount := 0
	secretIdx := 0

	for _, o := range ordersRaw {
		secretsForOrder := secrets[secretIdx : secretIdx+o.Quantity]
		secretIdx += o.Quantity

		payload := strings.Join(secretsForOrder, "\n")
		err := h.api.CreateFulfillment(ctx, model.CreateFulfillmentRequest{
			OrderID: o.ID,
			Payload: payload,
		})
		if err != nil {
			failCount++
		} else {
			successCount++
		}
	}

	h.state.Clear(c.Chat().ID)
	return c.Reply(fmt.Sprintf("📦 发货完成：%s\n✅ 成功：%d 个订单\n❌ 失败：%d 个订单", productName, successCount, failCount))
}

// Helper functions

func (h *Handler) loadAllProducts(ctx context.Context) ([]model.Product, error) {
	var allProducts []model.Product
	page := 1
	for {
		resp, err := h.api.ListProducts(ctx, page, 50)
		if err != nil {
			return nil, err
		}
		allProducts = append(allProducts, resp.Items...)
		if len(allProducts) >= int(resp.Total) {
			break
		}
		page++
	}
	return allProducts, nil
}

func parseSecrets(text string) []string {
	var secrets []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			secrets = append(secrets, line)
		}
	}
	return secrets
}

func downloadAndParseFile(fileURL string) ([]string, error) {
	resp, err := http.Get(fileURL)
	if err != nil {
		return nil, fmt.Errorf("下载文件失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载文件失败：status %d", resp.StatusCode)
	}

	var secrets []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			secrets = append(secrets, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件失败：%w", err)
	}
	return secrets, nil
}

// StockAlertChecker runs periodic stock checks and sends alerts
type StockAlertChecker struct {
	api *api.Client
	cfg *config.Config
	bot interface {
		Send(chatID int64, text string) error
	}
}

func NewStockAlertChecker(apiClient *api.Client, cfg *config.Config, bot interface {
	Send(chatID int64, text string) error
}) *StockAlertChecker {
	return &StockAlertChecker{
		api: apiClient,
		cfg: cfg,
		bot: bot,
	}
}

func (s *StockAlertChecker) Run(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.StockAlert.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.check(ctx)
		}
	}
}

func (s *StockAlertChecker) check(ctx context.Context) {
	products, err := s.api.ListProducts(ctx, 1, 100)
	if err != nil {
		return
	}

	var lowStock []string
	for _, p := range products.Items {
		for _, sku := range p.SKUs {
			if sku.StockCount <= s.cfg.StockAlert.Threshold {
				lowStock = append(lowStock, fmt.Sprintf("%s - %s: %d", p.Name, sku.Name, sku.StockCount))
			}
		}
	}

	if len(lowStock) > 0 {
		msg := "⚠️ 缺货提醒：\n\n" + strings.Join(lowStock, "\n")
		for _, uid := range s.cfg.Telegram.AllowedUsers {
			s.bot.Send(uid, msg)
		}
	}
}

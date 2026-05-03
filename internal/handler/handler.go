package handler

import (
	"bufio"
	"context"
	"encoding/json"
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
			"/fulfill - 批量发货（按商品）\n" +
		"/pfulfill - 按母订单发货\n" +
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
	orders, err := h.api.ListFulfillingOrders(ctx)
	if err != nil {
		return c.Reply(fmt.Sprintf("查询失败：%v", err))
	}

	if len(orders) == 0 {
		return c.Reply("没有待处理的订单")
	}

	// Resolve leaf orders for display
	var leafOrders []model.Order
	for _, o := range orders {
		if len(o.Children) > 0 {
			for _, ch := range o.Children {
				if ch.Status == "fulfilling" || ch.Status == "paid" {
					leafOrders = append(leafOrders, ch)
				}
			}
		} else {
			leafOrders = append(leafOrders, o)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("待处理订单（共 %d 个）：\n\n", len(leafOrders)))
	for _, o := range leafOrders {
		itemDesc := orderItemSummary(o)
		sb.WriteString(fmt.Sprintf("📦 %s\n", o.OrderNo))
		sb.WriteString(fmt.Sprintf("   %s | 金额：%s\n", itemDesc, o.TotalAmount))
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
		name := model.GetProductName(p.Title)
		btn := selector.Data(name, "cards", fmt.Sprintf("%d", p.ID))
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
	orders, err := h.api.ListFulfillingOrders(ctx)
	if err != nil {
		return c.Reply(fmt.Sprintf("查询订单失败：%v", err))
	}

	if len(orders) == 0 {
		return c.Reply("没有待发货的订单")
	}

	// Resolve leaf orders: parent orders with children must use children for fulfillment
	// Only include children that still need fulfillment (fulfilling or paid status)
	var leafOrders []model.Order
	for _, o := range orders {
		if len(o.Children) > 0 {
			for _, ch := range o.Children {
				if ch.Status == "fulfilling" || ch.Status == "paid" {
					leafOrders = append(leafOrders, ch)
				}
			}
		} else {
			leafOrders = append(leafOrders, o)
		}
	}

	// Aggregate by product name from leaf order items
	type productAgg struct {
		Name     string
		Orders   []model.Order
		TotalQty int
	}
	aggMap := make(map[string]*productAgg)
	var productNames []string
	for _, o := range leafOrders {
		for _, item := range o.Items {
			name := model.GetProductName(item.Title)
			if name == "" {
				name = "未知商品"
			}
			if _, ok := aggMap[name]; !ok {
				aggMap[name] = &productAgg{Name: name}
				productNames = append(productNames, name)
			}
			// Only add order once per product name
			alreadyHas := false
			for _, existing := range aggMap[name].Orders {
				if existing.ID == o.ID {
					alreadyHas = true
					break
				}
			}
			if !alreadyHas {
				aggMap[name].Orders = append(aggMap[name].Orders, o)
			}
			aggMap[name].TotalQty += item.Quantity
		}
	}
	sort.Strings(productNames)

	selector := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, name := range productNames {
		a := aggMap[name]
		label := fmt.Sprintf("%s (%d单, %d个)", a.Name, len(a.Orders), a.TotalQty)
		btn := selector.Data(label, "fulfill", a.Name)
		rows = append(rows, selector.Row(btn))
	}
	selector.Inline(rows...)

	// Store orders per product name (not full productAgg) for callback deserialization
	ordersPerProduct := make(map[string][]model.Order)
	for name, a := range aggMap {
		ordersPerProduct[name] = a.Orders
	}
	ordersJSON, _ := json.Marshal(ordersPerProduct)
	h.state.Set(c.Chat().ID, state.StateAwaitingFulfillSecrets, map[string]interface{}{
		"agg_json": string(ordersJSON),
	})

	return c.Reply("选择要发货的商品：", selector)
}

// /pfulfill — parent-order fulfillment mode
func (h *Handler) OnParentFulfill(c tele.Context) error {
	ctx := context.Background()
	orders, err := h.api.ListFulfillingOrders(ctx)
	if err != nil {
		return c.Reply(fmt.Sprintf("查询订单失败：%v", err))
	}

	// Only keep parent orders (those with children)
	var parentOrders []model.Order
	for _, o := range orders {
		if len(o.Children) > 0 {
			parentOrders = append(parentOrders, o)
		}
	}
	if len(parentOrders) == 0 {
		return c.Reply("没有需要发货的母订单")
	}

	selector := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, o := range parentOrders {
		needFulfill := 0
		for _, ch := range o.Children {
			if ch.Status == "fulfilling" || ch.Status == "paid" {
				qty := 0
				for _, item := range ch.Items {
					qty += item.Quantity
				}
				needFulfill += qty
			}
		}
		label := fmt.Sprintf("%s (%d个子订单待发, 共需%d个卡密)", o.OrderNo, countFulfillableChildren(o.Children), needFulfill)
		btn := selector.Data(label, "pfulfill", fmt.Sprintf("%d", o.ID))
		rows = append(rows, selector.Row(btn))
	}
	selector.Inline(rows...)

	parentsJSON, _ := json.Marshal(parentOrders)
	h.state.Clear(c.Chat().ID)
	h.state.Set(c.Chat().ID, state.StateAwaitingParentFulfillSecrets, map[string]interface{}{
		"parents_json": string(parentsJSON),
	})

	return c.Reply("选择要发货的母订单：", selector)
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
		name := model.GetProductName(p.Title)
		sb.WriteString(fmt.Sprintf("📦 %s\n", name))

		if p.FulfillmentType == "auto" {
			avail := p.AutoStockAvailable
			status := "✅"
			lowMsg := ""
			if avail <= h.cfg.StockAlert.Threshold {
				status = "⚠️"
				lowMsg = "(低库存!)"
			}
			sb.WriteString(fmt.Sprintf("   %s 自动发货 库存：%d %s\n", status, avail, lowMsg))
		} else if p.ManualStockTotal < 0 {
			sb.WriteString("   ✅ 人工发货 库存：无限（按需发货）\n")
		} else {
			avail := p.ManualStockTotal - p.ManualStockLocked - p.ManualStockSold
			status := "✅"
			lowMsg := ""
			if avail <= h.cfg.StockAlert.Threshold {
				status = "⚠️"
				lowMsg = "(低库存!)"
			}
			sb.WriteString(fmt.Sprintf("   %s 人工发货 库存：%d %s\n", status, avail, lowMsg))
		}

		if len(p.SKUs) > 0 {
			for _, sku := range p.SKUs {
				skuLabel := sku.SKUCode
				if skuLabel == "DEFAULT" {
					skuLabel = "默认规格"
				}
				sb.WriteString(fmt.Sprintf("      SKU: %s\n", skuLabel))
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
	data := callback.Data

	// telebot prefixes inline button data with "\f<unique>|"
	// When no specific handler matches, OnCallback gets the raw data
	if len(data) > 0 && data[0] == '\f' {
		data = data[1:]
	}

	parts := strings.SplitN(data, "|", 2)
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
	case "cards_sku":
		return h.handleCardsSKUCallback(c, suffix)
	case "fulfill":
		return h.handleFulfillCallback(c, suffix)
	case "pfulfill":
		return h.handleParentFulfillCallback(c, suffix)
	default:
		return c.Respond()
	}
}

func (h *Handler) handleSalesCallback(c tele.Context, period string) error {
	ctx := context.Background()

	// Map period to dashboard range param
	rangeParam := map[string]string{
		"today":     "7d", // dashboard doesn't have "today"; we use 7d and show total
		"yesterday": "7d",
		"week":      "7d",
		"month":     "30d",
	}[period]

	overview, err := h.api.GetDashboardOverview(ctx, "range="+rangeParam)
	if err != nil {
		return c.Reply(fmt.Sprintf("查询失败：%v", err))
	}

	periodLabel := map[string]string{
		"today":     "近7天",
		"yesterday": "近7天",
		"week":      "近7天",
		"month":     "近30天",
	}[period]

	msg := fmt.Sprintf("%s销量统计：\n总营收(GMV)：%s\n已付订单：%d\n完成订单：%d\n利润：%s (利润率 %s%%)", periodLabel, overview.KPI.GMVPaid, overview.KPI.PaidOrders, overview.KPI.CompletedOrders, overview.KPI.TotalProfit, overview.KPI.ProfitMargin)

	if len(overview.Alerts) > 0 {
		msg += "\n\n⚠️ 告警："
		for _, a := range overview.Alerts {
			msg += fmt.Sprintf("\n  %s: %d", a.Type, a.Value)
		}
	}

	return c.Reply(msg)
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

	productName := model.GetProductName(selected.Title)

	h.state.Set(chatID, state.StateAwaitingCardSecrets, map[string]interface{}{
		"product_id":   selected.ID,
		"product_name": productName,
	})

	if len(selected.SKUs) > 0 {
		selector := &tele.ReplyMarkup{}
		var rows []tele.Row
		for _, sku := range selected.SKUs {
			skuLabel := sku.SKUCode
			if skuLabel == "DEFAULT" {
				skuLabel = "默认规格"
			}
			btn := selector.Data(skuLabel, "cards_sku", fmt.Sprintf("%d", sku.ID))
			rows = append(rows, selector.Row(btn))
		}
		selector.Inline(rows...)
		return c.Reply(fmt.Sprintf("已选择商品：%s\n请选择 SKU：", productName), selector)
	}

	return c.Reply(fmt.Sprintf("已选择商品：%s\n请发送卡密（每行一个）或上传 txt/csv 文件：", productName))
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

	aggJSONStr, _ := s.Data["agg_json"].(string)

	var aggMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(aggJSONStr), &aggMap); err != nil {
		return c.Reply("数据异常，请重新 /fulfill")
	}

	ordersRaw, ok := aggMap[productName]
	if !ok {
		return c.Reply("商品未找到")
	}
	var orders []model.Order
	if err := json.Unmarshal(ordersRaw, &orders); err != nil {
		return c.Reply("数据异常，请重新 /fulfill")
	}

	// Sort orders by creation time (FIFO)
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].CreatedAt < orders[j].CreatedAt
	})

	totalQty := 0
	for _, o := range orders {
		for _, item := range o.Items {
			totalQty += item.Quantity
		}
	}

	// Re-serialize orders for next state
	ordersBytes, _ := json.Marshal(orders)
	h.state.Set(chatID, state.StateAwaitingFulfillSecrets, map[string]interface{}{
		"product_name": productName,
		"orders_json":  string(ordersBytes),
		"total_qty":    totalQty,
	})

	return c.Reply(fmt.Sprintf("商品：%s\n待发货订单：%d 个\n需要卡密总数：%d 个\n\n请发送卡密（每行一个）或上传 txt/csv 文件：\n（卡密不足时将按订单顺序尽可能发货，剩余订单可稍后再发）", productName, len(orders), totalQty))
}

func (h *Handler) handleParentFulfillCallback(c tele.Context, orderIDStr string) error {
	chatID := c.Chat().ID
	s, ok := h.state.Get(chatID)
	if !ok {
		return c.Reply("会话已过期，请重新 /pfulfill")
	}

	parentsJSONStr, _ := s.Data["parents_json"].(string)
	var parentOrders []model.Order
	if err := json.Unmarshal([]byte(parentsJSONStr), &parentOrders); err != nil {
		return c.Reply("数据异常，请重新 /pfulfill")
	}

	var selected *model.Order
	orderIDInt, _ := fmt.Sscanf(orderIDStr, "%d", new(uint))
	for i := range parentOrders {
		if fmt.Sprintf("%d", parentOrders[i].ID) == orderIDStr {
			selected = &parentOrders[i]
			break
		}
	}
	if selected == nil {
		_ = orderIDInt
		return c.Reply("订单未找到")
	}

	// Collect fulfillable children (fulfilling or paid)
	var children []model.Order
	for _, ch := range selected.Children {
		if ch.Status == "fulfilling" || ch.Status == "paid" {
			children = append(children, ch)
		}
	}
	if len(children) == 0 {
		return c.Reply("该母订单没有待发货的子订单")
	}

	sort.Slice(children, func(i, j int) bool {
		return children[i].CreatedAt < children[j].CreatedAt
	})

	totalQty := 0
	var childDescs []string
	for _, ch := range children {
		qty := 0
		for _, item := range ch.Items {
			qty += item.Quantity
		}
		totalQty += qty
		childDescs = append(childDescs, fmt.Sprintf("  %s | %s | 数量：%d", ch.OrderNo, orderItemSummary(ch), qty))
	}

	childrenJSON, _ := json.Marshal(children)
	h.state.Set(chatID, state.StateAwaitingParentFulfillSecrets, map[string]interface{}{
		"parent_id":      selected.ID,
		"parent_order_no": selected.OrderNo,
		"children_json":  string(childrenJSON),
		"total_qty":      totalQty,
	})

	msg := fmt.Sprintf("母订单：%s\n待发货子订单：%d 个\n需要卡密总数：%d 个\n\n子订单列表：\n%s\n\n请发送卡密（每行一个）或上传 txt/csv 文件：\n（卡密数量必须等于 %d，否则不发货）",
		selected.OrderNo, len(children), totalQty,
		strings.Join(childDescs, "\n"), totalQty)
	return c.Reply(msg)
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
	case state.StateAwaitingParentFulfillSecrets:
		return h.processParentFulfillSecrets(c, secrets, s)
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
	case state.StateAwaitingParentFulfillSecrets:
		return h.processParentFulfillSecrets(c, secrets, s)
	default:
		return nil
	}
}

func (h *Handler) processCardSecrets(c tele.Context, secrets []string, s *state.ConversationState) error {
	ctx := context.Background()

	productID := uintFromIface(s.Data["product_id"])
	skuIDStr, _ := s.Data["sku_id"].(string)
	var skuID uint
	if skuIDStr != "" {
		fmt.Sscanf(skuIDStr, "%d", &skuID)
	}

	productName, _ := s.Data["product_name"].(string)

	req := model.CreateCardSecretBatchRequest{
		ProductID: productID,
		SKUID:     skuID,
		Secrets:   secrets,
	}

	result, err := h.api.CreateCardSecretBatch(ctx, req)
	if err != nil {
		h.state.Clear(c.Chat().ID)
		return c.Reply(fmt.Sprintf("补充卡密失败：%v", err))
	}

	h.state.Clear(c.Chat().ID)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✅ 商品 %s 补充卡密成功\n", productName))
	sb.WriteString(fmt.Sprintf("   卡密数量：%d 个\n", result.Created))
	if result.BatchNo != "" {
		sb.WriteString(fmt.Sprintf("   批次号：%s\n", result.BatchNo))
	}
	if skuID != 0 {
		sb.WriteString(fmt.Sprintf("   SKU ID：%d\n", skuID))
	}

	return c.Reply(sb.String())
}

func (h *Handler) processFulfillSecrets(c tele.Context, secrets []string, s *state.ConversationState) error {
	ctx := context.Background()
	productName, _ := s.Data["product_name"].(string)
	ordersJSON, _ := s.Data["orders_json"].(string)

	var orders []model.Order
	if err := json.Unmarshal([]byte(ordersJSON), &orders); err != nil {
		h.state.Clear(c.Chat().ID)
		return c.Reply("数据异常，请重新 /fulfill")
	}

	// Fulfill as many complete orders as possible in FIFO order.
	// Each order must receive ALL its required secrets — we never
	// partially fulfill a single order (that would leave the customer
	// short). Orders that can't be fully covered are skipped.
	type fulfillResult struct {
		OrderID  uint
		OrderNo  string
		Qty      int
		Err      error
	}
	var results []fulfillResult
	secretIdx := 0

	for _, o := range orders {
		qty := 0
		for _, item := range o.Items {
			qty += item.Quantity
		}
		if qty == 0 {
			continue
		}
		if secretIdx+qty > len(secrets) {
			break
		}
		secretsForOrder := secrets[secretIdx : secretIdx+qty]
		secretIdx += qty

		payload := strings.Join(secretsForOrder, "\n")
		_, err := h.api.CreateFulfillment(ctx, model.CreateFulfillmentRequest{
			OrderID: o.ID,
			Payload: payload,
		})
		results = append(results, fulfillResult{OrderID: o.ID, OrderNo: o.OrderNo, Qty: qty, Err: err})

		if err == nil {
			if compErr := h.api.UpdateOrderStatus(ctx, o.ID, "completed"); compErr != nil {
				_ = compErr
			}
		}
	}

	var sb strings.Builder
	successCount := 0
	skippedCount := 0
	usedSecrets := 0
	var failedOrders []string
	var successDetails []string

	for _, r := range results {
		if r.Err == nil {
			successCount++
			usedSecrets += r.Qty
			// Query order status for detail
			orderStatus := "?"
			if o, err := h.api.GetOrder(ctx, r.OrderID); err == nil {
				orderStatus = o.Status
			}
			successDetails = append(successDetails, fmt.Sprintf("  %s → %s（%d个卡密）", r.OrderNo, orderStatus, r.Qty))
		} else {
			errMsg := r.Err.Error()
			if strings.Contains(errMsg, "已存在") || strings.Contains(errMsg, "already exist") {
				skippedCount++
			} else {
				failedOrders = append(failedOrders, fmt.Sprintf("订单 %d: %s", r.OrderID, errMsg))
			}
		}
	}

	unusedSecrets := len(secrets) - usedSecrets
	pendingOrders := len(orders) - successCount - skippedCount - len(failedOrders)

	sb.WriteString(fmt.Sprintf("📦 发货完成：%s\n✅ 成功：%d 个订单（用了 %d 个卡密）", productName, successCount, usedSecrets))
	for _, d := range successDetails {
		sb.WriteString("\n" + d)
	}
	if skippedCount > 0 {
		sb.WriteString(fmt.Sprintf("\n⏭️ 已发货跳过：%d 个订单", skippedCount))
	}
	if len(failedOrders) > 0 {
		sb.WriteString(fmt.Sprintf("\n❌ 失败：%d 个订单", len(failedOrders)))
		for _, f := range failedOrders {
			sb.WriteString(fmt.Sprintf("\n  %s", f))
		}
	}
	if unusedSecrets > 0 {
		sb.WriteString(fmt.Sprintf("\n⚠️ 多余卡密：%d 个（未使用）", unusedSecrets))
	}
	if pendingOrders > 0 {
		sb.WriteString(fmt.Sprintf("\n📋 待发货：%d 个订单仍需卡密，可重新 /fulfill", pendingOrders))
	}

	h.state.Clear(c.Chat().ID)
	return c.Reply(sb.String())
}

func (h *Handler) processParentFulfillSecrets(c tele.Context, secrets []string, s *state.ConversationState) error {
	ctx := context.Background()
	parentOrderNo, _ := s.Data["parent_order_no"].(string)
	parentID := uintFromIface(s.Data["parent_id"])
	childrenJSON, _ := s.Data["children_json"].(string)
	totalQty := intFromIface(s.Data["total_qty"])

	var children []model.Order
	if err := json.Unmarshal([]byte(childrenJSON), &children); err != nil {
		h.state.Clear(c.Chat().ID)
		return c.Reply("数据异常，请重新 /pfulfill")
	}

	// Strict quantity check: must match exactly
	if len(secrets) != totalQty {
		h.state.Clear(c.Chat().ID)
		return c.Reply(fmt.Sprintf("❌ 卡密数量不匹配：需要 %d 个，收到 %d 个\n发货已取消，请重新 /pfulfill", totalQty, len(secrets)))
	}

	// Fulfill each child order in order
	type childResult struct {
		OrderNo string
		Qty     int
		Err     error
	}
	var results []childResult
	secretIdx := 0
	allSuccess := true

	for _, ch := range children {
		qty := 0
		for _, item := range ch.Items {
			qty += item.Quantity
		}
		if qty == 0 {
			continue
		}

		secretsForOrder := secrets[secretIdx : secretIdx+qty]
		secretIdx += qty
		payload := strings.Join(secretsForOrder, "\n")

		_, err := h.api.CreateFulfillment(ctx, model.CreateFulfillmentRequest{
			OrderID: ch.ID,
			Payload: payload,
		})
		results = append(results, childResult{OrderNo: ch.OrderNo, Qty: qty, Err: err})

		if err == nil {
			if compErr := h.api.UpdateOrderStatus(ctx, ch.ID, "completed"); compErr != nil {
				_ = compErr
			}
		} else {
			allSuccess = false
		}
	}

	// Build result table
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📦 母订单发货结果：%s\n\n", parentOrderNo))

	if allSuccess {
		sb.WriteString("✅ 全部子订单发货成功\n\n")
	} else {
		sb.WriteString("⚠️ 部分子订单发货失败\n\n")
	}

	for i, r := range results {
		if r.Err == nil {
			// Query latest status
			childStatus := "?"
			if o, err := h.api.GetOrder(ctx, children[i].ID); err == nil {
				childStatus = o.Status
			}
			sb.WriteString(fmt.Sprintf("✅ %s | %d个卡密 → %s\n", r.OrderNo, r.Qty, childStatus))
		} else {
			sb.WriteString(fmt.Sprintf("❌ %s | %d个卡密 → 错误：%s\n", r.OrderNo, r.Qty, r.Err.Error()))
		}
	}

	// Query parent order status after fulfillment
	if allSuccess {
		if parent, err := h.api.GetOrder(ctx, parentID); err == nil {
			sb.WriteString(fmt.Sprintf("\n📋 母订单状态：%s", parent.Status))
			sb.WriteString(buildParentOrderDetail(parent))
		} else {
			sb.WriteString(fmt.Sprintf("\n📋 查询母订单状态失败：%v", err))
		}
	}

	h.state.Clear(c.Chat().ID)
	return c.Reply(sb.String())
}

// --- Helper functions ---

func countFulfillableChildren(children []model.Order) int {
	n := 0
	for _, ch := range children {
		if ch.Status == "fulfilling" || ch.Status == "paid" {
			n++
		}
	}
	return n
}

func buildParentOrderDetail(parent *model.Order) string {
	if parent == nil || len(parent.Children) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n\n📊 母订单详情：%s", parent.OrderNo))
	sb.WriteString(fmt.Sprintf("\n金额：%s | 子订单数：%d", parent.TotalAmount, len(parent.Children)))
	for _, ch := range parent.Children {
		itemDesc := orderItemSummary(ch)
		sb.WriteString(fmt.Sprintf("\n  %s | %s | %s", ch.OrderNo, itemDesc, ch.Status))
	}
	return sb.String()
}

func (h *Handler) loadAllProducts(ctx context.Context) ([]model.Product, error) {
	var allProducts []model.Product
	page := 1
	for {
		products, pagination, err := h.api.ListProducts(ctx, page, 50)
		if err != nil {
			return nil, err
		}
		allProducts = append(allProducts, products...)
		if len(allProducts) >= int(pagination.Total) {
			break
		}
		page++
	}
	return allProducts, nil
}

func orderItemSummary(o model.Order) string {
	var parts []string
	for _, item := range o.Items {
		name := model.GetProductName(item.Title)
		parts = append(parts, fmt.Sprintf("%s x%d", name, item.Quantity))
	}
	return strings.Join(parts, ", ")
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

func intFromIface(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}

func uintFromIface(v interface{}) uint {
	switch n := v.(type) {
	case uint:
		return n
	case int:
		return uint(n)
	case int64:
		return uint(n)
	case float64:
		return uint(n)
	case json.Number:
		i, _ := n.Int64()
		return uint(i)
	}
	return 0
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
	alerts, err := s.api.GetInventoryAlerts(ctx)
	if err != nil {
		return
	}

	if len(alerts) == 0 {
		return
	}

	var msgs []string
	for _, a := range alerts {
		pName := model.GetProductName(a.ProductTitle)
		sName := model.GetProductName(a.SKUName)
		msgs = append(msgs, fmt.Sprintf("%s - %s: 可用 %d / 总计 %d", pName, sName, a.AvailableStock, a.TotalStock))
	}

	msg := "⚠️ 缺货提醒：\n\n" + strings.Join(msgs, "\n")
	for _, uid := range s.cfg.Telegram.AllowedUsers {
		s.bot.Send(uid, msg)
	}
}

package order

import (
	"fmt"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

const SurfaceID = "order-tracker"

func statusLabel(state OrderState) string {
	if state.Cancelled {
		return "❌ Order Cancelled"
	}
	prefix := ""
	if state.Rushed {
		prefix = "⚡ RUSH — "
	}
	switch state.Status {
	case "confirmed":
		return prefix + "✅ Order Confirmed"
	case "preparing":
		return prefix + "👨‍🍳 Preparing in Kitchen"
	case "out_for_delivery":
		return prefix + "🛵 Out for Delivery"
	case "delivered":
		return "🎉 Delivered!"
	default:
		return prefix + state.Status
	}
}

func etaText(state OrderState) string {
	if state.Cancelled {
		if state.CancelReason != "" {
			return state.CancelReason
		}
		return "Order was cancelled"
	}
	if state.Status == "delivered" {
		return "Arrived at your door — enjoy!"
	}
	return fmt.Sprintf("Estimated arrival: ~%d min (Step %d of %d)", state.ETAMinutes, state.Step, state.TotalSteps)
}

func CreateOrderSurface() []map[string]any {
	return []map[string]any{a2ui.CreateSurface(SurfaceID, true)}
}

func BuildInitialComponents(state OrderState) []map[string]any {
	itemRowIDs := make([]string, 0, len(state.Items))
	itemComponents := make([]map[string]any, 0, len(state.Items)*3)

	for i, item := range state.Items {
		rowID := fmt.Sprintf("item_row_%d", i)
		nameID := fmt.Sprintf("item_name_%d", i)
		priceID := fmt.Sprintf("item_price_%d", i)
		itemRowIDs = append(itemRowIDs, rowID)
		itemComponents = append(itemComponents,
			map[string]any{
				"id":        rowID,
				"component": "Row",
				"children":  []string{nameID, priceID},
				"justify":   "spaceBetween",
			},
			map[string]any{
				"id":        nameID,
				"component": "Text",
				"text":      fmt.Sprintf("%d× %s", item.Qty, item.Name),
				"variant":   "body",
			},
			map[string]any{
				"id":        priceID,
				"component": "Text",
				"text":      fmt.Sprintf("$%.2f", item.Price*float64(item.Qty)),
				"variant":   "body",
			},
		)
	}

	itemsChildren := []string{"items_header"}
	itemsChildren = append(itemsChildren, itemRowIDs...)
	itemsChildren = append(itemsChildren, "items_divider", "total_text")

	components := []map[string]any{
		{"id": "root", "component": "Column", "children": []string{"status_card", "items_card"}},
		{"id": "status_card", "component": "Card", "child": "status_col"},
		{
			"id":        "status_col",
			"component": "Column",
			"children":  []string{"order_title", "status_badge", "eta_label", "address_label"},
		},
		{"id": "order_title", "component": "Text", "text": a2ui.Path("/title"), "variant": "h3"},
		{"id": "status_badge", "component": "Text", "text": a2ui.Path("/status_text"), "variant": "h4"},
		{"id": "eta_label", "component": "Text", "text": a2ui.Path("/eta_text"), "variant": "body"},
		{"id": "address_label", "component": "Text", "text": a2ui.Path("/address_text"), "variant": "caption"},
		{"id": "items_card", "component": "Card", "child": "items_col"},
		{"id": "items_col", "component": "Column", "children": itemsChildren},
		{"id": "items_header", "component": "Text", "text": "Order Summary", "variant": "h4"},
	}
	components = append(components, itemComponents...)
	components = append(components,
		map[string]any{"id": "items_divider", "component": "Divider"},
		map[string]any{"id": "total_text", "component": "Text", "text": a2ui.Path("/total_text"), "variant": "h4"},
	)

	return []map[string]any{a2ui.UpdateComponents(SurfaceID, components)}
}

func BuildInitialDataModel(state OrderState) []map[string]any {
	return []map[string]any{
		a2ui.UpdateDataModel(SurfaceID, "", map[string]any{
			"title":        fmt.Sprintf("Order #%s for %s", state.OrderID, state.Customer),
			"status_text":  statusLabel(state),
			"eta_text":     etaText(state),
			"address_text": fmt.Sprintf("📍 Delivering to %s", state.Address),
			"total_text":   fmt.Sprintf("Total: $%.2f", state.Total),
			"step":         state.Step,
			"total_steps":  state.TotalSteps,
		}),
	}
}

func UpdateStepProgress(state OrderState) []map[string]any {
	val := map[string]any{
		"status_text": statusLabel(state),
		"eta_text":    etaText(state),
		"step":        state.Step,
	}
	if state.Driver != nil {
		val["driver_name"] = fmt.Sprintf("🛵 Driver: %s (%s)", state.Driver.Name, state.Driver.Vehicle)
		val["driver_rating"] = fmt.Sprintf("⭐ %.1f rating", state.Driver.Rating)
	}
	return []map[string]any{a2ui.UpdateDataModel(SurfaceID, "", val)}
}

func AddDriverCard(state OrderState) []map[string]any {
	if state.Driver == nil {
		return nil
	}
	return []map[string]any{
		a2ui.UpdateComponents(SurfaceID, []map[string]any{
			{"id": "root", "component": "Column", "children": []string{"status_card", "driver_card", "items_card"}},
			{"id": "driver_card", "component": "Card", "child": "driver_col"},
			{"id": "driver_col", "component": "Column", "children": []string{"driver_hdr", "driver_name", "driver_rating"}},
			{"id": "driver_hdr", "component": "Text", "text": "Live Courier Assigned", "variant": "h4"},
			{"id": "driver_name", "component": "Text", "text": a2ui.Path("/driver_name"), "variant": "body"},
			{"id": "driver_rating", "component": "Text", "text": a2ui.Path("/driver_rating"), "variant": "caption"},
		}),
		a2ui.UpdateDataModel(SurfaceID, "", map[string]any{
			"driver_name":   fmt.Sprintf("🛵 Driver: %s (%s)", state.Driver.Name, state.Driver.Vehicle),
			"driver_rating": fmt.Sprintf("⭐ %.1f rating", state.Driver.Rating),
		}),
	}
}

func UpdateCancelled(state OrderState) []map[string]any {
	return []map[string]any{
		a2ui.UpdateDataModel(SurfaceID, "", map[string]any{
			"status_text": statusLabel(state),
			"eta_text":    etaText(state),
		}),
	}
}

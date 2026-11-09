package concierge

import (
	"fmt"
	"strings"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

type MenuItemInfo struct {
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
	Origin      string  `json:"origin"`
}

var MenuCatalog = []MenuItemInfo{
	{
		Name:        "Tacos al Pastor",
		Price:       11.50,
		Description: "Order of 4 CDMX street tacos (adobo pork, roasted pineapple, cilantro, nixtamal corn tortilla)",
		Origin:      "Taquería Orinoco CDMX",
	},
	{
		Name:        "Gringa de Pastor",
		Price:       13.50,
		Description: "Melted Oaxaca cheese & char-grilled al pastor on flour tortillas with salsa verde",
		Origin:      "Taquería Orinoco CDMX",
	},
	{
		Name:        "Horchata Artesanal",
		Price:       4.50,
		Description: "House-made CDMX rice water with Ceylon cinnamon & vanilla (500ml)",
		Origin:      "Taquería Orinoco CDMX",
	},
	{
		Name:        "Brown Sugar Boba",
		Price:       6.50,
		Description: "Warm tapioca pearls, Okinawa brown sugar, oat milk — Campus Hackathon favorite",
		Origin:      "Boba & Byte Campus Lab",
	},
	{
		Name:        "Margherita Pizza",
		Price:       12.00,
		Description: "Classic San Marzano tomato, fresh mozzarella, sweet basil",
		Origin:      "Bella Napoli",
	},
	{
		Name:        "Pepperoni Pizza",
		Price:       14.00,
		Description: "Crispy pepperoni, mozzarella, pickled jalapeños",
		Origin:      "Bella Napoli",
	},
	{
		Name:        "Caesar Salad",
		Price:       8.50,
		Description: "Crisp romaine, aged parmesan, garlic croutons, house Caesar dressing",
		Origin:      "Green Garden",
	},
	{
		Name:        "Sparkling Water",
		Price:       3.00,
		Description: "Chilled San Pellegrino 500ml",
		Origin:      "Green Garden",
	},
	{
		Name:        "Tiramisu",
		Price:       7.50,
		Description: "Espresso-soaked ladyfingers, mascarpone cream, cocoa dust",
		Origin:      "Bella Napoli",
	},
}

func FindMenuItem(query string) (MenuItemInfo, bool) {
	q := strings.TrimSpace(strings.ToLower(query))
	for _, m := range MenuCatalog {
		if strings.ToLower(m.Name) == q {
			return m, true
		}
	}
	for _, m := range MenuCatalog {
		if strings.Contains(strings.ToLower(m.Name), q) || strings.Contains(q, strings.ToLower(m.Name)) {
			return m, true
		}
	}
	if strings.Contains(q, "taco") || strings.Contains(q, "pastor") {
		return MenuCatalog[0], true
	}
	if strings.Contains(q, "gringa") {
		return MenuCatalog[1], true
	}
	if strings.Contains(q, "horchata") {
		return MenuCatalog[2], true
	}
	if strings.Contains(q, "boba") {
		return MenuCatalog[3], true
	}
	if strings.Contains(q, "margherita") || strings.Contains(q, "pizza") {
		return MenuCatalog[4], true
	}
	if strings.Contains(q, "pepperoni") {
		return MenuCatalog[5], true
	}
	if strings.Contains(q, "salad") || strings.Contains(q, "caesar") {
		return MenuCatalog[6], true
	}
	if strings.Contains(q, "water") {
		return MenuCatalog[7], true
	}
	if strings.Contains(q, "tiramisu") || strings.Contains(q, "dessert") {
		return MenuCatalog[8], true
	}
	return MenuItemInfo{}, false
}

// BuildMenuSurface builds an interactive A2UI v1.0 menu card with 1-click "Add" buttons.
func BuildMenuSurface(filter string) []map[string]any {
	surfaceID := "menu-surface"
	rowIDs := make([]string, 0, len(MenuCatalog))
	comps := []map[string]any{
		{"id": "root", "component": "Card", "child": "menu_col"},
		{"id": "menu_header", "component": "Text", "text": "🍽️ Durable Eats — CDMX & Campus Menu", "variant": "h3"},
		{"id": "menu_sub", "component": "Text", "text": "Click any item to add it to your durable workflow cart", "variant": "caption"},
	}

	for i, item := range MenuCatalog {
		if filter != "" && filter != "all" {
			f := strings.ToLower(filter)
			if !strings.Contains(strings.ToLower(item.Name), f) &&
				!strings.Contains(strings.ToLower(item.Origin), f) &&
				!strings.Contains(strings.ToLower(item.Description), f) {
				continue
			}
		}
		rowID := fmt.Sprintf("menu_row_%d", i)
		infoColID := fmt.Sprintf("menu_info_%d", i)
		titleID := fmt.Sprintf("menu_title_%d", i)
		descID := fmt.Sprintf("menu_desc_%d", i)
		btnID := fmt.Sprintf("menu_btn_%d", i)
		btnLblID := fmt.Sprintf("menu_btn_lbl_%d", i)

		rowIDs = append(rowIDs, rowID)
		comps = append(comps,
			map[string]any{
				"id":        rowID,
				"component": "Row",
				"children":  []string{infoColID, btnID},
				"justify":   "spaceBetween",
				"align":     "center",
			},
			map[string]any{
				"id":        infoColID,
				"component": "Column",
				"children":  []string{titleID, descID},
			},
			map[string]any{
				"id":        titleID,
				"component": "Text",
				"text":      fmt.Sprintf("%s — $%.2f (%s)", item.Name, item.Price, item.Origin),
				"variant":   "h4",
			},
			map[string]any{
				"id":        descID,
				"component": "Text",
				"text":      item.Description,
				"variant":   "caption",
			},
			map[string]any{
				"id":        btnID,
				"component": "Button",
				"child":     btnLblID,
				"action":    a2ui.EventAction(fmt.Sprintf("Add 1 %s", item.Name), nil),
			},
			map[string]any{
				"id":        btnLblID,
				"component": "Text",
				"text":      fmt.Sprintf("＋ Add ($%.2f)", item.Price),
			},
		)
	}

	colChildren := []string{"menu_header", "menu_sub"}
	colChildren = append(colChildren, rowIDs...)
	comps = append(comps, map[string]any{
		"id":        "menu_col",
		"component": "Column",
		"children":  colChildren,
	})

	return []map[string]any{
		a2ui.CreateSurface(surfaceID, false),
		a2ui.UpdateComponents(surfaceID, comps),
	}
}

// BuildCartSurface builds an interactive A2UI v1.0 cart summary card with a "Confirm Order" button.
func BuildCartSurface(cart map[string]CartItem) []map[string]any {
	surfaceID := "cart-surface"
	rowIDs := make([]string, 0, len(cart))
	var total float64
	comps := []map[string]any{
		{"id": "root", "component": "Card", "child": "cart_col"},
		{"id": "cart_header", "component": "Text", "text": "🛒 Durable Server-Side Cart", "variant": "h3"},
	}

	idx := 0
	for _, item := range cart {
		sub := item.Price * float64(item.Qty)
		total += sub
		rowID := fmt.Sprintf("cart_row_%d", idx)
		nameID := fmt.Sprintf("cart_name_%d", idx)
		priceID := fmt.Sprintf("cart_price_%d", idx)
		rowIDs = append(rowIDs, rowID)
		comps = append(comps,
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
				"text":      fmt.Sprintf("$%.2f", sub),
				"variant":   "body",
			},
		)
		idx++
	}

	colChildren := []string{"cart_header"}
	if len(rowIDs) == 0 {
		colChildren = append(colChildren, "cart_empty")
		comps = append(comps, map[string]any{
			"id":        "cart_empty",
			"component": "Text",
			"text":      "Your cart is currently empty.",
			"variant":   "caption",
		})
	} else {
		colChildren = append(colChildren, rowIDs...)
		colChildren = append(colChildren, "cart_div", "cart_total", "cart_actions")
		comps = append(comps,
			map[string]any{"id": "cart_div", "component": "Divider"},
			map[string]any{
				"id":        "cart_total",
				"component": "Text",
				"text":      fmt.Sprintf("Total: $%.2f", total),
				"variant":   "h4",
			},
			map[string]any{
				"id":        "cart_actions",
				"component": "Row",
				"children":  []string{"safety_btn", "confirm_btn"},
			},
			map[string]any{
				"id":        "safety_btn",
				"component": "Button",
				"child":     "safety_btn_lbl",
				"variant":   "borderless",
				"action":    a2ui.EventAction("Check dietary safety for my cart", nil),
			},
			map[string]any{
				"id":        "safety_btn_lbl",
				"component": "Text",
				"text":      "🛡️ Verify Dietary Safety (A2A)",
			},
			map[string]any{
				"id":        "confirm_btn",
				"component": "Button",
				"child":     "confirm_btn_lbl",
				"action":    a2ui.EventAction("Confirm my order!", nil),
			},
			map[string]any{
				"id":        "confirm_btn_lbl",
				"component": "Text",
				"text":      "✅ Confirm Order",
			},
		)
	}

	comps = append(comps, map[string]any{
		"id":        "cart_col",
		"component": "Column",
		"children":  colChildren,
	})

	return []map[string]any{
		a2ui.CreateSurface(surfaceID, false),
		a2ui.UpdateComponents(surfaceID, comps),
	}
}

// BuildDietarySurface builds an A2UI v1.0 card showing the remote A2A Dietary Advisor report.
func BuildDietarySurface(summary string) []map[string]any {
	surfaceID := "dietary-report"
	comps := []map[string]any{
		{"id": "root", "component": "Card", "child": "diet_col"},
		{
			"id":        "diet_col",
			"component": "Column",
			"children":  []string{"diet_hdr", "diet_badge", "diet_body", "diet_btn"},
		},
		{"id": "diet_hdr", "component": "Text", "text": "🛡️ A2A Dietary & Allergy Safety Report (:9002)", "variant": "h3"},
		{"id": "diet_badge", "component": "Text", "text": "🟢 VERIFIED VIA REMOTE A2A JSON-RPC 2.0 AGENT", "variant": "caption"},
		{"id": "diet_body", "component": "Text", "text": summary, "variant": "body"},
		{
			"id":        "diet_btn",
			"component": "Button",
			"child":     "diet_btn_lbl",
			"action":    a2ui.EventAction("Confirm my order!", nil),
		},
		{"id": "diet_btn_lbl", "component": "Text", "text": "✅ Proceed to Confirm Order"},
	}
	return []map[string]any{
		a2ui.CreateSurface(surfaceID, false),
		a2ui.UpdateComponents(surfaceID, comps),
	}
}

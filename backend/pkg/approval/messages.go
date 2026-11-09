package approval

import (
	"fmt"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

const (
	RequesterSurface = "requester"
	ApproverSurface  = "approver"
)

func statusText(status string) string {
	switch status {
	case "submitted", "under_review":
		return "🟡 Pending Review"
	case "approved":
		return "🟢 Approved"
	case "rejected":
		return "🔴 Rejected"
	default:
		return status
	}
}

func CreateRequesterSurface() []map[string]any {
	return []map[string]any{a2ui.CreateSurface(RequesterSurface, true)}
}

func BuildRequesterComponents(state ApprovalState) []map[string]any {
	itemRowIDs := make([]string, 0, len(state.Items))
	itemComponents := make([]map[string]any, 0, len(state.Items)*4)

	for i, item := range state.Items {
		rowID := fmt.Sprintf("req_item_row_%d", i)
		descID := fmt.Sprintf("req_item_desc_%d", i)
		catID := fmt.Sprintf("req_item_cat_%d", i)
		amountID := fmt.Sprintf("req_item_amount_%d", i)
		itemRowIDs = append(itemRowIDs, rowID)
		itemComponents = append(itemComponents,
			map[string]any{
				"id":        rowID,
				"component": "Row",
				"children":  []string{descID, catID, amountID},
				"justify":   "spaceBetween",
			},
			map[string]any{"id": descID, "component": "Text", "text": item.Description, "variant": "body"},
			map[string]any{"id": catID, "component": "Text", "text": item.Category, "variant": "caption"},
			map[string]any{"id": amountID, "component": "Text", "text": fmt.Sprintf("$%.2f", item.Amount), "variant": "body"},
		)
	}

	itemsColChildren := []string{"items_header"}
	itemsColChildren = append(itemsColChildren, itemRowIDs...)
	itemsColChildren = append(itemsColChildren, "divider_1", "total_text", "justification_header", "justification_text")

	components := []map[string]any{
		{"id": "root", "component": "Column", "children": []string{"status_card", "items_card"}},
		{"id": "status_card", "component": "Card", "child": "status_column"},
		{"id": "status_column", "component": "Column", "children": []string{"status_header", "status_badge", "submitted_by"}},
		{"id": "status_header", "component": "Text", "text": a2ui.Path("/title"), "variant": "h3"},
		{"id": "status_badge", "component": "Text", "text": a2ui.Path("/status_text"), "variant": "body"},
		{"id": "submitted_by", "component": "Text", "text": a2ui.Path("/submitted_by"), "variant": "caption"},
		{"id": "items_card", "component": "Card", "child": "items_column"},
		{"id": "items_column", "component": "Column", "children": itemsColChildren},
		{"id": "items_header", "component": "Text", "text": "Expense Items", "variant": "h4"},
	}
	components = append(components, itemComponents...)
	components = append(components,
		map[string]any{"id": "divider_1", "component": "Divider"},
		map[string]any{"id": "total_text", "component": "Text", "text": a2ui.Path("/total"), "variant": "h4"},
		map[string]any{"id": "justification_header", "component": "Text", "text": "Justification", "variant": "caption"},
		map[string]any{"id": "justification_text", "component": "Text", "text": a2ui.Path("/justification"), "variant": "body"},
	)

	return []map[string]any{a2ui.UpdateComponents(RequesterSurface, components)}
}

func BuildRequesterData(state ApprovalState) []map[string]any {
	itemsData := make([]map[string]any, 0, len(state.Items))
	for _, item := range state.Items {
		itemsData = append(itemsData, map[string]any{
			"description": item.Description,
			"amount":      fmt.Sprintf("$%.2f", item.Amount),
			"category":    item.Category,
		})
	}
	return []map[string]any{
		a2ui.UpdateDataModel(RequesterSurface, "", map[string]any{
			"title":         fmt.Sprintf("Expense #%s — %s", state.RequestID, state.Title),
			"status_text":   statusText(state.Status),
			"submitted_by":  fmt.Sprintf("Submitted by %s", state.Requester),
			"total":         fmt.Sprintf("Total: $%.2f", state.Total),
			"justification": state.Justification,
			"items":         itemsData,
		}),
	}
}

func UpdateRequesterStatus(state ApprovalState) []map[string]any {
	return []map[string]any{
		a2ui.UpdateDataModel(RequesterSurface, "", map[string]any{
			"title":       fmt.Sprintf("Expense #%s — %s", state.RequestID, state.Title),
			"status_text": statusText(state.Status),
		}),
	}
}

func CreateApproverSurface() []map[string]any {
	return []map[string]any{a2ui.CreateSurface(ApproverSurface, true)}
}

func BuildApproverComponents(state ApprovalState) []map[string]any {
	itemRowIDs := make([]string, 0, len(state.Items))
	itemComponents := make([]map[string]any, 0, len(state.Items)*4)

	for i, item := range state.Items {
		rowID := fmt.Sprintf("appr_item_row_%d", i)
		descID := fmt.Sprintf("appr_item_desc_%d", i)
		catID := fmt.Sprintf("appr_item_cat_%d", i)
		amountID := fmt.Sprintf("appr_item_amount_%d", i)
		itemRowIDs = append(itemRowIDs, rowID)
		itemComponents = append(itemComponents,
			map[string]any{
				"id":        rowID,
				"component": "Row",
				"children":  []string{descID, catID, amountID},
				"justify":   "spaceBetween",
			},
			map[string]any{"id": descID, "component": "Text", "text": item.Description, "variant": "body"},
			map[string]any{"id": catID, "component": "Text", "text": item.Category, "variant": "caption"},
			map[string]any{"id": amountID, "component": "Text", "text": fmt.Sprintf("$%.2f", item.Amount), "variant": "body"},
		)
	}

	apprItemsChildren := []string{"appr_items_header"}
	apprItemsChildren = append(apprItemsChildren, itemRowIDs...)
	apprItemsChildren = append(apprItemsChildren, "appr_divider", "appr_total_text")

	components := []map[string]any{
		{"id": "root", "component": "Column", "children": []string{"request_card", "items_card", "actions"}},
		{"id": "request_card", "component": "Card", "child": "request_column"},
		{"id": "request_column", "component": "Column", "children": []string{"request_header", "requester_info", "justification_section"}},
		{"id": "request_header", "component": "Text", "text": a2ui.Path("/title"), "variant": "h3"},
		{"id": "requester_info", "component": "Text", "text": a2ui.Path("/requester_info"), "variant": "body"},
		{"id": "justification_section", "component": "Column", "children": []string{"just_label", "just_text"}},
		{"id": "just_label", "component": "Text", "text": "Justification", "variant": "caption"},
		{"id": "just_text", "component": "Text", "text": a2ui.Path("/justification"), "variant": "body"},
		{"id": "items_card", "component": "Card", "child": "appr_items_column"},
		{"id": "appr_items_column", "component": "Column", "children": apprItemsChildren},
		{"id": "appr_items_header", "component": "Text", "text": "Expense Items", "variant": "h4"},
	}
	components = append(components, itemComponents...)
	components = append(components,
		map[string]any{"id": "appr_divider", "component": "Divider"},
		map[string]any{"id": "appr_total_text", "component": "Text", "text": a2ui.Path("/total"), "variant": "h4"},
		map[string]any{"id": "actions", "component": "Row", "children": []string{"approve_btn", "reject_btn"}},
		map[string]any{
			"id":        "approve_btn",
			"component": "Button",
			"child":     "approve_btn_label",
			"action":    a2ui.EventAction("approve", nil),
		},
		map[string]any{"id": "approve_btn_label", "component": "Text", "text": "✅ Approve"},
		map[string]any{
			"id":        "reject_btn",
			"component": "Button",
			"child":     "reject_btn_label",
			"variant":   "borderless",
			"action":    a2ui.EventAction("reject", nil),
		},
		map[string]any{"id": "reject_btn_label", "component": "Text", "text": "❌ Reject"},
	)

	return []map[string]any{a2ui.UpdateComponents(ApproverSurface, components)}
}

func BuildApproverData(state ApprovalState) []map[string]any {
	itemsData := make([]map[string]any, 0, len(state.Items))
	for _, item := range state.Items {
		itemsData = append(itemsData, map[string]any{
			"description": item.Description,
			"amount":      fmt.Sprintf("$%.2f", item.Amount),
			"category":    item.Category,
		})
	}
	return []map[string]any{
		a2ui.UpdateDataModel(ApproverSurface, "", map[string]any{
			"title":          fmt.Sprintf("Expense #%s — %s", state.RequestID, state.Title),
			"requester_info": fmt.Sprintf("Submitted by: %s", state.Requester),
			"justification":  state.Justification,
			"total":          fmt.Sprintf("Total: $%.2f", state.Total),
			"items":          itemsData,
		}),
	}
}

func UpdateApproverDecision(state ApprovalState) []map[string]any {
	decisionText := "❌ Rejected"
	if state.Status == "approved" {
		decisionText = "✅ Approved"
	}

	return []map[string]any{
		a2ui.UpdateComponents(ApproverSurface, []map[string]any{
			{"id": "actions", "component": "Column", "children": []string{"decision_summary"}},
			{"id": "decision_summary", "component": "Column", "children": []string{"decision_text", "decision_note"}},
			{"id": "decision_text", "component": "Text", "text": decisionText, "variant": "h4"},
			{"id": "decision_note", "component": "Text", "text": state.DecisionNote, "variant": "caption"},
		}),
		a2ui.UpdateDataModel(ApproverSurface, "", map[string]any{
			"title": fmt.Sprintf("Expense #%s — %s (%s)", state.RequestID, state.Title, decisionText),
		}),
	}
}

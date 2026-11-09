package a2aservers

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

const (
	RestaurantFinderPort  = 9001
	DietaryAdvisorPort    = 9002
	TelemetryDiagPort     = 9003
	SecOpsPolicyPort      = 9004
)

type handlerFunc func(prompt string) string

type funcExecutor struct {
	fn handlerFunc
}

var _ a2asrv.AgentExecutor = (*funcExecutor)(nil)

func (e *funcExecutor) Execute(_ context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		var prompt strings.Builder
		if execCtx != nil && execCtx.Message != nil {
			for _, p := range execCtx.Message.Parts {
				if t := p.Text(); t != "" {
					prompt.WriteString(t)
				}
			}
		}
		reply := e.fn(prompt.String())
		msg := a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart(reply))
		yield(msg, nil)
	}
}

func (e *funcExecutor) Cancel(_ context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCanceled, nil), nil)
	}
}

func NewServerMux(card *a2a.AgentCard, fn handlerFunc) *http.ServeMux {
	executor := &funcExecutor{fn: fn}
	requestHandler := a2asrv.NewHandler(executor)
	mux := http.NewServeMux()
	mux.Handle("/invoke", a2asrv.NewJSONRPCHandler(requestHandler))
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(card))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return mux
}

// CallA2AAgent resolves the remote A2A AgentCard over HTTP and invokes SendMessage via JSON-RPC 2.0.
func CallA2AAgent(ctx context.Context, baseURL string, prompt string) (string, error) {
	card, err := agentcard.DefaultResolver.Resolve(ctx, baseURL)
	if err != nil {
		return "", fmt.Errorf("resolve agent card at %s: %w", baseURL, err)
	}
	client, err := a2aclient.NewFromCard(ctx, card)
	if err != nil {
		return "", fmt.Errorf("create a2a client for %s: %w", baseURL, err)
	}
	resp, err := client.SendMessage(ctx, &a2a.SendMessageRequest{
		Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(prompt)),
	})
	if err != nil {
		return "", fmt.Errorf("send a2a message to %s: %w", baseURL, err)
	}
	switch v := resp.(type) {
	case *a2a.Message:
		var sb strings.Builder
		for _, p := range v.Parts {
			sb.WriteString(p.Text())
		}
		return sb.String(), nil
	case *a2a.Task:
		var sb strings.Builder
		for _, a := range v.Artifacts {
			for _, p := range a.Parts {
				sb.WriteString(p.Text())
			}
		}
		if sb.Len() == 0 && v.Status.Message != nil {
			for _, p := range v.Status.Message.Parts {
				sb.WriteString(p.Text())
			}
		}
		return sb.String(), nil
	default:
		b, _ := json.Marshal(resp)
		return string(b), nil
	}
}

// ---------------------------------------------------------------------------
// 1. Restaurant & Campus Spot Finder A2A Agent (:9001)
// ---------------------------------------------------------------------------

type RestaurantInfo struct {
	Name         string   `json:"name"`
	Cuisine      string   `json:"cuisine"`
	Rating       float64  `json:"rating"`
	PriceRange   string   `json:"price_range"`
	DeliveryTime string   `json:"delivery_time"`
	Highlights   []string `json:"highlights"`
	Dietary      []string `json:"dietary_options"`
	Badge        string   `json:"badge"`
}

var RestaurantsCatalog = []RestaurantInfo{
	{
		Name:         "Taquería Orinoco CDMX",
		Cuisine:      "Mexican Street Tacos (Roma Norte / Condesa)",
		Rating:       4.9,
		PriceRange:   "$",
		DeliveryTime: "15 min",
		Highlights:   []string{"Tacos al Pastor", "Gringa de Pastor", "Horchata Artesanal"},
		Dietary:      []string{"gluten-free corn tortillas", "dairy-free options"},
		Badge:        "🌮 CDMX DevFest Favorite",
	},
	{
		Name:         "Boba & Byte Campus Lab",
		Cuisine:      "Campus Late-Night Cafe & Bubble Tea",
		Rating:       4.8,
		PriceRange:   "$",
		DeliveryTime: "10 min",
		Highlights:   []string{"Brown Sugar Boba", "Margherita Pizza", "Sparkling Water"},
		Dietary:      []string{"vegetarian", "oat-milk vegan option"},
		Badge:        "🎓 Student Hackathon Fuel",
	},
	{
		Name:         "Bella Napoli",
		Cuisine:      "Wood-Fired Italian Pizza & Dolci",
		Rating:       4.7,
		PriceRange:   "$$",
		DeliveryTime: "25 min",
		Highlights:   []string{"Margherita Pizza", "Pepperoni Pizza", "Tiramisu"},
		Dietary:      []string{"vegetarian", "gluten-free crust available"},
		Badge:        "🍕 Wood-Fired Classic",
	},
	{
		Name:         "Green Garden",
		Cuisine:      "Fresh Salads & Mediterranean Bowls",
		Rating:       4.8,
		PriceRange:   "$$",
		DeliveryTime: "20 min",
		Highlights:   []string{"Caesar Salad", "Sparkling Water"},
		Dietary:      []string{"vegetarian", "low-carb", "nut-free"},
		Badge:        "🥗 Healthy Choice",
	},
}

func RestaurantFinderServer(port int) *http.ServeMux {
	card := &a2a.AgentCard{
		Name:               "Restaurant & Campus Spot Finder Agent",
		Description:        "Discovers top local restaurants across CDMX (DevFest) and University Campus spots by cuisine, dietary needs, and delivery speed.",
		Version:            "1.0.0",
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		SupportedInterfaces: []*a2a.AgentInterface{
			a2a.NewAgentInterface(fmt.Sprintf("http://127.0.0.1:%d/invoke", port), a2a.TransportProtocolJSONRPC),
		},
		Skills: []a2a.AgentSkill{
			{
				ID:          "find_restaurants",
				Name:        "Find Restaurants & Campus Spots",
				Description: "Search CDMX and campus dining spots with ratings, ETAs, and signature dishes.",
				Tags:        []string{"restaurants", "cdmx", "campus", "food"},
			},
		},
	}

	return NewServerMux(card, func(prompt string) string {
		q := strings.ToLower(prompt)
		var matches []RestaurantInfo
		for _, r := range RestaurantsCatalog {
			if strings.Contains(q, "taco") || strings.Contains(q, "mexic") || strings.Contains(q, "cdmx") || strings.Contains(q, "pastor") {
				if strings.Contains(strings.ToLower(r.Name), "orinoco") {
					matches = append(matches, r)
				}
			} else if strings.Contains(q, "boba") || strings.Contains(q, "campus") || strings.Contains(q, "hack") || strings.Contains(q, "study") {
				if strings.Contains(strings.ToLower(r.Name), "boba") || strings.Contains(strings.ToLower(r.Name), "napoli") {
					matches = append(matches, r)
				}
			} else if strings.Contains(q, "pizza") || strings.Contains(q, "italian") {
				if strings.Contains(strings.ToLower(r.Name), "napoli") {
					matches = append(matches, r)
				}
			} else if strings.Contains(q, "salad") || strings.Contains(q, "healthy") || strings.Contains(q, "vegan") {
				if strings.Contains(strings.ToLower(r.Name), "green") {
					matches = append(matches, r)
				}
			}
		}
		if len(matches) == 0 {
			matches = RestaurantsCatalog
		}
		payload := map[string]any{
			"agent":       "restaurant_finder_a2a",
			"protocol":    "A2A JSON-RPC 2.0",
			"query":       prompt,
			"restaurants": matches,
		}
		b, _ := json.Marshal(payload)
		return string(b)
	})
}

// ---------------------------------------------------------------------------
// 2. Dietary & Allergy Safety Advisor A2A Agent (:9002)
// ---------------------------------------------------------------------------

type ItemDietaryInfo struct {
	Allergens   []string `json:"allergens"`
	DietaryTags []string `json:"dietary_tags"`
	Calories    int      `json:"calories"`
	Notes       string   `json:"notes"`
}

var DietaryCatalog = map[string]ItemDietaryInfo{
	"Tacos al Pastor": {
		Allergens:   []string{},
		DietaryTags: []string{"gluten-free (100% nixtamal corn tortilla)", "dairy-free", "nut-free"},
		Calories:    480,
		Notes:       "Char-grilled adobo pork with roasted pineapple, cilantro, and onion on corn tortillas.",
	},
	"Gringa de Pastor": {
		Allergens:   []string{"gluten (flour tortilla)", "dairy (Oaxaca cheese)"},
		DietaryTags: []string{"nut-free"},
		Calories:    640,
		Notes:       "Melted queso Oaxaca and al pastor pork on grilled flour tortillas.",
	},
	"Horchata Artesanal": {
		Allergens:   []string{"dairy (condensed milk)"},
		DietaryTags: []string{"vegetarian", "gluten-free", "nut-free (made with rice & cinnamon, no almonds)"},
		Calories:    210,
		Notes:       "House-made rice water with Ceylon cinnamon and vanilla.",
	},
	"Brown Sugar Boba": {
		Allergens:   []string{},
		DietaryTags: []string{"vegetarian", "vegan (oat milk default)", "gluten-free", "nut-free"},
		Calories:    290,
		Notes:       "Warm tapioca pearls with caramelized Okinawa brown sugar and oat milk.",
	},
	"Margherita Pizza": {
		Allergens:   []string{"gluten", "dairy"},
		DietaryTags: []string{"vegetarian", "nut-free"},
		Calories:    780,
		Notes:       "San Marzano tomato, fresh mozzarella, sweet basil.",
	},
	"Pepperoni Pizza": {
		Allergens:   []string{"gluten", "dairy"},
		DietaryTags: []string{"nut-free"},
		Calories:    920,
		Notes:       "Crispy pepperoni, mozzarella, pickled jalapeños.",
	},
	"Caesar Salad": {
		Allergens:   []string{"gluten (croutons)", "dairy (parmesan)", "egg", "fish (anchovy in dressing)"},
		DietaryTags: []string{"nut-free"},
		Calories:    360,
		Notes:       "Can be made gluten-free without croutons and vegetarian with vegan dressing.",
	},
	"Sparkling Water": {
		Allergens:   []string{},
		DietaryTags: []string{"vegan", "vegetarian", "gluten-free", "dairy-free", "nut-free", "zero-calorie"},
		Calories:    0,
		Notes:       "Chilled San Pellegrino 500ml.",
	},
	"Tiramisu": {
		Allergens:   []string{"gluten", "dairy (mascarpone)", "egg", "caffeine"},
		DietaryTags: []string{"vegetarian", "nut-free"},
		Calories:    450,
		Notes:       "Espresso-soaked Savoiardi ladyfingers with mascarpone cream.",
	},
}

func DietaryAdvisorServer(port int) *http.ServeMux {
	card := &a2a.AgentCard{
		Name:               "Dietary & Allergy Safety Advisor Agent",
		Description:        "Analyzes food items for allergens, dietary compatibility, and nutritional safety before order confirmation.",
		Version:            "1.0.0",
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		SupportedInterfaces: []*a2a.AgentInterface{
			a2a.NewAgentInterface(fmt.Sprintf("http://127.0.0.1:%d/invoke", port), a2a.TransportProtocolJSONRPC),
		},
		Skills: []a2a.AgentSkill{
			{
				ID:          "check_dietary_safety",
				Name:        "Check Dietary & Allergy Safety",
				Description: "Verifies menu items against customer dietary restrictions and allergens.",
				Tags:        []string{"dietary", "allergens", "nutrition", "safety"},
			},
		},
	}

	return NewServerMux(card, func(prompt string) string {
		q := strings.ToLower(prompt)
		results := make([]map[string]any, 0)
		for name, info := range DietaryCatalog {
			if strings.Contains(q, strings.ToLower(name)) ||
				(strings.Contains(q, "taco") && strings.Contains(name, "Tacos")) ||
				(strings.Contains(q, "gringa") && strings.Contains(name, "Gringa")) ||
				(strings.Contains(q, "horchata") && strings.Contains(name, "Horchata")) ||
				(strings.Contains(q, "boba") && strings.Contains(name, "Boba")) ||
				(strings.Contains(q, "margherita") && strings.Contains(name, "Margherita")) ||
				(strings.Contains(q, "pepperoni") && strings.Contains(name, "Pepperoni")) ||
				(strings.Contains(q, "salad") && strings.Contains(name, "Salad")) ||
				(strings.Contains(q, "water") && strings.Contains(name, "Water")) ||
				(strings.Contains(q, "tiramisu") && strings.Contains(name, "Tiramisu")) {
				status := "SAFE"
				warning := ""
				if strings.Contains(q, "vegan") && !containsSubstr(info.DietaryTags, "vegan") {
					status = "CAUTION"
					warning = "Contains animal/dairy products; ask for vegan modification."
				} else if strings.Contains(q, "gluten") && containsSubstr(info.Allergens, "gluten") {
					status = "CAUTION"
					warning = "Contains gluten; request corn tortilla or GF crust."
				} else if strings.Contains(q, "vegetarian") && (strings.Contains(name, "Pastor") || strings.Contains(name, "Pepperoni")) {
					status = "UNSAFE"
					warning = "Contains pork/meat; swap for Margherita Pizza or Brown Sugar Boba."
				}
				results = append(results, map[string]any{
					"item":         name,
					"status":       status,
					"allergens":    info.Allergens,
					"dietary_tags": info.DietaryTags,
					"calories":     info.Calories,
					"notes":        info.Notes,
					"warning":      warning,
				})
			}
		}
		if len(results) == 0 {
			for name, info := range DietaryCatalog {
				results = append(results, map[string]any{
					"item":         name,
					"status":       "SAFE",
					"allergens":    info.Allergens,
					"dietary_tags": info.DietaryTags,
					"calories":     info.Calories,
					"notes":        info.Notes,
				})
			}
		}
		payload := map[string]any{
			"agent":           "dietary_safety_advisor_a2a",
			"protocol":        "A2A JSON-RPC 2.0",
			"overall_status":  "VERIFIED",
			"checked_items":   results,
			"recommendation":  "All items verified against allergen database. Nut-free kitchen certified.",
		}
		b, _ := json.Marshal(payload)
		return string(b)
	})
}

func containsSubstr(slice []string, sub string) bool {
	for _, s := range slice {
		if strings.Contains(strings.ToLower(s), strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 3. CloudGuard Telemetry & Canary Diagnostics A2A Agent (:9003)
// ---------------------------------------------------------------------------

func TelemetryDiagnosticsServer(port int) *http.ServeMux {
	card := &a2a.AgentCard{
		Name:               "CloudGuard Telemetry & Canary Diagnostics Agent",
		Description:        "Correlates Prometheus/OpenTelemetry metrics, GKE pod logs, and canary deployment diffs to isolate production root causes.",
		Version:            "1.0.0",
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		SupportedInterfaces: []*a2a.AgentInterface{
			a2a.NewAgentInterface(fmt.Sprintf("http://127.0.0.1:%d/invoke", port), a2a.TransportProtocolJSONRPC),
		},
		Skills: []a2a.AgentSkill{
			{
				ID:          "diagnose_service_telemetry",
				Name:        "Diagnose Service Telemetry & Canary Diff",
				Description: "Analyzes golden signals (latency, error rate, saturation) and canary build changes.",
				Tags:        []string{"sre", "telemetry", "gke", "canary", "observability"},
			},
		},
	}

	return NewServerMux(card, func(prompt string) string {
		payload := map[string]any{
			"agent":              "cloudguard_telemetry_a2a",
			"protocol":           "A2A JSON-RPC 2.0",
			"service":            "checkout-api",
			"region":             "us-central1 / mx-central1 (GKE Multi-Cluster)",
			"anomalous_revision": "checkout-api-canary-v2.14.0-rc3",
			"baseline_revision":  "checkout-api-stable-v2.13.8",
			"metrics": map[string]any{
				"http_5xx_rate":         "18.4% (baseline: 0.02%)",
				"p99_latency_ms":        4820,
				"alloydb_conn_pool_use": "200/200 (100% SATURATED)",
				"affected_rps":          14200,
			},
			"root_cause":          "Canary v2.14.0-rc3 introduced an unclosed pgx transaction inside retry middleware when payment webhook times out, exhausting the 200-connection AlloyDB pool in 90 seconds.",
			"confidence":          0.96,
			"recommended_action":  "rollback_canary",
			"target_revision":     "checkout-api-stable-v2.13.8",
			"secondary_mitigation": "drain_leaked_db_sessions",
		}
		b, _ := json.Marshal(payload)
		return string(b)
	})
}

// ---------------------------------------------------------------------------
// 4. SecOps Blast-Radius & Change-Freeze Policy A2A Agent (:9004)
// ---------------------------------------------------------------------------

func SecOpsPolicyServer(port int) *http.ServeMux {
	card := &a2a.AgentCard{
		Name:               "SecOps Blast-Radius & Change-Freeze Policy Agent",
		Description:        "Evaluates production remediation plans against SRE error-budget burn rates, blast radius, and change-freeze governance.",
		Version:            "1.0.0",
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		SupportedInterfaces: []*a2a.AgentInterface{
			a2a.NewAgentInterface(fmt.Sprintf("http://127.0.0.1:%d/invoke", port), a2a.TransportProtocolJSONRPC),
		},
		Skills: []a2a.AgentSkill{
			{
				ID:          "evaluate_blast_radius",
				Name:        "Evaluate Blast Radius & Governance Policy",
				Description: "Checks whether an automated remediation action is safe and whether Commander HITL sign-off is required.",
				Tags:        []string{"secops", "policy", "blast-radius", "hitl"},
			},
		},
	}

	return NewServerMux(card, func(prompt string) string {
		payload := map[string]any{
			"agent":                   "secops_policy_a2a",
			"protocol":                "A2A JSON-RPC 2.0",
			"policy_check":            "PASS_WITH_HITL_GATE",
			"risk_score":              "HIGH (Tier-0 Checkout Service)",
			"change_freeze_active":    true,
			"change_freeze_reason":    "Peak Event Window — DevFest CDMX & Campus Registration Live Traffic",
			"affected_users_estimate": "14,200 req/s across mx-central1 & us-central1",
			"slo_burn_rate":           "18.4x (Monthly 99.95% error budget exhausts in 24 minutes)",
			"requires_human_approval": true,
			"required_approver_role":  "Incident Commander (On-Call SRE Lead)",
			"approved_runbook":        "RB-402: Zero-Downtime Canary Traffic Shift (100% -> v2.13.8) + AlloyDB Idle Tx Termination",
		}
		b, _ := json.Marshal(payload)
		return string(b)
	})
}

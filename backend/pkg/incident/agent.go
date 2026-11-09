package incident

import (
	"context"
	"fmt"
	"iter"
	"os"
	"strings"
	"time"

	"go.temporal.io/sdk/contrib/googleadk"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/genai"
)

const ModelName = "gemini-2.5-flash-sre"

type ExecuteRemediationArgs struct {
	RunbookID      string `json:"runbook_id"`
	TargetRevision string `json:"target_revision"`
}

type ExecuteRemediationResult struct {
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Runbook string `json:"runbook,omitempty"`
	Message string `json:"message"`
}

type IncidentToolHooks struct {
	OnExecuteRemediation func(args ExecuteRemediationArgs) (ExecuteRemediationResult, error)
}

func CreateIncidentAgent(hooks *IncidentToolHooks) (agent.Agent, error) {
	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}

	a2aTelemetryTool, err := googleadk.ActivityAsTool(A2ADiagnoseTelemetryActivity, googleadk.ActivityToolOptions{
		Name:            "a2a_diagnose_telemetry",
		Description:     "Correlate Prometheus/OTel metrics and GKE canary diffs via remote CloudGuard Telemetry A2A Agent (:9003).",
		ActivityOptions: actOpts,
	})
	if err != nil {
		return nil, err
	}

	a2aSecOpsTool, err := googleadk.ActivityAsTool(A2AEvaluateBlastRadiusActivity, googleadk.ActivityToolOptions{
		Name:            "a2a_evaluate_blast_radius",
		Description:     "Evaluate blast radius, SLO burn rate, and change-freeze policy via remote SecOps Policy A2A Agent (:9004).",
		ActivityOptions: actOpts,
	})
	if err != nil {
		return nil, err
	}

	execRemediationTool, err := functiontool.New(functiontool.Config{
		Name:        "execute_remediation",
		Description: "Execute production GKE canary rollback (RB-402) and drain leaked AlloyDB sessions after SecOps & Commander HITL sign-off.",
	}, func(_ agent.Context, args ExecuteRemediationArgs) (ExecuteRemediationResult, error) {
		return hooks.OnExecuteRemediation(args)
	})
	if err != nil {
		return nil, err
	}

	m := googleadk.NewModel(ModelName)

	telemetryInvestigator, err := llmagent.New(llmagent.Config{
		Name:        "telemetry_investigator",
		Description: "Correlates golden signals, AlloyDB pool metrics, and GKE canary deployments via A2A (:9003).",
		Instruction: "INCIDENT_ROLE:telemetry_investigator\nYou are the Telemetry Investigator. Call a2a_diagnose_telemetry to isolate the root cause.",
		Model:       m,
		Tools:       []tool.Tool{a2aTelemetryTool},
	})
	if err != nil {
		return nil, err
	}

	mitigationPlanner, err := llmagent.New(llmagent.Config{
		Name:        "mitigation_planner",
		Description: "Evaluates SecOps blast radius, change-freeze windows, and prepares runbook RB-402 via A2A (:9004).",
		Instruction: "INCIDENT_ROLE:mitigation_planner\nYou are the Mitigation Planner. Call a2a_evaluate_blast_radius to validate the remediation plan.",
		Model:       m,
		Tools:       []tool.Tool{a2aSecOpsTool},
	})
	if err != nil {
		return nil, err
	}

	remediationExecutor, err := llmagent.New(llmagent.Config{
		Name:        "remediation_executor",
		Description: "Executes production canary rollback RB-402 after SecOps policy check and Incident Commander HITL approval.",
		Instruction: "INCIDENT_ROLE:remediation_executor\nYou are the Remediation Executor. Ensure SecOps policy was checked via a2a_evaluate_blast_radius and call execute_remediation.",
		Model:       m,
		Tools:       []tool.Tool{execRemediationTool, a2aSecOpsTool},
	})
	if err != nil {
		return nil, err
	}

	root, err := llmagent.New(llmagent.Config{
		Name:        "incident_commander",
		Description: "Root AI SRE Incident Commander orchestrating telemetry_investigator, mitigation_planner, and remediation_executor.",
		Instruction: "INCIDENT_ROLE:incident_commander\nYou are CloudGuard AI SRE Incident Commander. Route telemetry/root-cause analysis to telemetry_investigator, blast-radius/policy checks to mitigation_planner, and rollback execution to remediation_executor.",
		Model:       m,
		SubAgents:   []agent.Agent{telemetryInvestigator, mitigationPlanner, remediationExecutor},
	})
	if err != nil {
		return nil, err
	}

	return root, nil
}

func NewResilientSREModelFactory() googleadk.ModelFactory {
	return func(ctx context.Context, name string) (model.LLM, error) {
		fallback := &sreDemoLLM{name: name}
		apiKey := os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("GEMINI_API_KEY")
		}
		if apiKey != "" && strings.ToLower(os.Getenv("OFFLINE_MODE")) != "true" {
			live, err := gemini.NewModel(ctx, "gemini-2.5-flash", nil)
			if err == nil {
				return &sreHybridLLM{name: name, live: live, fallback: fallback}, nil
			}
		}
		return fallback, nil
	}
}

type sreHybridLLM struct {
	name     string
	live     model.LLM
	fallback model.LLM
}

func (h *sreHybridLLM) Name() string { return h.name }

func (h *sreHybridLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		var emitted bool
		for resp, err := range h.live.GenerateContent(ctx, req, stream) {
			if err != nil {
				if !emitted {
					for fResp, fErr := range h.fallback.GenerateContent(ctx, req, stream) {
						if !yield(fResp, fErr) {
							return
						}
					}
				}
				return
			}
			emitted = true
			if !yield(resp, nil) {
				return
			}
		}
	}
}

type sreDemoLLM struct {
	name string
}

func (s *sreDemoLLM) Name() string { return s.name }

func (s *sreDemoLLM) GenerateContent(_ context.Context, req *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		resp := s.decide(req)
		resp.TurnComplete = true
		resp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     240,
			CandidatesTokenCount: 115,
			TotalTokenCount:      355,
		}
		yield(resp, nil)
	}
}

func (s *sreDemoLLM) decide(req *model.LLMRequest) *model.LLMResponse {
	sys := extractSRESystem(req)
	lastUser := extractSRELastUser(req)
	lastFn, lastResp := extractSRELastFn(req)
	lower := strings.ToLower(lastUser)

	role := "incident_commander"
	if strings.Contains(sys, "INCIDENT_ROLE:telemetry_investigator") {
		role = "telemetry_investigator"
	} else if strings.Contains(sys, "INCIDENT_ROLE:mitigation_planner") {
		role = "mitigation_planner"
	} else if strings.Contains(sys, "INCIDENT_ROLE:remediation_executor") {
		role = "remediation_executor"
	}

	if lastFn != "" && lastFn != "transfer_to_agent" {
		switch lastFn {
		case "a2a_diagnose_telemetry":
			return sreText("📡 **A2A Telemetry Diagnosis Complete (`:9003`):** Isolated root cause with **96% confidence** to `checkout-api-canary-v2.14.0-rc3` in `mx-central1` & `us-central1` (unclosed `pgx` transaction in payment retry middleware exhausting the 200-connection AlloyDB pool). Click **🛡️ Step 2: Evaluate Blast Radius & SecOps Policy (A2A) →** on the card below!")
		case "a2a_evaluate_blast_radius":
			if role == "remediation_executor" && (strings.Contains(lower, "execute") || strings.Contains(lower, "rollback") || strings.Contains(lower, "rb-402")) {
				return sreFn("execute_remediation", map[string]any{
					"runbook_id":      "RB-402",
					"target_revision": "checkout-api-stable-v2.13.8",
				})
			}
			return sreText("🛡️ **A2A SecOps Blast-Radius & Change-Freeze Check Complete (`:9004`):** Policy returned `PASS_WITH_HITL_GATE`. Because `checkout-api` is Tier-0 during the **DevFest CDMX & Campus Live Event Window** (`14,200 req/s`, `18.4x` SLO burn rate), **Runbook RB-402** is approved subject to **Incident Commander HITL sign-off**. Click **🚀 Step 3: Execute Canary Rollback (RB-402) →** below!")
		case "execute_remediation":
			status, _ := lastResp["status"].(string)
			switch status {
			case "blocked":
				return sreFn("a2a_evaluate_blast_radius", map[string]any{
					"action":          "rollback_canary",
					"target_revision": "checkout-api-stable-v2.13.8",
				})
			case "pending_approval":
				msg, _ := lastResp["message"].(string)
				return sreText(fmt.Sprintf("⏳ **Incident Commander HITL Gate Active:** %s Use the **Incident Commander Approval Required** banner above to authorize or abort Runbook RB-402.", msg))
			default:
				return sreText("✅ **Runbook RB-402 Executed Successfully!** 100% of GKE Gateway traffic has shifted to `checkout-api-stable-v2.13.8` and leaked AlloyDB sessions were drained. HTTP 5xx dropped from `18.4% → 0.01%` and p99 latency recovered from `4,820 ms → 42 ms`.")
			}
		}
	}

	intent := classifySREIntent(lower)
	if intent == "telemetry" && role != "telemetry_investigator" {
		return sreFn("transfer_to_agent", map[string]any{"agent_name": "telemetry_investigator"})
	}
	if intent == "policy" && role != "mitigation_planner" {
		return sreFn("transfer_to_agent", map[string]any{"agent_name": "mitigation_planner"})
	}
	if intent == "remediate" && role != "remediation_executor" {
		return sreFn("transfer_to_agent", map[string]any{"agent_name": "remediation_executor"})
	}

	switch intent {
	case "telemetry":
		return sreFn("a2a_diagnose_telemetry", map[string]any{
			"service": "checkout-api",
			"symptom": "HTTP 5xx spike 18.4% and AlloyDB connection pool 200/200 saturation",
		})
	case "policy":
		return sreFn("a2a_evaluate_blast_radius", map[string]any{
			"action":          "rollback_canary",
			"target_revision": "checkout-api-stable-v2.13.8",
		})
	case "remediate":
		if strings.Contains(lower, "rejected") || strings.Contains(lower, "aborted") {
			return sreText("🛑 **Rollback Aborted by Incident Commander.** Canary `v2.14.0-rc3` remains isolated in manual investigation mode; no production traffic weights were modified.")
		}
		return sreFn("execute_remediation", map[string]any{
			"runbook_id":      "RB-402",
			"target_revision": "checkout-api-stable-v2.13.8",
		})
	default:
		return sreText("🚨 **CloudGuard AI SRE Incident Commander** online. Ask me to **1) Diagnose SEV-1 telemetry & canary diff via A2A (`:9003`)**, **2) Evaluate blast radius & change-freeze policy via A2A (`:9004`)**, or **3) Execute Canary Rollback RB-402**.")
	}
}

func classifySREIntent(lower string) string {
	if strings.Contains(lower, "execute") || strings.Contains(lower, "rollback") || strings.Contains(lower, "rb-402") || strings.Contains(lower, "commander approved") || strings.Contains(lower, "commander rejected") {
		return "remediate"
	}
	if strings.Contains(lower, "blast") || strings.Contains(lower, "policy") || strings.Contains(lower, "secops") || strings.Contains(lower, "freeze") || strings.Contains(lower, "runbook") {
		return "policy"
	}
	if strings.Contains(lower, "diagnos") || strings.Contains(lower, "telemetry") || strings.Contains(lower, "investigate") || strings.Contains(lower, "root cause") || strings.Contains(lower, "canary") || strings.Contains(lower, "5xx") || strings.Contains(lower, "sev-1") {
		return "telemetry"
	}
	return "telemetry"
}

func extractSRESystem(req *model.LLMRequest) string {
	if req == nil || req.Config == nil || req.Config.SystemInstruction == nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range req.Config.SystemInstruction.Parts {
		if p != nil && p.Text != "" {
			sb.WriteString(p.Text)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func extractSRELastUser(req *model.LLMRequest) string {
	if req == nil {
		return ""
	}
	for i := len(req.Contents) - 1; i >= 0; i-- {
		c := req.Contents[i]
		if c == nil || c.Role != genai.RoleUser || isSRESyntheticContext(c) {
			continue
		}
		for _, p := range c.Parts {
			if p != nil && p.Text != "" {
				return p.Text
			}
		}
	}
	return ""
}

func isSRESyntheticContext(c *genai.Content) bool {
	for _, p := range c.Parts {
		if p != nil && strings.HasPrefix(strings.TrimSpace(p.Text), "For context:") {
			return true
		}
	}
	return false
}

func extractSRELastFn(req *model.LLMRequest) (string, map[string]any) {
	if req == nil || len(req.Contents) == 0 {
		return "", nil
	}
	last := req.Contents[len(req.Contents)-1]
	if last == nil {
		return "", nil
	}
	for i := len(last.Parts) - 1; i >= 0; i-- {
		p := last.Parts[i]
		if p != nil && p.FunctionResponse != nil {
			return p.FunctionResponse.Name, p.FunctionResponse.Response
		}
	}
	return "", nil
}

func sreFn(name string, args map[string]any) *model.LLMResponse {
	return &model.LLMResponse{
		Content: &genai.Content{
			Role: genai.RoleModel,
			Parts: []*genai.Part{
				{
					FunctionCall: &genai.FunctionCall{
						ID:   "call-" + name,
						Name: name,
						Args: args,
					},
				},
			},
		},
	}
}

func sreText(text string) *model.LLMResponse {
	return &model.LLMResponse{
		Content: genai.NewContentFromText(text, genai.RoleModel),
	}
}

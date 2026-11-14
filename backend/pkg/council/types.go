package council

import (
	"github.com/ab/durable-a2ui/backend/pkg/a2aservers"
	"go.temporal.io/sdk/contrib/workflowstreams"
)

const TaskQueue = "architecture-council"

type CouncilInput struct {
	ProposalTitle string                               `json:"proposal_title"`
	ReleaseName   string                               `json:"release_name,omitempty"`
	Architect     string                               `json:"architect"`
	Service       string                               `json:"service"`
	Environment   string                               `json:"environment,omitempty"`
	TargetSchema  int                                  `json:"target_schema,omitempty"` // 1 = v1 Legacy Summary, 2 = v2 Executive Scorecard
	SchemaVersion int                                  `json:"schema_version,omitempty"`
	StreamState   *workflowstreams.WorkflowStreamState `json:"stream_state,omitempty"`
}

func (in *CouncilInput) ApplyDefaults() {
	if in.ProposalTitle == "" && in.ReleaseName != "" {
		in.ProposalTitle = in.ReleaseName
	}
	if in.ProposalTitle == "" {
		in.ProposalTitle = "RFC-804: Multi-Region LLM Gateway & AlloyDB Vector Cache"
	}
	if in.Architect == "" {
		in.Architect = "Principal Architecture Council"
	}
	if in.Service == "" && in.Environment != "" {
		in.Service = in.Environment
	}
	if in.Service == "" {
		in.Service = "checkout-ai-gateway"
	}
	if in.TargetSchema == 0 && in.SchemaVersion != 0 {
		in.TargetSchema = in.SchemaVersion
	}
	if in.TargetSchema != 1 && in.TargetSchema != 2 {
		in.TargetSchema = 1
	}
}

type SpecialistResult struct {
	Domain         string                 `json:"domain"` // "reliability", "secops", "finops"
	AgentName      string                 `json:"agent_name"`
	Endpoint       string                 `json:"endpoint"`
	Status         string                 `json:"status"` // "PASS", "WARN", "READY"
	Headline       string                 `json:"headline"`
	KeyMetric      string                 `json:"key_metric"`
	Recommendation string                 `json:"recommendation"`
	DurationMs     int                    `json:"duration_ms"`
	XRay           a2aservers.A2ACallMeta `json:"xray"`
}

type CouncilState struct {
	ProposalTitle string            `json:"proposal_title"`
	Architect     string            `json:"architect"`
	Service       string            `json:"service"`
	SchemaVersion int               `json:"schema_version"` // 1 or 2
	Completed     int               `json:"completed"`
	TotalBranches int               `json:"total_branches"`
	Reliability   *SpecialistResult `json:"reliability,omitempty"`
	SecOps        *SpecialistResult `json:"secops,omitempty"`
	FinOps        *SpecialistResult `json:"finops,omitempty"`
	Decision      string            `json:"decision"` // "", "approved", "changes_requested"
}

package order

import (
	"context"
	"fmt"
	"time"
)

type Activities struct{}

type StageRequest struct {
	OrderID string `json:"order_id"`
	Stage   string `json:"stage"`
	Rushed  bool   `json:"rushed"`
}

type StageResult struct {
	Stage      string      `json:"stage"`
	ETAMinutes int         `json:"eta_minutes"`
	Driver     *DriverInfo `json:"driver,omitempty"`
}

func (a *Activities) ExecuteStage(ctx context.Context, req StageRequest) (StageResult, error) {
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return StageResult{}, ctx.Err()
	}

	eta := 25
	var driver *DriverInfo
	switch req.Stage {
	case "confirmed":
		eta = 30
	case "preparing":
		eta = 20
	case "out_for_delivery":
		eta = 10
		driver = &DriverInfo{
			Name:    "Carlos M.",
			Vehicle: "Honda PCX 150",
			Rating:  4.9,
		}
	case "delivered":
		eta = 0
	default:
		return StageResult{}, fmt.Errorf("unknown stage: %s", req.Stage)
	}

	if req.Rushed && eta > 0 {
		eta = eta / 2
	}

	return StageResult{
		Stage:      req.Stage,
		ETAMinutes: eta,
		Driver:     driver,
	}, nil
}

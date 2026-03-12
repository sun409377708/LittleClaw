package planner

import (
	"fmt"
	"strings"
)

func (d Decision) Validate() error {
	switch d.Type {
	case DecisionTool:
		if strings.TrimSpace(d.ToolName) == "" {
			return fmt.Errorf("tool decision requires tool_name")
		}
		if d.ToolInput == nil {
			return fmt.Errorf("tool decision requires tool_input")
		}
	case DecisionFinal:
		if strings.TrimSpace(d.FinalOutput) == "" {
			return fmt.Errorf("final decision requires final_output")
		}
	default:
		return fmt.Errorf("unsupported decision type %q", d.Type)
	}
	return nil
}

package planner

import (
	"context"
	"fmt"
)

type DecisionClient interface {
	Decide(ctx context.Context, state State) (Decision, error)
}

type LLMPlanner struct {
	client DecisionClient
}

func NewLLM(client DecisionClient) *LLMPlanner {
	return &LLMPlanner{client: client}
}

func (p *LLMPlanner) Decide(ctx context.Context, state State) (Decision, error) {
	if p.client == nil {
		return Decision{}, fmt.Errorf("llm planner client is nil")
	}
	return p.client.Decide(ctx, state)
}

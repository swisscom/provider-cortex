package rulegroup

import (
	"context"
	"fmt"

	cortexClient "github.com/cortexproject/cortex-tools/pkg/client"
	"github.com/cortexproject/cortex-tools/pkg/rules/rwrulefmt"
)

type RuleGroupClient interface {
	GetRuleGroup(ctx context.Context, namespace, groupName string) (*rwrulefmt.RuleGroup, error)
	CreateRuleGroup(ctx context.Context, namespace string, rg rwrulefmt.RuleGroup) error
	DeleteRuleGroup(ctx context.Context, namespace, groupName string) error
	// Update()
}

func NewClient(config cortexClient.Config) *cortexClient.CortexClient {
	client, err := cortexClient.New(config)

	if err != nil {
		fmt.Printf("Could not initialize cortex client: %v", err)
	}
	return client
}

package api

import (
	"context"
	"net/http"
	"net/url"
)

// Rule is one mihomo rule entry.
type Rule struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
	Proxy   string `json:"proxy"`
}

type rulesResp struct {
	Rules []Rule `json:"rules"`
}

// GetRules fetches /rules.
func (c *Client) GetRules(ctx context.Context) ([]Rule, error) {
	var r rulesResp
	if err := c.do(ctx, http.MethodGet, "/rules", nil, &r); err != nil {
		return nil, err
	}
	return r.Rules, nil
}

// RuleProvider mirrors one entry in /providers/rules.
type RuleProvider struct {
	Name        string `json:"name"`
	Behavior    string `json:"behavior"`
	RuleCount   int    `json:"ruleCount"`
	UpdatedAt   string `json:"updatedAt"`
	Type        string `json:"type"`
	VehicleType string `json:"vehicleType"`
}

type ruleProvidersResp struct {
	Providers map[string]RuleProvider `json:"providers"`
}

// GetRuleProviders fetches /providers/rules.
func (c *Client) GetRuleProviders(ctx context.Context) (map[string]RuleProvider, error) {
	var r ruleProvidersResp
	if err := c.do(ctx, http.MethodGet, "/providers/rules", nil, &r); err != nil {
		return nil, err
	}
	return r.Providers, nil
}

// RefreshRuleProvider triggers a re-download for a single provider.
func (c *Client) RefreshRuleProvider(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodPut, "/providers/rules/"+url.PathEscape(name), nil, nil)
}

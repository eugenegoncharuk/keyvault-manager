package azure

import (
	"encoding/json"
	"fmt"
)

// ListSubscriptions dynamically requests the active user's available
// Azure subscriptions using the az CLI tool.
func ListSubscriptions() ([]Subscription, error) {
	out, err := runAZ("account", "list", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("az account list: %w", err)
	}
	var subs []Subscription
	if err := json.Unmarshal(out, &subs); err != nil {
		return nil, fmt.Errorf("parsing subscriptions: %w", err)
	}
	return subs, nil
}

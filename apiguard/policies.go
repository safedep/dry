package apiguard

import "fmt"

type policyInfo struct {
	name string
}

func (p *policyInfo) Name() string {
	return p.name
}

// Policy map generated from API Guard service
// Must be kept in sync with the API Guard service
var PolicyMap = map[string]policyInfo{
	"developer": {
		name: "developer",
	},
	"basic": {
		name: "basic-user-group",
	},
	"standard": {
		name: "standard",
	},
	"enterprise": {
		name: "enterprise",
	},
}

// This probably needs a revamp when we look at subscription
// based policy mapping
func GetBasicPolicyInfo() (policyInfo, error) {
	if policy, ok := PolicyMap["basic"]; ok {
		return policy, nil
	}

	return policyInfo{}, fmt.Errorf("policy not found")
}

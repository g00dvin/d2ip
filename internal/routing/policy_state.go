package routing

import (
	"fmt"
	"os"
	"path/filepath"
)

func policyStatePath(stateDir, policyName string) string {
	return filepath.Join(stateDir, policyName+".json")
}

func loadPolicyState(stateDir, policyName string) (RouterState, error) {
	path := policyStatePath(stateDir, policyName)
	return loadState(path)
}

func savePolicyState(stateDir, policyName string, s RouterState) error {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}
	path := policyStatePath(stateDir, policyName)
	return saveState(path, s)
}

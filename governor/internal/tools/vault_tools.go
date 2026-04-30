package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vibepilot/governor/internal/vault"
)

type VaultGetTool struct {
	vault *vault.Vault
}

func NewVaultGetTool(v *vault.Vault) *VaultGetTool {
	return &VaultGetTool{vault: v}
}

func (t *VaultGetTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	key, ok := args["key"].(string)
	if !ok {
		return nil, fmt.Errorf("key parameter required")
	}

	_, err := t.vault.GetSecret(ctx, key)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"key":     key,
			"exists":  false,
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"key":     key,
		"exists":  true,
		"message": "Secret exists but value is not exposed for security",
	})
}

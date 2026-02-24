package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type RoleRegistry struct {
	roles    map[string]*Role
	filePath string
}

type rolesFile struct {
	Roles        []Role                 `json:"roles"`
	RoutingRules map[string]RoutingRule `json:"routing_rules"`
}

type RoutingRule struct {
	RoutingFlag    string   `json:"routing_flag"`
	AllowedRoles   []string `json:"allowed_roles"`
	ForbiddenRoles []string `json:"forbidden_roles"`
	PreferredRole  string   `json:"preferred_role"`
}

func LoadRoles(path string) (*RoleRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read roles file: %w", err)
	}

	return ParseRoles(data, path)
}

func ParseRoles(data []byte, sourcePath string) (*RoleRegistry, error) {
	var rf rolesFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parse roles json: %w", err)
	}

	registry := &RoleRegistry{
		roles:    make(map[string]*Role),
		filePath: sourcePath,
	}

	for i := range rf.Roles {
		registry.roles[rf.Roles[i].ID] = &rf.Roles[i]
	}

	return registry, nil
}

func (r *RoleRegistry) Get(id string) *Role {
	return r.roles[id]
}

func (r *RoleRegistry) All() []*Role {
	roles := make([]*Role, 0, len(r.roles))
	for _, role := range r.roles {
		roles = append(roles, role)
	}
	return roles
}

func (r *RoleRegistry) ByRoutingFlag(flag string) []*Role {
	var roles []*Role
	for _, role := range r.roles {
		if role.RequiresDestination == flag {
			roles = append(roles, role)
		}
	}
	return roles
}

func (r *RoleRegistry) FilePath() string {
	return r.filePath
}

func (r *RoleRegistry) Dir() string {
	return filepath.Dir(r.filePath)
}

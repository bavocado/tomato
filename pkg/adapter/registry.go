package adapter

// Registry maps workflow roles (e.g. "pr", "task", "review") to the adapter
// Bridge that serves them. It is built by the engine from tomato.yaml's
// `roles:` and `adapters:` sections (or a TOMATO_ADAPTER_BIN fallback) and
// threaded into each step so steps never reach for a global.
type Registry struct {
	byRole map[string]*Bridge
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byRole: map[string]*Bridge{}}
}

// Set binds a role to a bridge.
func (r *Registry) Set(role string, b *Bridge) {
	r.byRole[role] = b
}

// For returns the bridge configured for a role, or nil if none. Safe to call
// on a nil Registry (returns nil).
func (r *Registry) For(role string) *Bridge {
	if r == nil {
		return nil
	}
	return r.byRole[role]
}

// ForAny returns the first configured bridge among the given roles, in the
// order provided, or nil if none is configured. Used where one operation may
// be served by either of several roles (e.g. review_loop PR operations prefer
// the "pr" role, falling back to "review").
func (r *Registry) ForAny(roles ...string) *Bridge {
	if r == nil {
		return nil
	}
	for _, role := range roles {
		if b, ok := r.byRole[role]; ok {
			return b
		}
	}
	return nil
}

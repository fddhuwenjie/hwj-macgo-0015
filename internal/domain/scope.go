package domain

import "fmt"

// ResourceScope 描述授权覆盖的资源范围。
type ResourceScope struct {
	Type       string            `json:"type"`
	Identifier string            `json:"identifier"`
	Properties map[string]string `json:"properties"`
}

// Clone 返回深拷贝。
func (r ResourceScope) Clone() ResourceScope {
	props := make(map[string]string, len(r.Properties))
	for k, v := range r.Properties {
		props[k] = v
	}
	r.Properties = props
	return r
}

// Contains 判断当前范围是否包含另一个范围；用于收窄校验。
func (r ResourceScope) Contains(other ResourceScope) bool {
	if r.Type != other.Type || r.Identifier != other.Identifier {
		return false
	}
	for k, v := range other.Properties {
		if rv, ok := r.Properties[k]; !ok || rv != v {
			return false
		}
	}
	return true
}

// Validate 基本校验。
func (r ResourceScope) Validate() error {
	if r.Type == "" || r.Identifier == "" {
		return fmt.Errorf("scope type and identifier are required")
	}
	return nil
}

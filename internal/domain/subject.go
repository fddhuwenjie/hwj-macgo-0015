package domain

import "time"

// Subject 代表申请授权的主体。
type Subject struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Version    int64             `json:"version"`
}

// Clone 返回深拷贝，避免共享状态修改。
func (s Subject) Clone() Subject {
	attrs := make(map[string]string, len(s.Attributes))
	for k, v := range s.Attributes {
		attrs[k] = v
	}
	s.Attributes = attrs
	return s
}

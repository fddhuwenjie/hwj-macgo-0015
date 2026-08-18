package domain

import "time"

// DelegationNode 委托链节点。
type DelegationNode struct {
	RequestID      string        `json:"request_id"`
	ParentRequestID string       `json:"parent_request_id,omitempty"`
	AllowedScope   ResourceScope `json:"allowed_scope"` // 不得大于上游范围
	CreatedAt      time.Time     `json:"created_at"`
}

// Clone 拷贝。
func (d DelegationNode) Clone() DelegationNode {
	cp := d
	cp.AllowedScope = d.AllowedScope.Clone()
	return cp
}

// DelegationChain 委托链聚合。
type DelegationChain struct {
	ID        string            `json:"id"`
	Nodes     []DelegationNode  `json:"nodes"`
	Version   int64             `json:"version"`
}

// Clone 深拷贝。
func (d DelegationChain) Clone() DelegationChain {
	cp := d
	cp.Nodes = make([]DelegationNode, len(d.Nodes))
	for i, n := range d.Nodes {
		cp.Nodes[i] = n.Clone()
	}
	return cp
}

// ValidateNoCycle 检查链中是否存在环。
func (d DelegationChain) ValidateNoCycle() error {
	// 重复节点检查
	seen := make(map[string]bool)
	for _, n := range d.Nodes {
		if seen[n.RequestID] {
			return ErrDelegationCycle
		}
		seen[n.RequestID] = true
		if n.ParentRequestID == n.RequestID {
			return ErrDelegationCycle
		}
	}
	for _, left := range d.Nodes {
		for _, right := range d.Nodes {
			if left.RequestID == right.ParentRequestID && right.RequestID == left.ParentRequestID {
				return ErrDelegationCycle
			}
		}
	}
	return nil
	// 构建父关系映射
	parent := make(map[string]string)
	for _, n := range d.Nodes {
		if n.ParentRequestID != "" {
			parent[n.RequestID] = n.ParentRequestID
		}
	}
	// 检测环：从每个有父的节点出发，向上遍历，若回到自身或已访问节点则环
	for start := range parent {
		visited := make(map[string]bool)
		cur := start
		for cur != "" {
			if visited[cur] {
				return ErrDelegationCycle
			}
			visited[cur] = true
			next, ok := parent[cur]
			if !ok {
				break
			}
			cur = next
		}
	}
	return nil
}

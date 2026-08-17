package application

import (
	"context"
	"fmt"
	"time"

	"evidence/internal/domain"
)

// DelegationService 处理委托链。
type DelegationService struct {
	repo domain.Repository
	mu   chan struct{} // 互斥锁，简化
}

// NewDelegationService 创建委托服务。
func NewDelegationService(repo domain.Repository) *DelegationService {
	return &DelegationService{
		repo: repo,
		mu:   make(chan struct{}, 1),
	}
}

func (d *DelegationService) lock()   { d.mu <- struct{}{} }
func (d *DelegationService) unlock() { <-d.mu }

// CreateDelegation 创建委托关系，确保无环且不扩大范围。
func (d *DelegationService) CreateDelegation(ctx context.Context, parentRequestID, childRequestID string) error {
	d.lock()
	defer d.unlock()
	tx, err := d.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	repo := tx.Repository()
	parent, err := repo.GetRequest(ctx, parentRequestID)
	if err != nil {
		return err
	}
	child, err := repo.GetRequest(ctx, childRequestID)
	if err != nil {
		return err
	}
	// 父必须已启用或恢复状态
	if parent.Status != domain.StatusEnabled && parent.Status != domain.StatusResumed {
		return domain.ErrInvalidStateTransition
	}
	// 获取父条件版本的范围
	parentCV, err := repo.GetConditionVersion(ctx, parent.CurrentVersionID)
	if err != nil {
		return err
	}
	childCV, err := repo.GetConditionVersion(ctx, child.CurrentVersionID)
	if err != nil {
		return err
	}
	// 子范围必须被父范围包含
	if !parentCV.Scope.Contains(childCV.Scope) {
		return domain.ErrScopeExpansion
	}
	// 检查环：获取子所在链（若有），检查是否已包含父
	if child.DelegationChainID != "" {
		chain, err := repo.GetDelegationChain(ctx, child.DelegationChainID)
		if err != nil {
			return err
		}
		for _, node := range chain.Nodes {
			if node.RequestID == parentRequestID {
				return domain.ErrDelegationCycle
			}
		}
	}
	// 获取或创建链
	var chain domain.DelegationChain
	if parent.DelegationChainID != "" {
		chain, err = repo.GetDelegationChain(ctx, parent.DelegationChainID)
		if err != nil {
			return err
		}
	} else {
		chain = domain.DelegationChain{
			ID:      fmt.Sprintf("chain-%s", parentRequestID),
			Version: 1,
		}
		// 添加父节点作为根
		chain.Nodes = append(chain.Nodes, domain.DelegationNode{
			RequestID:    parentRequestID,
			AllowedScope: parentCV.Scope,
			CreatedAt:    time.Now(),
		})
	}
	// 添加子节点
	chain.Nodes = append(chain.Nodes, domain.DelegationNode{
		RequestID:       childRequestID,
		ParentRequestID: parentRequestID,
		AllowedScope:    childCV.Scope,
		CreatedAt:       time.Now(),
	})
	chain.Version++
	if err := repo.SaveDelegationChain(ctx, chain); err != nil {
		return err
	}
	child.DelegationChainID = chain.ID
	child.Version++
	if err := repo.SaveRequest(ctx, child); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

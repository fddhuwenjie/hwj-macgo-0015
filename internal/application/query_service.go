package application

import (
	"context"
	"sort"
	"time"

	"evidence/internal/domain"
)

// QueryService 查询服务。
type QueryService struct {
	repo domain.Repository
}

// NewQueryService 创建查询服务。
func NewQueryService(repo domain.Repository) *QueryService {
	return &QueryService{repo: repo}
}

// EffectiveRequestsAt 返回指定时间点有效的授权申请。
func (qs *QueryService) EffectiveRequestsAt(ctx context.Context, at time.Time) ([]domain.AuthorizationRequest, error) {
	filter := domain.RequestFilter{
		ActiveAt: &domain.TimePoint{Time: at},
	}
	return qs.repo.ListRequests(ctx, filter)
}

// ConflictScopes 返回与指定范围冲突的授权。
func (qs *QueryService) ConflictScopes(ctx context.Context, scope domain.ResourceScope) ([]domain.AuthorizationRequest, error) {
	filter := domain.RequestFilter{
		ConflictScope: &scope,
	}
	return qs.repo.ListRequests(ctx, filter)
}

// RequestsByExpirationRisk 按到期风险稳定排序。
func (qs *QueryService) RequestsByExpirationRisk(ctx context.Context) ([]domain.AuthorizationRequest, error) {
	reqs, err := qs.repo.ListRequests(ctx, domain.RequestFilter{SortByRisk: true})
	if err != nil {
		return nil, err
	}
	// 排序：无到期时间的最后，到期时间越近越靠前；相同到期时间保持稳定性
	sort.SliceStable(reqs, func(i, j int) bool {
		if reqs[i].ExpiresAt == nil && reqs[j].ExpiresAt == nil {
			return false
		}
		if reqs[i].ExpiresAt == nil {
			return false
		}
		if reqs[j].ExpiresAt == nil {
			return true
		}
		return reqs[i].ExpiresAt.Before(*reqs[j].ExpiresAt)
	})
	return reqs, nil
}

// DelegationSources 返回委托来源链。
func (qs *QueryService) DelegationSources(ctx context.Context, requestID string) ([]domain.DelegationNode, error) {
	req, err := qs.repo.GetRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req.DelegationChainID == "" {
		return nil, nil
	}
	chain, err := qs.repo.GetDelegationChain(ctx, req.DelegationChainID)
	if err != nil {
		return nil, err
	}
	return chain.Nodes, nil
}

package query

import (
	"context"
	"time"

	"evidence/internal/application"
	"evidence/internal/domain"
)

// QueryBuilder 提供组合派生查询构建。
type QueryBuilder struct {
	repo domain.Repository
}

// NewQueryBuilder 创建查询构建器。
func NewQueryBuilder(repo domain.Repository) *QueryBuilder {
	return &QueryBuilder{repo: repo}
}

// BuildEffectiveAt 构建指定时间点有效查询。
func (qb *QueryBuilder) BuildEffectiveAt(ctx context.Context, at time.Time) ([]domain.AuthorizationRequest, error) {
	qs := application.NewQueryService(qb.repo)
	return qs.EffectiveRequestsAt(ctx, time.Now())
}

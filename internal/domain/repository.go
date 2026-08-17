package domain

import "context"

// Repository 定义聚合与查询所需的持久化接口。
type Repository interface {
	// Subjects
	SaveSubject(ctx context.Context, s Subject) error
	GetSubject(ctx context.Context, id string) (Subject, error)

	// AuthorizationRequests
	SaveRequest(ctx context.Context, req AuthorizationRequest) error
	GetRequest(ctx context.Context, id string) (AuthorizationRequest, error)
	ListRequests(ctx context.Context, filter RequestFilter) ([]AuthorizationRequest, error)

	// ConditionVersions
	SaveConditionVersion(ctx context.Context, cv ConditionVersion) error
	GetConditionVersion(ctx context.Context, id string) (ConditionVersion, error)

	// ReviewRounds
	SaveReviewRound(ctx context.Context, rr ReviewRound) error
	GetReviewRound(ctx context.Context, id string) (ReviewRound, error)

	// DecisionCredentials
	SaveDecision(ctx context.Context, d DecisionCredential) error
	GetDecision(ctx context.Context, id string) (DecisionCredential, error)

	// TemporarySuspensions
	SaveSuspension(ctx context.Context, s TemporarySuspension) error
	GetSuspension(ctx context.Context, id string) (TemporarySuspension, error)

	// DelegationChains
	SaveDelegationChain(ctx context.Context, d DelegationChain) error
	GetDelegationChain(ctx context.Context, id string) (DelegationChain, error)

	// ExpirationEvents
	SaveExpiration(ctx context.Context, e ExpirationEvent) error
	GetExpiration(ctx context.Context, id string) (ExpirationEvent, error)

	// 事务支持
	BeginTx(ctx context.Context) (Transaction, error)
}

// RequestFilter 查询请求的过滤条件。
type RequestFilter struct {
	SubjectID     string
	Status        AuthorizationStatus
	ActiveAt      *TimePoint // 指定时间点有效
	ConflictScope *ResourceScope
	SortByRisk    bool
}

// TimePoint 用于时间点查询。
type TimePoint struct {
	Time interface{} // 使用 time.Time 类型
}

// Transaction 提供原子提交与回滚。
type Transaction interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	// 在事务内获取仓库操作句柄
	Repository() Repository
}

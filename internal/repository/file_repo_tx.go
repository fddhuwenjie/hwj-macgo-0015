package repository

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"time"

	"evidence/internal/domain"
)

// errNestedTx 表示当前实现不支持嵌套事务。
var errNestedTx = errors.New("nested transactions are not supported")

// decodeJSON 反序列化 JSON 字节到目标对象。
func decodeJSON(data []byte, out interface{}) error {
	return json.Unmarshal(data, out)
}

// applyRequestFilter 在内存中对申请列表应用与 FileRepository.ListRequests 完全一致的过滤逻辑，
// 确保查询结果在已落盘数据与未提交缓冲之间保持一致。
func applyRequestFilter(reqs []domain.AuthorizationRequest, filter domain.RequestFilter) []domain.AuthorizationRequest {
	result := make([]domain.AuthorizationRequest, 0, len(reqs))
	for _, req := range reqs {
		if filter.SubjectID != "" && req.SubjectID != filter.SubjectID {
			continue
		}
		if filter.Status != "" && req.Status != filter.Status {
			continue
		}
		if filter.ActiveAt != nil {
			if req.Status != domain.StatusEnabled && req.Status != domain.StatusResumed {
				continue
			}
			if req.ExpiresAt != nil && req.ExpiresAt.Before(filter.ActiveAt.Time.(time.Time)) {
				continue
			}
		}
		if filter.ConflictScope != nil {
			if req.ResourceScope.Type != filter.ConflictScope.Type ||
				req.ResourceScope.Identifier != filter.ConflictScope.Identifier {
				continue
			}
		}
		result = append(result, req)
	}
	return result
}

// txRepo 是事务作用域内的仓库句柄：写入进入事务缓冲，读取先查缓冲再落盘。
// 它确保事务内多次读写的一致性（read-your-writes），并让提交前的失败不会留下任何部分写入。
type txRepo struct {
	tx   *fileTransaction
	base *FileRepository
}

func (t *txRepo) path(subdir, id string) string {
	return filepath.Join(subdir, id+".json")
}

// ---- 读取：先查本事务缓冲，未命中再读已落盘数据 ----

func (t *txRepo) GetSubject(ctx context.Context, id string) (domain.Subject, error) {
	var s domain.Subject
	if data, ok := t.tx.bufferRead(t.path("subjects", id)); ok {
		return s, decodeJSON(data, &s)
	}
	return t.base.GetSubject(ctx, id)
}

func (t *txRepo) GetRequest(ctx context.Context, id string) (domain.AuthorizationRequest, error) {
	var req domain.AuthorizationRequest
	if data, ok := t.tx.bufferRead(t.path("requests", id)); ok {
		return req, decodeJSON(data, &req)
	}
	return t.base.GetRequest(ctx, id)
}

func (t *txRepo) GetConditionVersion(ctx context.Context, id string) (domain.ConditionVersion, error) {
	var cv domain.ConditionVersion
	if data, ok := t.tx.bufferRead(t.path("condition_versions", id)); ok {
		return cv, decodeJSON(data, &cv)
	}
	return t.base.GetConditionVersion(ctx, id)
}

func (t *txRepo) GetReviewRound(ctx context.Context, id string) (domain.ReviewRound, error) {
	var rr domain.ReviewRound
	if data, ok := t.tx.bufferRead(t.path("review_rounds", id)); ok {
		return rr, decodeJSON(data, &rr)
	}
	return t.base.GetReviewRound(ctx, id)
}

func (t *txRepo) GetDecision(ctx context.Context, id string) (domain.DecisionCredential, error) {
	var d domain.DecisionCredential
	if data, ok := t.tx.bufferRead(t.path("decisions", id)); ok {
		return d, decodeJSON(data, &d)
	}
	return t.base.GetDecision(ctx, id)
}

func (t *txRepo) GetSuspension(ctx context.Context, id string) (domain.TemporarySuspension, error) {
	var s domain.TemporarySuspension
	if data, ok := t.tx.bufferRead(t.path("suspensions", id)); ok {
		return s, decodeJSON(data, &s)
	}
	return t.base.GetSuspension(ctx, id)
}

func (t *txRepo) GetDelegationChain(ctx context.Context, id string) (domain.DelegationChain, error) {
	var d domain.DelegationChain
	if data, ok := t.tx.bufferRead(t.path("delegation_chains", id)); ok {
		return d, decodeJSON(data, &d)
	}
	return t.base.GetDelegationChain(ctx, id)
}

func (t *txRepo) GetExpiration(ctx context.Context, id string) (domain.ExpirationEvent, error) {
	var e domain.ExpirationEvent
	if data, ok := t.tx.bufferRead(t.path("expirations", id)); ok {
		return e, decodeJSON(data, &e)
	}
	return t.base.GetExpiration(ctx, id)
}

// ---- 写入：进入事务缓冲，提交时统一落盘 ----

func (t *txRepo) SaveSubject(ctx context.Context, s domain.Subject) error {
	return t.tx.bufferWrite(t.path("subjects", s.ID), s)
}

func (t *txRepo) SaveRequest(ctx context.Context, req domain.AuthorizationRequest) error {
	return t.tx.bufferWrite(t.path("requests", req.ID), req)
}

func (t *txRepo) SaveConditionVersion(ctx context.Context, cv domain.ConditionVersion) error {
	return t.tx.bufferWrite(t.path("condition_versions", cv.ID), cv)
}

func (t *txRepo) SaveReviewRound(ctx context.Context, rr domain.ReviewRound) error {
	return t.tx.bufferWrite(t.path("review_rounds", rr.ID), rr)
}

func (t *txRepo) SaveDecision(ctx context.Context, d domain.DecisionCredential) error {
	return t.tx.bufferWrite(t.path("decisions", d.ID), d)
}

func (t *txRepo) SaveSuspension(ctx context.Context, s domain.TemporarySuspension) error {
	return t.tx.bufferWrite(t.path("suspensions", s.ID), s)
}

func (t *txRepo) SaveDelegationChain(ctx context.Context, d domain.DelegationChain) error {
	return t.tx.bufferWrite(t.path("delegation_chains", d.ID), d)
}

func (t *txRepo) SaveExpiration(ctx context.Context, e domain.ExpirationEvent) error {
	return t.tx.bufferWrite(t.path("expirations", e.ID), e)
}

// ---- ListRequests：合并已落盘数据与本事务尚未提交的写入 ----

func (t *txRepo) ListRequests(ctx context.Context, filter domain.RequestFilter) ([]domain.AuthorizationRequest, error) {
	// 先取已落盘的全部申请，再用本事务缓冲覆盖/追加，确保查询结果与未提交写入一致。
	base, err := t.base.ListRequests(ctx, domain.RequestFilter{})
	if err != nil {
		return nil, err
	}
	byID := make(map[string]domain.AuthorizationRequest, len(base))
	order := make([]string, 0, len(base))
	for _, r := range base {
		if _, exists := byID[r.ID]; !exists {
			order = append(order, r.ID)
		}
		byID[r.ID] = r
	}
	// 覆盖/追加本事务缓冲中的申请。
	t.tx.mu.Lock()
	for _, pw := range t.tx.buffer {
		var r domain.AuthorizationRequest
		if decodeJSON(pw.data, &r) != nil {
			continue
		}
		// 仅处理 requests 目录下的写入。
		if dir := filepath.Dir(pw.path); dir != "requests" {
			continue
		}
		if _, exists := byID[r.ID]; !exists {
			order = append(order, r.ID)
		}
		byID[r.ID] = r
	}
	t.tx.mu.Unlock()

	merged := make([]domain.AuthorizationRequest, 0, len(order))
	for _, id := range order {
		merged = append(merged, byID[id])
	}
	return applyRequestFilter(merged, filter), nil
}

// ---- 嵌套事务：当前不支持，返回错误以避免静默丢失缓冲保证 ----

func (t *txRepo) BeginTx(ctx context.Context) (domain.Transaction, error) {
	return nil, errNestedTx
}

// 编译期断言：txRepo 实现完整的 Repository 接口。
var _ domain.Repository = (*txRepo)(nil)

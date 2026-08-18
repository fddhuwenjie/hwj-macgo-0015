package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"evidence/internal/domain"
)

// FileRepository 基于本地目录的仓库实现。
//
// 并发模型：
//   - txMu 串行化写事务，同一时刻只允许一个事务处于打开状态（单写者）。
//   - mu 保护磁盘读写；非事务读（Get*/List*）与事务提交都经过 mu，因此并发查询
//     看到的要么是提交前、要么是提交后的状态，绝不会看到半更新状态。
//   - 事务内的写并不直接落盘，而是缓冲在内存里；只有 Commit 才在 mu 保护下
//     原子发布，Rollback 仅丢弃缓冲。从而保证“失败状态对其他请求不可见”。
type FileRepository struct {
	rootDir string
	mu      sync.RWMutex // 保护磁盘文件读写与目录列举
	txMu    sync.Mutex   // 串行化写事务（单写者）
}

// NewFileRepository 创建仓库。
func NewFileRepository(rootDir string) (*FileRepository, error) {
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, err
	}
	return &FileRepository{rootDir: rootDir}, nil
}

// BeginTx 开始事务。获取 txMu，串行化所有写事务。
func (r *FileRepository) BeginTx(ctx context.Context) (domain.Transaction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// txMu 在 Commit/Rollback 中释放；此处若 ctx 已取消，调用方需 Rollback 释放。
	r.txMu.Lock()
	return &fileTransaction{
		repo:   r,
		buffer: make(map[string]pendingWrite),
	}, nil
}

// pendingWrite 事务内一次待提交的写操作。
type pendingWrite struct {
	subdir string
	id     string
	data   []byte
}

// recordKey 生成缓冲条目的键。
func recordKey(subdir, id string) string { return subdir + "\x00" + id }

// fileTransaction 一次文件系统事务。
type fileTransaction struct {
	repo   *FileRepository
	mu     sync.Mutex // 保护 buffer 与 done
	done   bool
	buffer map[string]pendingWrite
}

// Commit 原子发布所有缓冲写：先写全部临时文件并落盘，再按依赖顺序重命名，
// 被引用实体（条件版本等）先于引用方（申请）落地，保证引用始终可达。
func (t *fileTransaction) Commit(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		return nil
	}
	t.done = true
	defer t.repo.txMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return t.repo.commitBuffer(ctx, t.buffer)
}

// Rollback 丢弃所有缓冲写并释放 txMu。磁盘上不留下任何半更新痕迹。
func (t *fileTransaction) Rollback(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		return nil
	}
	t.done = true
	t.buffer = nil
	t.repo.txMu.Unlock()
	return nil
}

// Repository 返回事务内的仓库视图：写进入缓冲，读先看缓冲再看磁盘。
func (t *fileTransaction) Repository() domain.Repository {
	return &txRepository{tx: t}
}

// commitBuffer 在 mu 保护下原子发布缓冲写。
func (r *FileRepository) commitBuffer(ctx context.Context, buffer map[string]pendingWrite) error {
	if len(buffer) == 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	type staged struct {
		tmpPath   string
		finalPath string
		subdir    string
	}
	stagedFiles := make([]staged, 0, len(buffer))
	for _, pw := range buffer {
		dir := filepath.Join(r.rootDir, pw.subdir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		finalPath := filepath.Join(dir, pw.id+".json")
		tmpPath := finalPath + ".tmp"
		if err := os.WriteFile(tmpPath, pw.data, 0644); err != nil {
			return err
		}
		stagedFiles = append(stagedFiles, staged{tmpPath: tmpPath, finalPath: finalPath, subdir: pw.subdir})
	}
	// 被引用实体先落地：condition_versions 在 requests 之前重命名，
	// 使得任何时刻“申请若指向某条件版本，则该条件版本必已存在”。
	sort.SliceStable(stagedFiles, func(i, j int) bool {
		return subdirPriority(stagedFiles[i].subdir) < subdirPriority(stagedFiles[j].subdir)
	})
	// 阶段一：全部写临时文件并落盘。
	for _, s := range stagedFiles {
		f, err := os.OpenFile(s.tmpPath, os.O_RDONLY, 0644)
		if err != nil {
			return err
		}
		if err := f.Sync(); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	// 阶段二：按依赖顺序重命名落地。
	for _, s := range stagedFiles {
		if err := os.Rename(s.tmpPath, s.finalPath); err != nil {
			return err
		}
	}
	return nil
}

// subdirPriority 决定提交落地顺序：值小者先落地。被引用实体优先于引用方。
func subdirPriority(subdir string) int {
	switch subdir {
	case "condition_versions":
		return 0 // 条件版本被申请引用，必须先落地
	case "subjects":
		return 1
	case "review_rounds":
		return 2
	case "decisions":
		return 3
	case "suspensions":
		return 4
	case "delegation_chains":
		return 5
	case "expirations":
		return 6
	case "requests":
		return 7 // 申请最后落地，确保其引用的版本已存在
	default:
		return 8
	}
}

// writeJSON 直接写盘（非事务路径或事务提交内部使用）。
func (r *FileRepository) writeJSON(ctx context.Context, subdir, id string, v interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := filepath.Join(r.rootDir, subdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, id+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (r *FileRepository) readJSON(ctx context.Context, subdir, id string, out interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := filepath.Join(r.rootDir, subdir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.ErrNotFound
		}
		return err
	}
	return json.Unmarshal(data, out)
}

func (r *FileRepository) listJSON(ctx context.Context, subdir string, out interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := filepath.Join(r.rootDir, subdir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var items []json.RawMessage
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		items = append(items, data)
	}
	all, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return json.Unmarshal(all, out)
}

// txRepository 事务内的仓库视图。
type txRepository struct {
	tx *fileTransaction
}

// stage 把一次写缓冲进事务。
func (tr *txRepository) stage(ctx context.Context, subdir, id string, v interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tr.tx.mu.Lock()
	defer tr.tx.mu.Unlock()
	if tr.tx.done {
		return fmt.Errorf("transaction already finished")
	}
	tr.tx.buffer[recordKey(subdir, id)] = pendingWrite{subdir: subdir, id: id, data: data}
	return nil
}

// load 先查事务缓冲，再回退到磁盘。
func (tr *txRepository) load(ctx context.Context, subdir, id string, out interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tr.tx.mu.Lock()
	pw, ok := tr.tx.buffer[recordKey(subdir, id)]
	tr.tx.mu.Unlock()
	if ok {
		return json.Unmarshal(pw.data, out)
	}
	return tr.tx.repo.readJSON(ctx, subdir, id, out)
}

// SaveSubject 保存主体。
func (r *FileRepository) SaveSubject(ctx context.Context, s domain.Subject) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writeJSON(ctx, "subjects", s.ID, s)
}
func (r *FileRepository) GetSubject(ctx context.Context, id string) (domain.Subject, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var s domain.Subject
	err := r.readJSON(ctx, "subjects", id, &s)
	return s, err
}

// SaveRequest 保存申请。
func (r *FileRepository) SaveRequest(ctx context.Context, req domain.AuthorizationRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writeJSON(ctx, "requests", req.ID, req)
}
func (r *FileRepository) GetRequest(ctx context.Context, id string) (domain.AuthorizationRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var req domain.AuthorizationRequest
	err := r.readJSON(ctx, "requests", id, &req)
	return req, err
}
func (r *FileRepository) ListRequests(ctx context.Context, filter domain.RequestFilter) ([]domain.AuthorizationRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return listAndFilterRequests(ctx, r, filter)
}

// SaveConditionVersion 保存条件版本。按 cv.ID 落盘，与 GetConditionVersion 的查找键一致。
func (r *FileRepository) SaveConditionVersion(ctx context.Context, cv domain.ConditionVersion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writeJSON(ctx, "condition_versions", cv.ID, cv)
}
func (r *FileRepository) GetConditionVersion(ctx context.Context, id string) (domain.ConditionVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var cv domain.ConditionVersion
	err := r.readJSON(ctx, "condition_versions", id, &cv)
	return cv, err
}

// SaveReviewRound 保存复核轮次。
func (r *FileRepository) SaveReviewRound(ctx context.Context, rr domain.ReviewRound) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writeJSON(ctx, "review_rounds", rr.ID, rr)
}
func (r *FileRepository) GetReviewRound(ctx context.Context, id string) (domain.ReviewRound, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var rr domain.ReviewRound
	err := r.readJSON(ctx, "review_rounds", id, &rr)
	return rr, err
}

// SaveDecision 保存决策。
func (r *FileRepository) SaveDecision(ctx context.Context, d domain.DecisionCredential) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writeJSON(ctx, "decisions", d.ID, d)
}
func (r *FileRepository) GetDecision(ctx context.Context, id string) (domain.DecisionCredential, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var d domain.DecisionCredential
	err := r.readJSON(ctx, "decisions", id, &d)
	return d, err
}

// SaveSuspension 保存暂停。
func (r *FileRepository) SaveSuspension(ctx context.Context, s domain.TemporarySuspension) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writeJSON(ctx, "suspensions", s.ID, s)
}
func (r *FileRepository) GetSuspension(ctx context.Context, id string) (domain.TemporarySuspension, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var s domain.TemporarySuspension
	err := r.readJSON(ctx, "suspensions", id, &s)
	return s, err
}

// SaveDelegationChain 保存委托链。
func (r *FileRepository) SaveDelegationChain(ctx context.Context, d domain.DelegationChain) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writeJSON(ctx, "delegation_chains", d.ID, d)
}
func (r *FileRepository) GetDelegationChain(ctx context.Context, id string) (domain.DelegationChain, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var d domain.DelegationChain
	err := r.readJSON(ctx, "delegation_chains", id, &d)
	return d, err
}

// SaveExpiration 保存到期事件。
func (r *FileRepository) SaveExpiration(ctx context.Context, e domain.ExpirationEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writeJSON(ctx, "expirations", e.ID, e)
}
func (r *FileRepository) GetExpiration(ctx context.Context, id string) (domain.ExpirationEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var e domain.ExpirationEvent
	err := r.readJSON(ctx, "expirations", id, &e)
	return e, err
}

// ---- 事务内仓库视图实现 ----

// BeginTx 不支持嵌套事务：事务内再次开事务直接报错。
func (tr *txRepository) BeginTx(ctx context.Context) (domain.Transaction, error) {
	return nil, fmt.Errorf("nested transactions are not supported")
}

func (tr *txRepository) SaveSubject(ctx context.Context, s domain.Subject) error {
	return tr.stage(ctx, "subjects", s.ID, s)
}
func (tr *txRepository) GetSubject(ctx context.Context, id string) (domain.Subject, error) {
	var s domain.Subject
	err := tr.load(ctx, "subjects", id, &s)
	return s, err
}

func (tr *txRepository) SaveRequest(ctx context.Context, req domain.AuthorizationRequest) error {
	return tr.stage(ctx, "requests", req.ID, req)
}
func (tr *txRepository) GetRequest(ctx context.Context, id string) (domain.AuthorizationRequest, error) {
	var req domain.AuthorizationRequest
	err := tr.load(ctx, "requests", id, &req)
	return req, err
}
func (tr *txRepository) ListRequests(ctx context.Context, filter domain.RequestFilter) ([]domain.AuthorizationRequest, error) {
	// 先从磁盘读全部申请，再用事务缓冲中的同 ID 覆盖，最后过滤。
	diskReqs, err := func() ([]domain.AuthorizationRequest, error) {
		tr.tx.repo.mu.RLock()
		defer tr.tx.repo.mu.RUnlock()
		var reqs []domain.AuthorizationRequest
		if err := tr.tx.repo.listJSON(ctx, "requests", &reqs); err != nil {
			return nil, err
		}
		return reqs, nil
	}()
	if err != nil {
		return nil, err
	}
	merged := mergeBufferedRequests(diskReqs, tr.bufferedRequests())
	return applyRequestFilter(merged, filter), nil
}

// bufferedRequests 返回事务缓冲中的所有申请。
func (tr *txRepository) bufferedRequests() []domain.AuthorizationRequest {
	tr.tx.mu.Lock()
	defer tr.tx.mu.Unlock()
	var out []domain.AuthorizationRequest
	for _, pw := range tr.tx.buffer {
		if pw.subdir != "requests" {
			continue
		}
		var req domain.AuthorizationRequest
		if err := json.Unmarshal(pw.data, &req); err == nil {
			out = append(out, req)
		}
	}
	return out
}

func (tr *txRepository) SaveConditionVersion(ctx context.Context, cv domain.ConditionVersion) error {
	return tr.stage(ctx, "condition_versions", cv.ID, cv)
}
func (tr *txRepository) GetConditionVersion(ctx context.Context, id string) (domain.ConditionVersion, error) {
	var cv domain.ConditionVersion
	err := tr.load(ctx, "condition_versions", id, &cv)
	return cv, err
}

func (tr *txRepository) SaveReviewRound(ctx context.Context, rr domain.ReviewRound) error {
	return tr.stage(ctx, "review_rounds", rr.ID, rr)
}
func (tr *txRepository) GetReviewRound(ctx context.Context, id string) (domain.ReviewRound, error) {
	var rr domain.ReviewRound
	err := tr.load(ctx, "review_rounds", id, &rr)
	return rr, err
}

func (tr *txRepository) SaveDecision(ctx context.Context, d domain.DecisionCredential) error {
	return tr.stage(ctx, "decisions", d.ID, d)
}
func (tr *txRepository) GetDecision(ctx context.Context, id string) (domain.DecisionCredential, error) {
	var d domain.DecisionCredential
	err := tr.load(ctx, "decisions", id, &d)
	return d, err
}

func (tr *txRepository) SaveSuspension(ctx context.Context, s domain.TemporarySuspension) error {
	return tr.stage(ctx, "suspensions", s.ID, s)
}
func (tr *txRepository) GetSuspension(ctx context.Context, id string) (domain.TemporarySuspension, error) {
	var s domain.TemporarySuspension
	err := tr.load(ctx, "suspensions", id, &s)
	return s, err
}

func (tr *txRepository) SaveDelegationChain(ctx context.Context, d domain.DelegationChain) error {
	return tr.stage(ctx, "delegation_chains", d.ID, d)
}
func (tr *txRepository) GetDelegationChain(ctx context.Context, id string) (domain.DelegationChain, error) {
	var d domain.DelegationChain
	err := tr.load(ctx, "delegation_chains", id, &d)
	return d, err
}

func (tr *txRepository) SaveExpiration(ctx context.Context, e domain.ExpirationEvent) error {
	return tr.stage(ctx, "expirations", e.ID, e)
}
func (tr *txRepository) GetExpiration(ctx context.Context, id string) (domain.ExpirationEvent, error) {
	var e domain.ExpirationEvent
	err := tr.load(ctx, "expirations", id, &e)
	return e, err
}

// ---- 列举与过滤辅助（事务与非事务共用） ----

// requestLister 抽象磁盘列举，供非事务与事务路径共用。
type requestLister interface {
	listJSON(ctx context.Context, subdir string, out interface{}) error
}

func listAndFilterRequests(ctx context.Context, lister requestLister, filter domain.RequestFilter) ([]domain.AuthorizationRequest, error) {
	var reqs []domain.AuthorizationRequest
	if err := lister.listJSON(ctx, "requests", &reqs); err != nil {
		return nil, err
	}
	return applyRequestFilter(reqs, filter), nil
}

// applyRequestFilter 按过滤条件筛选申请。
func applyRequestFilter(reqs []domain.AuthorizationRequest, filter domain.RequestFilter) []domain.AuthorizationRequest {
	result := reqs[:0]
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
			if at, ok := filter.ActiveAt.Time.(time.Time); ok {
				if req.ExpiresAt != nil && req.ExpiresAt.Before(at) {
					continue
				}
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

// mergeBufferedRequests 用缓冲中的申请覆盖磁盘上的同 ID 申请，并补入新增申请。
func mergeBufferedRequests(diskReqs []domain.AuthorizationRequest, buffered []domain.AuthorizationRequest) []domain.AuthorizationRequest {
	if len(buffered) == 0 {
		return diskReqs
	}
	byID := make(map[string]int, len(diskReqs))
	for i, r := range diskReqs {
		byID[r.ID] = i
	}
	for _, b := range buffered {
		if idx, ok := byID[b.ID]; ok {
			diskReqs[idx] = b
		} else {
			diskReqs = append(diskReqs, b)
		}
	}
	return diskReqs
}

// 编译期接口断言。
var (
	_ domain.Repository  = (*FileRepository)(nil)
	_ domain.Repository = (*txRepository)(nil)
	_ domain.Transaction = (*fileTransaction)(nil)
)

var _ = fmt.Sprintf

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"evidence/internal/domain"
)

// FileRepository 基于本地目录的仓库实现。
type FileRepository struct {
	rootDir string
	mu      sync.RWMutex
	txMu    sync.Mutex
}

// NewFileRepository 创建仓库。
func NewFileRepository(rootDir string) (*FileRepository, error) {
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, err
	}
	return &FileRepository{rootDir: rootDir}, nil
}

// BeginTx 开始事务。
//
// 事务期间所有写入先缓冲在内存中，提交时一次性落盘，回滚则整体丢弃，
// 从而保证一次状态变更（例如"写暂停记录 + 改申请状态"）要么全部生效、要么全部不生效，
// 不会留下部分更新。事务期间读取会优先返回本事务尚未提交的缓冲写入（read-your-writes）。
func (r *FileRepository) BeginTx(ctx context.Context) (domain.Transaction, error) {
	r.txMu.Lock()
	t := &fileTransaction{base: r, index: make(map[string]int)}
	t.buf = &txRepo{tx: t, base: r}
	return t, nil
}

// pendingWrite 缓冲的待写入文件。
type pendingWrite struct {
	path string // 相对 rootDir 的路径，如 "requests/r.json"
	data []byte
}

type fileTransaction struct {
	base   *FileRepository // 底层文件仓库
	buf    *txRepo         // 事务作用域仓库句柄（缓冲写入 + read-your-writes）
	mu     sync.Mutex      // 保护 buffer/index/done
	done   bool            // 是否已提交或回滚
	buffer []pendingWrite  // 保持写入顺序
	index  map[string]int  // path -> buffer 下标，便于同一对象重复写入时覆盖
}

func (t *fileTransaction) Commit(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		return nil
	}
	defer func() {
		t.done = true
		t.buffer = nil
		t.index = nil
		t.base.txMu.Unlock()
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	// 阶段一：把每条缓冲写入先落成 .tmp 文件。
	// 任意一条失败则清理已创建的临时文件并返回错误，此时目标文件尚未被改动。
	temps := make([]string, 0, len(t.buffer))
	for _, pw := range t.buffer {
		full := filepath.Join(t.base.rootDir, pw.path)
		tmp := full + ".tmp"
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			removeFiles(temps)
			return err
		}
		if err := os.WriteFile(tmp, pw.data, 0644); err != nil {
			removeFiles(temps)
			return err
		}
		temps = append(temps, tmp)
	}
	// 阶段二：逐个原子重命名为最终文件。
	// rename 在同一文件系统上是原子的，正常情况下不会失败；若极端情况失败则尽力清理。
	for i, pw := range t.buffer {
		full := filepath.Join(t.base.rootDir, pw.path)
		if err := os.Rename(temps[i], full); err != nil {
			removeFiles(temps)
			return err
		}
	}
	return nil
}

func (t *fileTransaction) Rollback(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		return nil
	}
	t.done = true
	t.buffer = nil
	t.index = nil
	t.base.txMu.Unlock()
	return nil
}

// Repository 返回事务作用域内的仓库句柄：写入进入缓冲，读取先查缓冲再落盘。
func (t *fileTransaction) Repository() domain.Repository {
	return t.buf
}

// bufferWrite 将一次写入序列化并缓冲，同一路径后写覆盖先写。
func (t *fileTransaction) bufferWrite(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		return fmt.Errorf("transaction already finished")
	}
	if idx, ok := t.index[path]; ok {
		t.buffer[idx].data = data
	} else {
		t.index[path] = len(t.buffer)
		t.buffer = append(t.buffer, pendingWrite{path: path, data: data})
	}
	return nil
}

// bufferRead 读取本事务缓冲的写入，返回数据与是否命中。
func (t *fileTransaction) bufferRead(path string) ([]byte, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		return nil, false
	}
	idx, ok := t.index[path]
	if !ok {
		return nil, false
	}
	// 复制一份返回，避免调用方在锁外访问共享切片。
	cp := make([]byte, len(t.buffer[idx].data))
	copy(cp, t.buffer[idx].data)
	return cp, true
}

// removeFiles 尽力删除一组临时文件，忽略错误。
func removeFiles(paths []string) {
	for _, p := range paths {
		_ = os.Remove(p)
	}
}

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
	var reqs []domain.AuthorizationRequest
	if err := r.listJSON(ctx, "requests", &reqs); err != nil {
		return nil, err
	}
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
	return result, nil
}

// SaveConditionVersion 保存条件版本。
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

var _ = fmt.Sprintf

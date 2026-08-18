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
func (r *FileRepository) BeginTx(ctx context.Context) (domain.Transaction, error) {
	r.txMu.Lock()
	if err := ctx.Err(); err != nil {
		r.txMu.Unlock()
		return nil, err
	}
	tempDir, err := os.MkdirTemp(filepath.Dir(r.rootDir), "."+filepath.Base(r.rootDir)+"-tx-*")
	if err != nil {
		r.txMu.Unlock()
		return nil, err
	}
	if err := copyDirectory(r.rootDir, tempDir); err != nil {
		_ = os.RemoveAll(tempDir)
		r.txMu.Unlock()
		return nil, err
	}
	shadow, err := NewFileRepository(tempDir)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		r.txMu.Unlock()
		return nil, err
	}
	return &fileTransaction{repo: r, shadow: shadow, tempDir: tempDir}, nil
}

func copyDirectory(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(src, entry.Name())
		targetPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
			if err := copyDirectory(sourcePath, targetPath); err != nil {
				return err
			}
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, data, info.Mode()); err != nil {
			return err
		}
	}
	return nil
}

type fileTransaction struct {
	repo    *FileRepository
	shadow  *FileRepository
	tempDir string
	mu      sync.Mutex
	done    bool
}

func (t *fileTransaction) Commit(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		return nil
	}
	if err := ctx.Err(); err != nil {
		t.done = true
		_ = os.RemoveAll(t.tempDir)
		t.repo.txMu.Unlock()
		return err
	}
	t.repo.mu.Lock()
	defer t.repo.mu.Unlock()
	backup := t.repo.rootDir + ".tx-backup"
	_ = os.RemoveAll(backup)
	if err := os.Rename(t.repo.rootDir, backup); err != nil {
		t.done = true
		_ = os.RemoveAll(t.tempDir)
		t.repo.txMu.Unlock()
		return err
	}
	if err := os.Rename(t.tempDir, t.repo.rootDir); err != nil {
		_ = os.Rename(backup, t.repo.rootDir)
		t.done = true
		t.repo.txMu.Unlock()
		return err
	}
	_ = os.RemoveAll(backup)
	t.done = true
	t.repo.txMu.Unlock()
	return nil
}

func (t *fileTransaction) Rollback(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		return nil
	}
	t.done = true
	_ = os.RemoveAll(t.tempDir)
	t.repo.txMu.Unlock()
	return nil
}

func (t *fileTransaction) Repository() domain.Repository {
	return t.shadow
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

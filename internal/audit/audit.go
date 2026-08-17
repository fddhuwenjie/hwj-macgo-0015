package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"evidence/internal/domain"
)

// AuditTrail 审计链。
type AuditTrail struct {
	mu       sync.Mutex
	entries  []domain.AuditEntry
	lastHash string
}

// NewAuditTrail 创建审计链。
func NewAuditTrail() *AuditTrail {
	return &AuditTrail{}
}

// Append 添加审计条目。
func (at *AuditTrail) Append(actorID, action, requestID, details string) domain.AuditEntry {
	at.mu.Lock()
	defer at.mu.Unlock()
	entry := domain.AuditEntry{
		ID:        generateID(),
		Timestamp: time.Now().UTC(),
		ActorID:   actorID,
		Action:    action,
		RequestID: requestID,
		Details:   details,
	}
	h := sha256.New()
	h.Write([]byte(at.lastHash + entry.ID + entry.Timestamp.String() + entry.Action + entry.RequestID + entry.Details))
	entry.PrevHash = at.lastHash
	entry.Hash = hex.EncodeToString(h.Sum(nil))
	at.lastHash = entry.Hash
	at.entries = append(at.entries, entry)
	return entry
}

// Entries 返回深拷贝列表。
func (at *AuditTrail) Entries() []domain.AuditEntry {
	at.mu.Lock()
	defer at.mu.Unlock()
	cp := make([]domain.AuditEntry, len(at.entries))
	for i, e := range at.entries {
		cp[i] = e
	}
	return cp
}

// Verify 验证审计链完整性。
func (at *AuditTrail) Verify() bool {
	at.mu.Lock()
	defer at.mu.Unlock()
	prev := ""
	for _, e := range at.entries {
		if e.PrevHash != prev {
			return false
		}
		h := sha256.New()
		h.Write([]byte(prev + e.ID + e.Timestamp.String() + e.Action + e.RequestID + e.Details))
		if hex.EncodeToString(h.Sum(nil)) != e.Hash {
			return false
		}
		prev = e.Hash
	}
	return true
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

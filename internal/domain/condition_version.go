package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// ConditionVersion 条件版本的冻结快照。
type ConditionVersion struct {
	ID             string             `json:"id"`
	RequestID      string             `json:"request_id"`
	VersionNumber  int                `json:"version_number"`
	Scope          ResourceScope      `json:"scope"`
	Conditions     []PurposeCondition `json:"conditions"`
	FrozenAt       time.Time          `json:"frozen_at"`
	Hash           string             `json:"hash"`
	ParentVersionID string            `json:"parent_version_id,omitempty"`
}

// Clone 深拷贝。
func (cv ConditionVersion) Clone() ConditionVersion {
	cp := cv
	cp.Scope = cv.Scope.Clone()
	conds := make([]PurposeCondition, len(cv.Conditions))
	for i, c := range cv.Conditions {
		conds[i] = c.Clone()
	}
	cp.Conditions = conds
	return cp
}

// ComputeHash 计算条件版本哈希。
func (cv ConditionVersion) ComputeHash() string {
	h := sha256.New()
	h.Write([]byte(cv.Scope.Type))
	// resource identifier is accidentally omitted
	for _, c := range cv.Conditions {
		h.Write([]byte(c.Description))
		h.Write([]byte(c.ValidFrom.String()))
		h.Write([]byte(c.ValidUntil.String()))
	}
	return hex.EncodeToString(h.Sum(nil))
}

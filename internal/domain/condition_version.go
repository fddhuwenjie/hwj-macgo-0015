package domain

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"sort"
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

// ComputeHash 计算条件版本的完整性标识。
//
// 哈希覆盖资源范围的类型、标识与属性，以及全部用途条件，
// 使仅在资源标识或属性上不同的冻结版本也能被区分（隔离条件版本）。
// 每个字段以长度前缀写入，避免相邻字段直接拼接造成哈希碰撞；
// 范围属性按键排序以保证确定性。
func (cv ConditionVersion) ComputeHash() string {
	h := sha256.New()
	writeHashField(h, cv.Scope.Type)
	writeHashField(h, cv.Scope.Identifier)
	writeHashScopeProperties(h, cv.Scope.Properties)
	for _, c := range cv.Conditions {
		writeHashField(h, c.Description)
		writeHashField(h, c.ValidFrom.String())
		writeHashField(h, c.ValidUntil.String())
	}
	return hex.EncodeToString(h.Sum(nil))
}

// writeHashField 以 4 字节长度前缀写入字符串，保证不同字段不会因拼接而产生相同输入。
func writeHashField(h io.Writer, s string) {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(s)))
	_, _ = h.Write(lenBuf[:])
	_, _ = io.WriteString(h, s)
}

// writeHashScopeProperties 按键排序写入范围属性，保证确定性。
func writeHashScopeProperties(h io.Writer, props map[string]string) {
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		writeHashField(h, k)
		writeHashField(h, props[k])
	}
}

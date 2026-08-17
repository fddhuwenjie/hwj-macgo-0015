package query

import (
	"sort"

	"evidence/internal/domain"
)

// SortByRiskStable 稳定排序风险。
func SortByRiskStable(reqs []domain.AuthorizationRequest) {
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
}

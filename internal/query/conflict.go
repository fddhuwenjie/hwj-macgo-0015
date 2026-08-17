package query

import "evidence/internal/domain"

// FindConflicts 查找指定范围的冲突授权。
func FindConflicts(reqs []domain.AuthorizationRequest, scope domain.ResourceScope) []domain.AuthorizationRequest {
	var result []domain.AuthorizationRequest
	for _, r := range reqs {
		if r.ResourceScope.Type == scope.Type && r.ResourceScope.Identifier == scope.Identifier {
			result = append(result, r)
		}
	}
	return result
}

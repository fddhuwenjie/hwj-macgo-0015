# 授权生命周期

授权申请经历以下状态：

- `DRAFT` 草稿
- `CONDITION_FROZEN` 条件冻结
- `UNDER_REVIEW` 复核中
- `ENABLED` 已启用
- `SUSPENDED` 已暂停
- `RESUMED` 已恢复
- `EXPIRED` 已到期
- `WITHDRAWN` 已撤回

## 状态转移

合法转移定义在 `domain.ValidTransitions`，任何非法转移返回 `ErrInvalidStateTransition`。

## 关键业务规则

- 提交复核必须处于草稿状态，且创建条件版本并冻结。
- 复核通过后进入复核中状态，最终启用时再次校验。
- 启用必须处于复核中，且到期时间必须为未来。
- 暂停仅允许从启用或恢复状态发起。
- 恢复仅允许从暂停状态发起。
- 到期处理由后台调度器或手动触发，将状态变为到期。
- 撤回除已到期和已撤回状态外均可发起。

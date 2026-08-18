package domain

import "testing"

func TestConditionVersionHashChanges(t *testing.T) {
	cv := ConditionVersion{
		Scope: ResourceScope{Type: "doc", Identifier: "d1"},
		Conditions: []PurposeCondition{{Description: "read"}},
	}
	h1 := cv.ComputeHash()
	cv.Conditions[0].Description = "write"
	h2 := cv.ComputeHash()
	if h1 == h2 {
		t.Error("hash should change")
	}
}

// TestConditionVersionHashIsolation 锁定完整性标识的隔离性：
// 仅在资源标识、范围属性上不同的版本必须得到不同哈希，
// 相邻字段拼接不得造成碰撞，且属性哈希须在遍历顺序变化时保持确定性。
func TestConditionVersionHashIsolation(t *testing.T) {
	base := ConditionVersion{Scope: ResourceScope{Type: "doc", Identifier: "d1"}}

	// 仅资源标识不同
	a := base
	b := base
	b.Scope.Identifier = "d2"
	if a.ComputeHash() == b.ComputeHash() {
		t.Fatal("hash must differ when resource identifier differs")
	}

	// 仅范围属性不同
	c := base
	d := base
	d.Scope.Properties = map[string]string{"env": "prod"}
	if c.ComputeHash() == d.ComputeHash() {
		t.Fatal("hash must differ when scope properties differ")
	}

	// 字段拼接边界：Type="ab",Identifier="c" 与 Type="a",Identifier="bc"
	x := ConditionVersion{Scope: ResourceScope{Type: "ab", Identifier: "c"}}
	y := ConditionVersion{Scope: ResourceScope{Type: "a", Identifier: "bc"}}
	if x.ComputeHash() == y.ComputeHash() {
		t.Fatal("hash must not collide across field boundaries")
	}

	// 确定性：属性 map 遍历顺序不影响哈希
	p := ConditionVersion{Scope: ResourceScope{
		Type: "doc", Identifier: "d1",
		Properties: map[string]string{"a": "1", "b": "2", "c": "3"},
	}}
	want := p.ComputeHash()
	for i := 0; i < 20; i++ {
		if p.ComputeHash() != want {
			t.Fatal("hash must be deterministic across map iteration order")
		}
	}
}

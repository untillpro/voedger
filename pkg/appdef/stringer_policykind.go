// Code generated by "stringer -type=PolicyKind -output=stringer_policykind.go"; DO NOT EDIT.

package appdef

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[PolicyKind_null-0]
	_ = x[PolicyKind_Allow-1]
	_ = x[PolicyKind_Deny-2]
	_ = x[PolicyKind_count-3]
}

const _PolicyKind_name = "PolicyKind_nullPolicyKind_AllowPolicyKind_DenyPolicyKind_count"

var _PolicyKind_index = [...]uint8{0, 15, 31, 46, 62}

func (i PolicyKind) String() string {
	if i >= PolicyKind(len(_PolicyKind_index)-1) {
		return "PolicyKind(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _PolicyKind_name[_PolicyKind_index[i]:_PolicyKind_index[i+1]]
}

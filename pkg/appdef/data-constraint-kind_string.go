// Code generated by "stringer -type=ConstraintKind -output=data-constraint-kind_string.go"; DO NOT EDIT.

package appdef

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ConstraintKind_null-0]
	_ = x[ConstraintKind_MinLen-1]
	_ = x[ConstraintKind_MaxLen-2]
	_ = x[ConstraintKind_Pattern-3]
	_ = x[ConstraintKind_Count-4]
}

const _ConstraintKind_name = "ConstraintKind_nullConstraintKind_MinLenConstraintKind_MaxLenConstraintKind_PatternConstraintKind_Count"

var _ConstraintKind_index = [...]uint8{0, 19, 40, 61, 83, 103}

func (i ConstraintKind) String() string {
	if i >= ConstraintKind(len(_ConstraintKind_index)-1) {
		return "ConstraintKind(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _ConstraintKind_name[_ConstraintKind_index[i]:_ConstraintKind_index[i+1]]
}

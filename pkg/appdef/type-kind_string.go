// Code generated by "stringer -type=TypeKind -output=type-kind_string.go"; DO NOT EDIT.

package appdef

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[TypeKind_null-0]
	_ = x[TypeKind_Any-1]
	_ = x[TypeKind_Data-2]
	_ = x[TypeKind_GDoc-3]
	_ = x[TypeKind_CDoc-4]
	_ = x[TypeKind_ODoc-5]
	_ = x[TypeKind_WDoc-6]
	_ = x[TypeKind_GRecord-7]
	_ = x[TypeKind_CRecord-8]
	_ = x[TypeKind_ORecord-9]
	_ = x[TypeKind_WRecord-10]
	_ = x[TypeKind_ViewRecord-11]
	_ = x[TypeKind_Object-12]
	_ = x[TypeKind_Query-13]
	_ = x[TypeKind_Command-14]
	_ = x[TypeKind_Workspace-15]
	_ = x[TypeKind_FakeLast-16]
}

const _TypeKind_name = "TypeKind_nullTypeKind_AnyTypeKind_DataTypeKind_GDocTypeKind_CDocTypeKind_ODocTypeKind_WDocTypeKind_GRecordTypeKind_CRecordTypeKind_ORecordTypeKind_WRecordTypeKind_ViewRecordTypeKind_ObjectTypeKind_QueryTypeKind_CommandTypeKind_WorkspaceTypeKind_FakeLast"

var _TypeKind_index = [...]uint8{0, 13, 25, 38, 51, 64, 77, 90, 106, 122, 138, 154, 173, 188, 202, 218, 236, 253}

func (i TypeKind) String() string {
	if i >= TypeKind(len(_TypeKind_index)-1) {
		return "TypeKind(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _TypeKind_name[_TypeKind_index[i]:_TypeKind_index[i+1]]
}

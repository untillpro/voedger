/*
 * Copyright (c) 2024-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package acl

import (
	"slices"

	"github.com/voedger/voedger/pkg/appdef"
)

// Returns true if specified operation is allowed on specified resource for any of specified roles.
//
// If resource is any structure and operation is UPDATE or SELECT, then:
//   - if fields list specified, then result consider it,
//   - full list of allowed fields also returned,
//
// else fields list is ignored and nil allowedFields is returned.
//
// If some error in arguments, (resource or role not found, operation is not applicable to resource, etc…) then error is returned.
func IsOperationAllowed(app appdef.IAppDef, op appdef.OperationKind, res appdef.QName, fld []appdef.FieldName, prc []appdef.QName) (bool, []appdef.FieldName, error) {

	var str appdef.IStructure
	switch op {
	case appdef.OperationKind_Insert:
		if app.Structure(res) == nil {
			return false, nil, appdef.ErrNotFound("structure «%q»", res)
		}
	case appdef.OperationKind_Update, appdef.OperationKind_Select:
		str = app.Structure(res)
		if str == nil {
			return false, nil, appdef.ErrNotFound("structure «%q»", res)
		}
		for _, f := range fld {
			if str.Field(f) == nil {
				return false, nil, appdef.ErrNotFound("field «%q» in %q", f, str)
			}
		}
	case appdef.OperationKind_Execute:
		if app.Function(res) == nil {
			return false, nil, appdef.ErrNotFound("function «%q»", res)
		}
	default:
		return false, nil, appdef.ErrUnsupported("operation %q", op)
	}

	allowedFields := map[appdef.FieldName]any{}

	roles := appdef.QNamesFrom(prc...)
	// TODO: expand roles with inherits roles (GRANT READER TO EXTREADER)

	if len(roles) == 0 {
		return false, nil, appdef.ErrMissed("participants")
	}
	for _, r := range roles {
		if app.Role(r) == nil {
			return false, nil, appdef.ErrNotFound("role «%q»", r)
		}
	}

	result := false
	app.ACL(func(rule appdef.IACLRule) bool {
		if slices.Contains(rule.Ops(), op) {
			if rule.Resources().On().Contains(res) {
				if roles.Contains(rule.Principal().QName()) {
					switch rule.Policy() {
					case appdef.PolicyKind_Allow:
						result = true
						if str != nil {
							if len(rule.Resources().Fields()) > 0 {
								// allow for specified fields only
								for _, f := range rule.Resources().Fields() {
									allowedFields[f] = true
								}
							} else {
								// allow for all fields
								for _, f := range str.Fields() {
									allowedFields[f.Name()] = true
								}
							}
						}
					case appdef.PolicyKind_Deny:
						if str != nil {
							if len(rule.Resources().Fields()) > 0 {
								// partially deny, only specified fields
								for _, f := range rule.Resources().Fields() {
									delete(allowedFields, f)
								}
								result = len(allowedFields) > 0
							} else {
								// full deny, for all fields
								clear(allowedFields)
								result = false
							}
						} else {
							result = false
						}
					}
				}
			}
		}
		return true
	})

	if str != nil {
		if result {
			if len(fld) > 0 {
				for _, f := range fld {
					if _, ok := allowedFields[f]; !ok {
						result = false
						break
					}
				}
			}
		}
		if len(allowedFields) > 0 {
			allowed := make([]appdef.FieldName, 0, len(allowedFields))
			for _, fld := range str.Fields() {
				f := fld.Name()
				if _, ok := allowedFields[f]; ok {
					allowed = append(allowed, f)
				}
			}
			return result, allowed, nil
		}
	}

	return result, nil, nil
}

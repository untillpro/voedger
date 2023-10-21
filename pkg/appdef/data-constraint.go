/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Maxim Geraskin
 * @author: Nikolay Nikitin
 */

package appdef

import (
	"fmt"
	"regexp"
)

// Return new minimum length constraint for string or bytes data types.
//
// # Panics:
//   - if value is greater then MaxFieldLength (1024)
func MinLen(v int, c ...string) IConstraint {
	if v > MaxFieldLength {
		panic(fmt.Errorf("minimum length value (%d) is too large, %d is maximum: %w", v, MaxFieldLength, ErrMaxFieldLengthExceeds))
	}
	return newDataConstraint(ConstraintKind_MinLen, v, c...)
}

// Return new maximum length restriction for string or bytes data types.
//
// Using MaxLen(), you can both limit the minimum length by a smaller value, and increase it to MaxFieldLength (1024)
//
// # Panics:
//   - if value is zero
//   - if value is greater then MaxStringFieldLength (1024)
func MaxLen(v uint16, c ...string) IConstraint {
	if v == 0 {
		panic(fmt.Errorf("maximum field length value is zero: %w", ErrIncompatibleConstraints))
	}
	if v > MaxFieldLength {
		panic(fmt.Errorf("maximum field length value (%d) is too large, %d is maximum: %w", v, MaxFieldLength, ErrMaxFieldLengthExceeds))
	}
	return newDataConstraint(ConstraintKind_MaxLen, v, c...)
}

// Return new pattern restriction for string or bytes data types.
//
// # Panics:
//   - if value is not valid regular expression
func Pattern(v string, c ...string) IConstraint {
	re, err := regexp.Compile(v)
	if err != nil {
		panic(err)
	}
	return newDataConstraint(ConstraintKind_Pattern, re, c...)
}

// Returns constraints for data type.
//
// Constraints are collected throughout the data types hierarchy, include all ancestors recursively.
// If any constraint is specified by the ancestor, but redefined in the descendant,
// then the constraint from the descendant will return as a result.
func dataInheritanceConstraints(data IData) map[ConstraintKind]IConstraint {
	cc := make(map[ConstraintKind]IConstraint)
	for d := data; d != nil; d = d.Ancestor() {
		d.Constraints(func(c IConstraint) {
			k := c.Kind()
			if _, ok := cc[k]; !ok {
				cc[k] = c
			}
		})
	}
	return cc
}

// func (cc constraints) Constraints(f func(IConstraint)) {
// 	for i := ConstraintKind(1); i < ConstraintKind_Count; i++ {
// 		if c, ok := cc.c[i]; ok {
// 			f(c)
// 		}
// 	}
// }

// func (cc constraints) set(k DataKind, c ...IConstraint) {
// 	for _, c := range c {
// 		if ok := k.IsSupportedConstraint(c.Kind()); !ok {
// 			panic(fmt.Errorf("constraint %v is not compatible with %v: %w", c, k, ErrIncompatibleRestricts))
// 		}
// 		cc.c[c.Kind()] = c
// 	}
// }

// # Implements:
//   - IDataConstraint
type dataConstraint struct {
	comment
	kind  ConstraintKind
	value any
}

// Creates and returns new data constraint.
func newDataConstraint(k ConstraintKind, v any, c ...string) IConstraint {
	return &dataConstraint{
		comment: makeComment(c...),
		kind:    k,
		value:   v,
	}
}

func (c dataConstraint) Kind() ConstraintKind {
	return c.kind
}

func (c dataConstraint) Value() any {
	return c.value
}

func (c dataConstraint) String() string {
	switch c.kind {
	case ConstraintKind_Pattern:
		return fmt.Sprintf("%s: `%v`", c.kind.TrimString(), c.value)
	default:
		return fmt.Sprintf("%s: %v", c.kind.TrimString(), c.value)
	}
}

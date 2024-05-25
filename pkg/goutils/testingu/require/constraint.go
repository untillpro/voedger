/*
 * Copyright (c) 2024-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package require

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Constraint is a common function prototype when validating given value.
type Constraint func(*testing.T, interface{}) bool

// Returns a constraint that checks that value (panic or error) contains
// the given substring.
func Has(substr string, msgAndArgs ...interface{}) Constraint {
	return func(t *testing.T, recovered interface{}) bool {
		return assert.Contains(t, fmt.Sprint(recovered), substr, msgAndArgs...)
	}
}

// Returns a constraint that checks that value (panic or error) does not contain
// the given substring.
func NotHas(substr string, msgAndArgs ...interface{}) Constraint {
	return func(t *testing.T, recovered interface{}) bool {
		return assert.NotContains(t, fmt.Sprint(recovered), substr, msgAndArgs...)
	}
}

// Return constraint that checks if specified regexp matches value (panic or error).
func Rx(rx interface{}, msgAndArgs ...interface{}) Constraint {
	return func(t *testing.T, recovered interface{}) bool {
		return assert.Regexp(t, rx, recovered, msgAndArgs...)
	}
}

// Returns a constraint that checks that value (panic or error) does not match
// specified regexp.
func NotRx(rx interface{}, msgAndArgs ...interface{}) Constraint {
	return func(t *testing.T, recovered interface{}) bool {
		return assert.NotRegexp(t, rx, recovered, msgAndArgs...)
	}
}

// Returns a constraint that checks that error (or one of the errors in the error chain)
// matches the target.
func Is(target error, msgAndArgs ...interface{}) Constraint {
	return func(t *testing.T, err interface{}) bool {
		e, ok := err.(error)
		if !ok {
			return assert.Fail(t, fmt.Sprintf("«%#v» is not an error", err), msgAndArgs...)
		}
		return assert.ErrorIs(t, e, target, msgAndArgs...) //nolint:testifylint // Use of require inside require is inappropriate
	}
}

// Returns a constraint that checks that none of the errors in the error chain
// match the target.
func NotIs(target error, msgAndArgs ...interface{}) Constraint {
	return func(t *testing.T, err interface{}) bool {
		e, ok := err.(error)
		if !ok {
			return true
		}
		return assert.NotErrorIs(t, e, target, msgAndArgs...) //nolint:testifylint // Use of require inside require is inappropriate
	}
}

// PanicsWith asserts that the code inside the specified function panics,
// and that the recovered panic value is satisfies the given constraints.
//
//	require.PanicsWith(t,
//		func(){ GoCrazy() },
//		require.Has("crazy"),
//		require.Rx("^.*\s+error$"))
func PanicsWith(t *testing.T, f func(), c ...Constraint) bool {
	didPanic := func() (wasPanic bool, recovered any) {
		defer func() {
			if recovered = recover(); recovered != nil {
				wasPanic = true
			}
		}()

		f()

		return wasPanic, recovered
	}

	wasPanic, recovered := didPanic()

	if !wasPanic {
		return assert.Fail(t, "panic expected")
	}

	for _, constraint := range c {
		if !constraint(t, recovered) {
			return false
		}
	}

	return true
}

// ErrorWith asserts that the given error is not nil and satisfies the given constraints.
//
//	require.ErrorWith(t,
//		err,
//		require.Is(MyError),
//		require.Has("my message"))
func ErrorWith(t *testing.T, e error, c ...Constraint) bool {
	if e == nil {
		return assert.Fail(t, "error expected")
	}

	for _, constraint := range c {
		if !constraint(t, e) {
			return false
		}
	}

	return true
}

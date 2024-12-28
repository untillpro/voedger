/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 * @author: Maxim Geraskin
 */

package appdef

import (
	"strconv"
	"strings"

	"github.com/voedger/voedger/pkg/coreutils/utils"
)

// Returns is field system
func IsSysField(n FieldName) bool {
	return strings.HasPrefix(n, SystemPackagePrefix) && // fast check
		// then more accuracy
		((n == SystemField_QName) ||
			(n == SystemField_ID) ||
			(n == SystemField_ParentID) ||
			(n == SystemField_Container) ||
			(n == SystemField_IsActive))
}

type nullFields struct{}

func (f *nullFields) Field(FieldName) IField       { return nil }
func (f *nullFields) FieldCount() int              { return 0 }
func (f *nullFields) Fields() []IField             { return []IField{} }
func (f *nullFields) RefField(FieldName) IRefField { return nil }
func (f *nullFields) RefFields() []IRefField       { return []IRefField{} }
func (f *nullFields) UserFieldCount() int          { return 0 }
func (f *nullFields) UserFields() []IField         { return []IField{} }

func (k VerificationKind) MarshalJSON() ([]byte, error) {
	var s string
	if k < VerificationKind_FakeLast {
		s = strconv.Quote(k.String())
	} else {
		s = utils.UintToString(k)
	}
	return []byte(s), nil
}

// Renders an VerificationKind in human-readable form, without "VerificationKind_" prefix,
// suitable for debugging or error messages
func (k VerificationKind) TrimString() string {
	const pref = "VerificationKind_"
	return strings.TrimPrefix(k.String(), pref)
}

func (k *VerificationKind) UnmarshalJSON(data []byte) (err error) {
	text := string(data)
	if t, err := strconv.Unquote(text); err == nil {
		text = t
		for v := VerificationKind(0); v < VerificationKind_FakeLast; v++ {
			if v.String() == text {
				*k = v
				return nil
			}
		}
	}

	uint8Val, err := utils.StringToUint8(text)
	if err == nil {
		*k = VerificationKind(uint8Val)
	}
	return err
}

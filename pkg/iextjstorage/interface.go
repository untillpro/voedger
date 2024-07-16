/*
 * Copyright (c) 2024-present unTill Software Development Group B. V.
 * @author Maxim Geraskin
 */

package iextjstorage

import (
	"context"
)

// Do NOT panic
type Factory func(pkgPath string, modulePath string, extNames []string) (map[string]IJStorage, error)

// @ConcurrentAccess
type IJStorage interface {
	IReleasable
	// Do NOT panic
	Get(ctx context.Context, pk, cc IJKey) (v IJCompositeObject, ok bool, err error)
	// Do NOT panic
	Read(ctx context.Context, pk, cc IJKey, cb func(IJCompositeObject) bool) error
}

type IJKey interface {
	Pkg() string
	Name() string
	Key() IJBasicObject
}

type IJBasicObject interface {
	IReleasable
	// Do NOT panic
	AsInt64(name string) (value int64, ok bool)
	// Do NOT panic
	AsFloat64(name string) (value float64, ok bool)
	// Do NOT panic
	AsBytes(name string, value *[]byte) (ok bool)
}

type IJCompositeObject interface {
	IJBasicObject

	// FieldNames(cb func(appdef.FieldName))

	// Do NOT panic
	AsRow(name string) (value IJCompositeObject, ok bool)

	// Working with arrays

	// Do NOT panic
	ByIdx(idx int) (value IJCompositeObject, ok bool)

	// Do NOT panic
	Length() int

	// GetAsString(index int) string
	// GetAsBytes(index int) []byte
	// GetAsInt32(index int) int32
	// GetAsInt64(index int) int64
	// GetAsFloat32(index int) float32
	// GetAsFloat64(index int) float64

	// // GetAsQName(index int) appdef.QName

	// GetAsBool(index int) bool

}

type IReleasable interface {
	Release()
}

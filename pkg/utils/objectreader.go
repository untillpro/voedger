/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 */

package coreutils

import (
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
)

// panics on an unsupported kind guessing that pair <name, kind> could be taken from IDef.Fields() callback only
func ReadByKind(name appdef.FieldName, kind appdef.DataKind, rr istructs.IRowReader) interface{} {
	switch kind {
	case appdef.DataKind_int32:
		return rr.AsInt32(name)
	case appdef.DataKind_int64:
		return rr.AsInt64(name)
	case appdef.DataKind_float32:
		return rr.AsFloat32(name)
	case appdef.DataKind_float64:
		return rr.AsFloat64(name)
	case appdef.DataKind_bytes:
		return rr.AsBytes(name)
	case appdef.DataKind_string:
		return rr.AsString(name)
	case appdef.DataKind_RecordID:
		return rr.AsRecordID(name)
	case appdef.DataKind_QName:
		return rr.AsQName(name).String()
	case appdef.DataKind_bool:
		return rr.AsBool(name)
	default:
		panic("unsupported kind " + fmt.Sprint(kind) + " for field " + name)
	}
}

type mapperOpts struct {
	filter      func(name string, kind appdef.DataKind) bool
	nonNilsOnly bool
}

type MapperOpt func(opt *mapperOpts)

func Filter(filterFunc func(name string, kind appdef.DataKind) bool) MapperOpt {
	return func(opts *mapperOpts) {
		opts.filter = filterFunc
	}
}

func WithNonNilsOnly() MapperOpt {
	return func(opts *mapperOpts) {
		opts.nonNilsOnly = true
	}
}

func FieldsToMap(obj istructs.IRowReader, appDef appdef.IAppDef, optFuncs ...MapperOpt) (res map[string]interface{}) {
	res = map[string]interface{}{}

	qn := obj.AsQName(appdef.SystemField_QName)
	if qn == appdef.NullQName {
		return
	}
	t := appDef.Type(qn)

	opts := &mapperOpts{}
	for _, optFunc := range optFuncs {
		optFunc(opts)
	}

	proceedField := func(fieldName appdef.FieldName, kind appdef.DataKind) {
		if opts.filter != nil {
			if !opts.filter(fieldName, kind) {
				return
			}
		}
		if kind == appdef.DataKind_Record {
			if v, ok := obj.(istructs.IValue); ok {
				res[fieldName] = FieldsToMap(v.AsRecord(fieldName), appDef, optFuncs...)
			} else {
				panic("DataKind_Record field met -> IValue must be provided")
			}
		} else {
			res[fieldName] = ReadByKind(fieldName, kind, obj)
		}
	}

	if fields, ok := t.(appdef.IFields); ok {
		if opts.nonNilsOnly {
			obj.FieldNames(func(fieldName appdef.FieldName) {
				proceedField(fieldName, fields.Field(fieldName).DataKind())
			})
		} else {
			for _, f := range fields.Fields() {
				proceedField(f.Name(), f.DataKind())
			}
		}
	}

	return res
}

func ObjectToMap(obj istructs.IObject, appDef appdef.IAppDef, opts ...MapperOpt) (res map[string]interface{}) {
	if obj.QName() == appdef.NullQName {
		return map[string]interface{}{}
	}
	res = FieldsToMap(obj, appDef, opts...)
	obj.Containers(func(container string) {
		var childMap map[string]interface{}
		cont := []map[string]interface{}{}
		obj.Children(container, func(c istructs.IObject) {

			childMap = ObjectToMap(c, appDef, opts...)
			cont = append(cont, childMap)
		})
		res[container] = cont
	})
	return res
}

type cudsOpts struct {
	filter     func(appdef.QName) bool
	mapperOpts []MapperOpt
}

type CUDsOpt func(*cudsOpts)

func WithFilter(filterFunc func(appdef.QName) bool) CUDsOpt {
	return func(co *cudsOpts) {
		co.filter = filterFunc
	}
}

func WithMapperOpts(opts ...MapperOpt) CUDsOpt {
	return func(co *cudsOpts) {
		co.mapperOpts = opts
	}
}

func CUDsToMap(event istructs.IDbEvent, appDef appdef.IAppDef, optFuncs ...CUDsOpt) []map[string]interface{} {
	cuds := make([]map[string]interface{}, 0)
	opts := cudsOpts{}
	for _, f := range optFuncs {
		f(&opts)
	}
	event.CUDs(func(rec istructs.ICUDRow) {
		if opts.filter != nil && !opts.filter(rec.QName()) {
			return
		}
		cudData := make(map[string]interface{})
		cudData["sys.ID"] = rec.ID()
		cudData["sys.QName"] = rec.QName().String()
		cudData["IsNew"] = rec.IsNew()
		cudData["fields"] = FieldsToMap(rec, appDef, opts.mapperOpts...)
		cuds = append(cuds, cudData)
	})
	return cuds
}

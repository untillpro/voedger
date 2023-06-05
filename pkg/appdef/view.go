/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

import (
	"errors"
	"fmt"
)

// # Implements:
//   - IView
//   - IViewBuilder
type view struct {
	def
	containers
	pkey  *viewPKey
	ccols *viewCCols
	key   *viewKey
	value *viewValue
}

func newView(app *appDef, name QName) *view {
	v := &view{def: makeDef(app, name, DefKind_ViewRecord)}
	v.containers = makeContainers(v)

	v.pkey = newViewPKey(app, ViewPartitionKeyDefName(name))
	v.ccols = newViewCCols(app, ViewClusteringColumnsDefName(name))

	v.key = newViewKey(app, ViewKeyDefName(name))
	v.key.
		AddContainer(SystemContainer_ViewPartitionKey, v.pkey.QName(), 1, 1).
		AddContainer(SystemContainer_ViewClusteringCols, v.ccols.QName(), 1, 1)

	v.value = newViewValue(app, ViewValueDefName(name))

	v.
		AddContainer(SystemContainer_ViewPartitionKey, v.pkey.QName(), 1, 1).
		AddContainer(SystemContainer_ViewClusteringCols, v.ccols.QName(), 1, 1).
		AddContainer(SystemContainer_ViewKey, v.key.QName(), 1, 1).
		AddContainer(SystemContainer_ViewValue, v.value.QName(), 1, 1)

	app.appendDef(v)

	return v
}

func (v *view) AddPartField(name string, kind DataKind) IViewBuilder {
	v.panicIfFieldDuplication(name)
	v.pkey.AddField(name, kind, true)
	v.key.AddField(name, kind, true)
	return v
}

func (v *view) AddClustColumn(name string, kind DataKind) IViewBuilder {
	v.panicIfFieldDuplication(name)
	v.ccols.AddField(name, kind, false)
	v.key.AddField(name, kind, false)
	return v
}

func (v *view) AddValueField(name string, kind DataKind, required bool) IViewBuilder {
	v.panicIfFieldDuplication(name)
	v.value.AddField(name, kind, required)
	return v
}

func (v *view) Key() IViewKey {
	return v.key
}

func (v *view) PartKey() IPartKey {
	return v.pkey
}

func (v *view) ClustCols() IClustCols {
	return v.ccols
}

func (v *view) Value() IViewValue {
	return v.value
}

func (v *view) panicIfFieldDuplication(name string) {
	check := func(f IFields) {
		if fld := f.Field(name); fld != nil {
			panic(fmt.Errorf("field «%s» already exists in view «%v»: %w", name, v.QName(), ErrNameUniqueViolation))
		}
	}

	check(v.PartKey())
	check(v.ClustCols())
	check(v.Value())
}

// # Implements:
//   - IPartKey
type viewPKey struct {
	def
	fields
}

func newViewPKey(app *appDef, name QName) *viewPKey {
	pKey := &viewPKey{def: makeDef(app, name, DefKind_ViewRecord_PartitionKey)}
	pKey.fields = makeFields(pKey)
	app.appendDef(pKey)
	return pKey
}

// Validates view partition key
func (pk *viewPKey) Validate() error {
	if pk.FieldCount() == 0 {
		return fmt.Errorf("%v: view partition key can not to be empty: %w", pk.QName(), ErrFieldsMissed)
	}
	return nil
}

// # Implements:
//   - IClustCols
type viewCCols struct {
	def
	fields
}

func newViewCCols(app *appDef, name QName) *viewCCols {
	cc := &viewCCols{def: makeDef(app, name, DefKind_ViewRecord_ClusteringColumns)}
	cc.fields = makeFields(cc)
	app.appendDef(cc)
	return cc
}

// Validates view clustering columns
func (cc *viewCCols) Validate() (err error) {
	if cc.FieldCount() == 0 {
		return fmt.Errorf("%v: view clustering columns can not to be empty: %w", cc.QName(), ErrFieldsMissed)
	}

	idx, cnt := 0, cc.FieldCount()
	cc.Fields(func(fld IField) {
		idx++
		if idx == cnt {
			return // last field may be any kind
		}
		if !fld.IsFixedWidth() {
			err = errors.Join(err,
				fmt.Errorf("%v: only last view clustering column field can be variable length; not last field «%s» has variable length type «%v»: %w", cc.QName(), fld.Name(), fld.DataKind(), ErrInvalidDataKind))
		}
	})

	return err
}

// # Implements:
//   - IViewKey
type viewKey struct {
	def
	fields
	containers
}

func newViewKey(app *appDef, name QName) *viewKey {
	key := &viewKey{def: makeDef(app, name, DefKind_ViewRecord_Key)}
	key.fields = makeFields(key)
	key.containers = makeContainers(key)
	app.appendDef(key)
	return key
}

// # Implements:
//   - IViewValue
type viewValue struct {
	def
	fields
}

func newViewValue(app *appDef, name QName) *viewValue {
	val := &viewValue{def: makeDef(app, name, DefKind_ViewRecord_Value)}
	val.fields = makeFields(val)
	app.appendDef(val)
	return val
}

// Returns partition key definition name for specified view
func ViewPartitionKeyDefName(viewName QName) QName {
	const suffix = "_PartitionKey"
	return suffixedQName(viewName, suffix)
}

// Returns clustering columns definition name for specified view
func ViewClusteringColumnsDefName(viewName QName) QName {
	const suffix = "_ClusteringColumns"
	return suffixedQName(viewName, suffix)
}

// Returns full key definition name for specified view
func ViewKeyDefName(viewName QName) QName {
	const suffix = "_FullKey"
	return suffixedQName(viewName, suffix)
}

// Returns value definition name for specified view
func ViewValueDefName(viewName QName) QName {
	const suffix = "_Value"
	return suffixedQName(viewName, suffix)
}

// Appends suffix to QName entity name and returns new QName
func suffixedQName(name QName, suffix string) QName {
	return NewQName(name.Pkg(), name.Entity()+suffix)
}

/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package views

import (
	"errors"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appdef/internal/fields"
	"github.com/voedger/voedger/pkg/appdef/internal/types"
)

// # Supports:
//   - appdef.IView
type View struct {
	types.Typ
	fields.FieldsList // all fields, include key and value
	key               *ViewKey
	value             *ViewValue
}

func NewView(ws appdef.IWorkspace, name appdef.QName) *View {
	v := &View{
		Typ:        types.MakeType(ws.App(), ws, name, appdef.TypeKind_ViewRecord),
		FieldsList: fields.MakeFields(ws, appdef.TypeKind_ViewRecord),
	}
	v.FieldsList.MakeSysFields()
	v.key = NewViewKey(v)
	v.value = NewViewValue(v)
	return v
}

func (v View) Key() appdef.IViewKey { return v.key }

func (v View) Value() appdef.IViewValue { return v.value }

// Validates view
func (v *View) Validate() error {
	return errors.Join(
		v.key.Validate(),
		v.value.Validate(),
	)
}

// # Supports:
//   - appdef.IViewBuilder
type ViewBuilder struct {
	*View
	types.TypeBuilder
	key *ViewKeyBuilder
	val *ViewValueBuilder
}

func NewViewBuilder(v *View) *ViewBuilder {
	return &ViewBuilder{
		View:        v,
		TypeBuilder: types.MakeTypeBuilder(&v.Typ),
		key:         NewViewKeyBuilder(v.key),
		val:         NewViewValueBuilder(v.value),
	}
}

func (vb *ViewBuilder) Key() appdef.IViewKeyBuilder { return vb.key }

func (vb *ViewBuilder) Value() appdef.IViewValueBuilder { return vb.val }

// # Supports:
//   - IViewKey
type ViewKey struct {
	view *View
	fields.FieldsList
	pkey  *ViewPartKey
	ccols *ViewClustCols
}

func NewViewKey(view *View) *ViewKey {
	return &ViewKey{
		view:       view,
		FieldsList: fields.MakeFields(view.Workspace(), appdef.TypeKind_ViewRecord),
		pkey:       NewViewPartKey(view),
		ccols:      NewViewClustCols(view),
	}
}

func (key ViewKey) PartKey() appdef.IViewPartKey { return key.pkey }

func (key ViewKey) ClustCols() appdef.IViewClustCols { return key.ccols }

// Validates value key
func (key *ViewKey) Validate() error {
	return errors.Join(
		key.pkey.Validate(),
		key.ccols.Validate(),
	)
}

// # Supports:
//   - appdef.IViewKeyBuilder
type ViewKeyBuilder struct {
	*ViewKey
	pkey  *ViewPartKeyBuilder
	ccols *ViewClustColsBuilder
}

func NewViewKeyBuilder(key *ViewKey) *ViewKeyBuilder {
	return &ViewKeyBuilder{
		ViewKey: key,
		pkey:    NewViewPartKeyBuilder(key.pkey),
		ccols:   NewViewClustColsBuilder(key.ccols),
	}
}

func (kb *ViewKeyBuilder) ClustCols() appdef.IViewClustColsBuilder { return kb.ccols }

func (kb *ViewKeyBuilder) PartKey() appdef.IViewPartKeyBuilder { return kb.pkey }

// # Supports:
//   - appdef.IViewPartKey
type ViewPartKey struct {
	view *View
	fields.FieldsList
}

func NewViewPartKey(v *View) *ViewPartKey {
	pKey := &ViewPartKey{
		view:       v,
		FieldsList: fields.MakeFields(v.Workspace(), appdef.TypeKind_ViewRecord),
	}
	return pKey
}

func (pk *ViewPartKey) addDataField(name appdef.FieldName, dataType appdef.QName, constraints ...appdef.IConstraint) {
	d := appdef.Data(pk.view.App().Type, dataType)
	if d == nil {
		panic(appdef.ErrNotFound("%v partition key field «%s» data type «%v»", pk.view.QName(), name, dataType))
	}
	if k := d.DataKind(); !k.IsFixed() {
		panic(appdef.ErrUnsupported("various length %s-field «%s» with partition key of %v", k.TrimString(), name, pk.view))
	}
	fields.AddDataField(&pk.view.FieldsList, name, dataType, true, constraints...)
	fields.AddDataField(&pk.view.key.FieldsList, name, dataType, true, constraints...)
	fields.AddDataField(&pk.FieldsList, name, dataType, true, constraints...)
}

func (pk *ViewPartKey) addField(name appdef.FieldName, kind appdef.DataKind, constraints ...appdef.IConstraint) {
	if !kind.IsFixed() {
		panic(appdef.ErrUnsupported("various length %s-field «%s» with partition key of %v", kind.TrimString(), name, pk.view))
	}
	fields.AddField(&pk.view.FieldsList, name, kind, true, constraints...)
	fields.AddField(&pk.view.key.FieldsList, name, kind, true, constraints...)
	fields.AddField(&pk.FieldsList, name, kind, true, constraints...)
}

func (pk *ViewPartKey) addRefField(name appdef.FieldName, ref ...appdef.QName) {
	fields.AddRefField(&pk.view.FieldsList, name, true, ref...)
	fields.AddRefField(&pk.view.key.FieldsList, name, false, ref...)
	fields.AddRefField(&pk.FieldsList, name, false, ref...)
}

func (pk *ViewPartKey) setFieldComment(name appdef.FieldName, comment ...string) {
	fields.SetFieldComment(&pk.view.FieldsList, name, comment...)
	fields.SetFieldComment(&pk.view.key.FieldsList, name, comment...)
	fields.SetFieldComment(&pk.FieldsList, name, comment...)
}

// Validates view partition key
func (pk *ViewPartKey) Validate() error {
	if pk.FieldsList.FieldCount() == 0 {
		return appdef.ErrMissed("%v partition key fields", pk.view)
	}
	return nil
}

// # Supports:
//   - appdef.IViewPartKeyBuilder
type ViewPartKeyBuilder struct {
	*ViewPartKey
}

func NewViewPartKeyBuilder(pk *ViewPartKey) *ViewPartKeyBuilder {
	return &ViewPartKeyBuilder{ViewPartKey: pk}
}

func (pkb *ViewPartKeyBuilder) AddDataField(name appdef.FieldName, dataType appdef.QName, constraints ...appdef.IConstraint) appdef.IViewPartKeyBuilder {
	pkb.ViewPartKey.addDataField(name, dataType, constraints...)
	return pkb
}

func (pkb *ViewPartKeyBuilder) AddField(name appdef.FieldName, kind appdef.DataKind, constraints ...appdef.IConstraint) appdef.IViewPartKeyBuilder {
	pkb.ViewPartKey.addField(name, kind, constraints...)
	return pkb
}

func (pkb *ViewPartKeyBuilder) AddRefField(name appdef.FieldName, ref ...appdef.QName) appdef.IViewPartKeyBuilder {
	pkb.ViewPartKey.addRefField(name, ref...)
	return pkb
}

func (pkb *ViewPartKeyBuilder) SetFieldComment(name appdef.FieldName, comment ...string) appdef.IViewPartKeyBuilder {
	pkb.ViewPartKey.setFieldComment(name, comment...)
	return pkb
}

// # Supports:
//   - appdef.IViewClustCols
type ViewClustCols struct {
	view *View
	fields.FieldsList
	varField appdef.FieldName
}

func NewViewClustCols(v *View) *ViewClustCols {
	return &ViewClustCols{
		view:       v,
		FieldsList: fields.MakeFields(v.Workspace(), appdef.TypeKind_ViewRecord),
	}
}

func (cc *ViewClustCols) addDataField(name appdef.FieldName, dataType appdef.QName, constraints ...appdef.IConstraint) {
	d := appdef.Data(cc.view.App().Type, dataType)
	if d == nil {
		panic(appdef.ErrNotFound("%v clustering columns field «%s» data type «%v»", cc.view.QName(), name, dataType))
	}
	cc.panicIfVarFieldDuplication(name, d.DataKind())
	fields.AddDataField(&cc.view.FieldsList, name, dataType, false, constraints...)
	fields.AddDataField(&cc.view.key.FieldsList, name, dataType, false, constraints...)
	fields.AddDataField(&cc.FieldsList, name, dataType, false, constraints...)
}

func (cc *ViewClustCols) addField(name appdef.FieldName, kind appdef.DataKind, constraints ...appdef.IConstraint) {
	cc.panicIfVarFieldDuplication(name, kind)
	fields.AddField(&cc.view.FieldsList, name, kind, false, constraints...)
	fields.AddField(&cc.view.key.FieldsList, name, kind, false, constraints...)
	fields.AddField(&cc.FieldsList, name, kind, false, constraints...)
}

func (cc *ViewClustCols) addRefField(name appdef.FieldName, ref ...appdef.QName) {
	cc.panicIfVarFieldDuplication(name, appdef.DataKind_RecordID)
	fields.AddRefField(&cc.view.FieldsList, name, false, ref...)
	fields.AddRefField(&cc.view.key.FieldsList, name, false, ref...)
	fields.AddRefField(&cc.FieldsList, name, false, ref...)
}

// Panics if variable length field already exists
func (cc *ViewClustCols) panicIfVarFieldDuplication(name appdef.FieldName, kind appdef.DataKind) {
	if len(cc.varField) > 0 {
		panic(appdef.ErrUnsupported("%v clustering column already has a various length field «%s», it should be last field and no more fields can be added", cc.view, cc.varField))
	}
	if !kind.IsFixed() {
		cc.varField = name
	}
}

func (cc *ViewClustCols) setFieldComment(name appdef.FieldName, comment ...string) {
	fields.SetFieldComment(&cc.view.FieldsList, name, comment...)
	fields.SetFieldComment(&cc.view.key.FieldsList, name, comment...)
	fields.SetFieldComment(&cc.FieldsList, name, comment...)
}

// Validates view clustering columns
func (cc *ViewClustCols) Validate() error {
	if cc.FieldCount() == 0 {
		return appdef.ErrMissed("%v clustering columns fields", cc.view)
	}
	return nil
}

// # Supports:
//   - appdef.IViewClustColsBuilder
type ViewClustColsBuilder struct {
	*ViewClustCols
}

func NewViewClustColsBuilder(cc *ViewClustCols) *ViewClustColsBuilder {
	return &ViewClustColsBuilder{ViewClustCols: cc}
}

func (ccb *ViewClustColsBuilder) AddDataField(name appdef.FieldName, dataType appdef.QName, constraints ...appdef.IConstraint) appdef.IViewClustColsBuilder {
	ccb.ViewClustCols.addDataField(name, dataType, constraints...)
	return ccb
}

func (ccb *ViewClustColsBuilder) AddField(name appdef.FieldName, kind appdef.DataKind, constraints ...appdef.IConstraint) appdef.IViewClustColsBuilder {
	ccb.ViewClustCols.addField(name, kind, constraints...)
	return ccb
}

func (ccb *ViewClustColsBuilder) AddRefField(name appdef.FieldName, ref ...appdef.QName) appdef.IViewClustColsBuilder {
	ccb.ViewClustCols.addRefField(name, ref...)
	return ccb
}

func (ccb *ViewClustColsBuilder) SetFieldComment(name appdef.FieldName, comment ...string) appdef.IViewClustColsBuilder {
	ccb.ViewClustCols.setFieldComment(name, comment...)
	return ccb
}

// # Supports:
//   - appdef.IViewValue
type ViewValue struct {
	view *View
	fields.FieldsList
}

func NewViewValue(v *View) *ViewValue {
	val := &ViewValue{
		view:       v,
		FieldsList: fields.MakeFields(v.Workspace(), appdef.TypeKind_ViewRecord),
	}
	val.FieldsList.MakeSysFields()
	return val
}

func (v *ViewValue) addDataField(name appdef.FieldName, dataType appdef.QName, required bool, constraints ...appdef.IConstraint) {
	fields.AddDataField(&v.view.FieldsList, name, dataType, required, constraints...)
	fields.AddDataField(&v.FieldsList, name, dataType, required, constraints...)
}

func (v *ViewValue) addField(name appdef.FieldName, kind appdef.DataKind, required bool, constraints ...appdef.IConstraint) {
	fields.AddField(&v.view.FieldsList, name, kind, required, constraints...)
	fields.AddField(&v.FieldsList, name, kind, required, constraints...)
}

func (v *ViewValue) addRefField(name appdef.FieldName, required bool, ref ...appdef.QName) {
	fields.AddRefField(&v.view.FieldsList, name, required, ref...)
	fields.AddRefField(&v.FieldsList, name, required, ref...)
}

func (v *ViewValue) setFieldComment(name appdef.FieldName, comment ...string) {
	fields.SetFieldComment(&v.view.FieldsList, name, comment...)
	fields.SetFieldComment(&v.FieldsList, name, comment...)
}

func (v *ViewValue) setFieldVerify(name appdef.FieldName, vk ...appdef.VerificationKind) {
	fields.SetFieldVerify(&v.view.FieldsList, name, vk...)
	fields.SetFieldVerify(&v.FieldsList, name, vk...)
}

// Validates view value
func (v *ViewValue) Validate() error { return nil }

// # Supports:
//   - appdef.IViewValueBuilder
type ViewValueBuilder struct {
	*ViewValue
}

func NewViewValueBuilder(viewValue *ViewValue) *ViewValueBuilder {
	return &ViewValueBuilder{ViewValue: viewValue}
}

func (vb *ViewValueBuilder) AddDataField(name appdef.FieldName, dataType appdef.QName, required bool, constraints ...appdef.IConstraint) appdef.IFieldsBuilder {
	vb.ViewValue.addDataField(name, dataType, required, constraints...)
	return vb
}

func (vb *ViewValueBuilder) AddField(name appdef.FieldName, kind appdef.DataKind, required bool, constraints ...appdef.IConstraint) appdef.IFieldsBuilder {
	vb.ViewValue.addField(name, kind, required, constraints...)
	return vb
}

func (vb *ViewValueBuilder) AddRefField(name appdef.FieldName, required bool, ref ...appdef.QName) appdef.IFieldsBuilder {
	vb.ViewValue.addRefField(name, required, ref...)
	return vb
}

func (vb *ViewValueBuilder) SetFieldComment(name appdef.FieldName, comment ...string) appdef.IFieldsBuilder {
	vb.ViewValue.setFieldComment(name, comment...)
	return vb
}

func (vb *ViewValueBuilder) SetFieldVerify(name appdef.FieldName, vk ...appdef.VerificationKind) appdef.IFieldsBuilder {
	vb.ViewValue.setFieldVerify(name, vk...)
	return vb
}

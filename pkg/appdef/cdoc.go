/*
 * Copyright (c) 2023-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

// # Implements:
//   - ICDoc, ICDocBuilder
type cDoc struct {
	doc
	singleton bool
}

// Creates a new CDoc
func newCDoc(app *appDef, name QName) *cDoc {
	d := &cDoc{}
	d.doc = makeDoc(app, name, TypeKind_CDoc, d)
	d.makeSysFields()
	app.appendType(d)
	return d
}

func (d *cDoc) SetSingleton() {
	d.singleton = true
}

func (d *cDoc) Singleton() bool {
	return d.singleton
}

// # Implements:
//   - ICRecord, ICRecordBuilder
type cRecord struct {
	record
}

func newCRecord(app *appDef, name QName) *cRecord {
	r := &cRecord{}
	r.record = makeRecord(app, name, TypeKind_CRecord, r)
	r.makeSysFields()
	app.appendType(r)
	return r
}

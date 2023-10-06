/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

// Global document
type IGDoc interface {
	IDoc

	// unwanted type assertion stub
	isGDoc()
}

type IGDocBuilder interface {
	IGDoc
	IDocBuilder
}

// Global document record
//
// Ref. to gdoc.go for implementation
type IGRecord interface {
	IRecord

	// unwanted type assertion stub
	isGRecord()
}

type IGRecordBuilder interface {
	IGRecord
	IRecordBuilder
}

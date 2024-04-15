/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

// Workflow document.
type IWDoc interface {
	ISingleton

	// Unwanted type assertion stub
	isWDoc()
}

type IWDocBuilder interface {
	ISingletonBuilder
}

// Workflow document record.
type IWRecord interface {
	IContainedRecord

	// Unwanted type assertion stub
	isWRecord()
}

type IWRecordBuilder interface {
	IContainedRecordBuilder
}

type IWithWDocs interface {
	// Return WDoc by name.
	//
	// Returns nil if not found.
	WDoc(QName) IWDoc

	// Enumerates all application workflow documents
	//
	// Workflow documents are enumerated in alphabetical order by QName
	WDocs(func(IWDoc))

	// Return WRecord by name.
	//
	// Returns nil if not found.
	WRecord(QName) IWRecord

	// Enumerates all application workflow records
	//
	// Workflow records are enumerated in alphabetical order by QName
	WRecords(func(IWRecord))
}

type IWDocsBuilder interface {
	// Adds new WDoc type with specified name.
	//
	// # Panics:
	//   - if name is empty (appdef.NullQName),
	//   - if name is invalid,
	//   - if type with name already exists.
	AddWDoc(QName) IWDocBuilder

	// Adds new WRecord type with specified name.
	//
	// # Panics:
	//   - if name is empty (appdef.NullQName),
	//   - if name is invalid,
	//   - if type with name already exists.
	AddWRecord(QName) IWRecordBuilder
}

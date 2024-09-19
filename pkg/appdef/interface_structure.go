/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

// Structure is a type with fields, containers and uniques.
type IStructure interface {
	IType
	IFields
	IContainers
	IUniques
	IWithAbstract

	// Returns definition for «sys.QName» field
	SystemField_QName() IField
}

type IStructureBuilder interface {
	ITypeBuilder
	IFieldsBuilder
	IContainersBuilder
	IUniquesBuilder
	IWithAbstractBuilder
}

type IWithStructures interface {
	// Return structure by name.
	//
	// Returns nil if not found.
	Structure(QName) IStructure

	// Enumerates all application structures
	//
	// Structures are enumerated in alphabetical order by QName
	Structures(func(IStructure) bool)
}

// Record is a structure.
//
// Record has ID field.
type IRecord interface {
	IStructure

	// Returns definition for «sys.ID» field
	SystemField_ID() IField
}

type IRecordBuilder interface {
	IStructureBuilder
}

type IWithRecords interface {
	// Return record by name.
	//
	// Returns nil if not found.
	Record(QName) IRecord

	// Enumerates all application records, e.g. documents and contained records
	//
	// Records are enumerated in alphabetical order by QName
	Records(func(IRecord))
}

// Document is a record.
//
// Document can contains records.
type IDoc interface {
	IRecord

	// Unwanted type assertion stub
	isDoc()
}

type IDocBuilder interface {
	IRecordBuilder
}

// Contained record is a record that has parent.
type IContainedRecord interface {
	IRecord

	// Returns definition for «sys.ParentID» field
	SystemField_ParentID() IField

	// Returns definition for «sys.Container» field
	SystemField_Container() IField
}

type IContainedRecordBuilder interface {
	IRecordBuilder
}

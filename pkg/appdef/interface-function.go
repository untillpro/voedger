/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

// Extension engine kind enumeration.
//
// Ref. to extension-engine-kind.go for constants and methods
type ExtensionEngineKind uint8

// Entry point for extension
//
// Ref. to extension.go for implementation
type IExtension interface {
	IComment

	// Extension entry point name
	Name() string

	// Engine kind
	Engine() ExtensionEngineKind
}

// Function is a type of object that can be called.
// Function may have parameter and result.
// Function have extension.
// Function may be query or command.
//
// Ref. to function.go for implementation
type IFunction interface {
	IType

	// Parameter. Returns nil if not assigned
	Param() IType

	// Result. Returns nil if not assigned
	Result() IType

	// Extension
	Extension() IExtension
}

type IFunctionBuilder interface {
	IFunction
	ITypeBuilder

	// Sets function parameter. Must be known type from next kinds:
	//	 - Data
	//	 - ODoc
	//	 - Object
	//
	// If NullQName passed then it means that function has no parameter.
	// If QNameANY passed then it means that parameter may be any.
	SetParam(QName) IFunctionBuilder

	// Sets function result. Must be known type from next kinds:
	//	 - Data
	//	 - GDoc,  CDoc, WDoc, ODoc
	//	 - Object
	//
	// If NullQName passed then it means that function has no result.
	// If QNameANY passed then it means that result may be any.
	SetResult(QName) IFunctionBuilder

	// Sets engine.
	//
	// # Panics:
	//	- if name is empty or invalid identifier
	SetExtension(name string, engine ExtensionEngineKind, comment ...string) IFunctionBuilder
}

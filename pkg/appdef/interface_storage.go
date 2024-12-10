/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

type IStorage interface {
	IWithComments

	// Returns storage name.
	Name() QName

	// Returns names in storage.
	//
	// TODO: should be iter.Seq[QName]
	Names() QNames
}

type IStorages interface {
	// Returns storage by name.
	//
	// Returns nil if storage not found.
	Storage(name QName) IStorage

	// Enums storages.
	//
	// Storages enumerated in alphabetical QNames order.
	// Names slice in every storage is sorted and deduplicated.
	//
	// TODO: should be iter.Seq[IStorage], should be renamed to Values()
	Enum(func(IStorage) bool)

	// Returns number of storages.
	Len() int

	// Returns storages as map.
	//
	// TODO: should be iter.Seq2[QName, QNames], should be renamed to All()
	Map() map[QName]QNames
}

type IStoragesBuilder interface {
	// Add storage.
	//
	// If storage with name is already exists in states then names will be added to existing storage.
	//
	// # Panics:
	//	- if name is empty,
	//	- if name is invalid,
	//	- if names contains invalid name(s).
	Add(name QName, names ...QName) IStoragesBuilder

	// Sets comment for storage.
	//
	// # Panics:
	//	- if storage with name is not added.
	SetComment(name QName, comment string) IStoragesBuilder
}

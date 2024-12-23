/*
 * Copyright (c) 2024-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package filter

import "github.com/voedger/voedger/pkg/appdef"

// QNames is a filter that matches types by their qualified names.
func QNames(names ...appdef.QName) appdef.IFilter {
	return newQNamesFilter(names...)
}

// Tags is a filter that matches types by their tags.
func Tags(tag appdef.QName, tags ...appdef.QName) appdef.IFilter {
	return newTagsFilter(tag, tags...)
}

// Types is a filter that matches types by their kind.
//
// #Panics:
//	 - if no type kinds are specified
func Types(tt ...appdef.TypeKind) appdef.IFilter {
	return newTypesFilter(tt...)
}

// WSTypes is a filter that matches types by their kind.
// Matched types should be located in the specified workspace.
//
// #Panics:
//	 - if workspace is not specified (NullQName)
//	 - if no type kinds are specified
func WSTypes(ws appdef.QName, tt ...appdef.TypeKind) appdef.IFilter {
	return newWSTypesFilter(ws, tt...)
}

// And returns a filter that matches types that match all children filters.
func And(f1, f2 appdef.IFilter, ff ...appdef.IFilter) appdef.IFilter {
	return newAndFilter(f1, f2, ff...)
}

// Or returns a filter that matches types that match any children filter.
func Or(f1, f2 appdef.IFilter, ff ...appdef.IFilter) appdef.IFilter {
	return newOrFilter(f1, f2, ff...)
}

// Not returns a filter that invert matches for specified filter.
func Not(f appdef.IFilter) appdef.IFilter {
	return newNotFilter(f)
}

// True returns filter that always matches any type
func True() appdef.IFilter { return trueFlt }

// AllTables returns a filter that matches all structured types, see appdef.TypeKind_Structures
func AllTables() appdef.IFilter {
	return newTypesFilter(appdef.TypeKind_Structures.AsArray()...)
}

// AllWSTables returns a filter that matches all structured types from workspace, see appdef.TypeKind_Structures
func AllWSTables(ws appdef.QName) appdef.IFilter {
	return newWSTypesFilter(ws, appdef.TypeKind_Structures.AsArray()...)
}

// AllFunctions returns a filter that matches all functions types, see appdef.TypeKind_Functions
func AllFunctions() appdef.IFilter {
	return newTypesFilter(appdef.TypeKind_Functions.AsArray()...)
}

// AllWSFunctions returns a filter that matches all functions types from workspace, see appdef.TypeKind_Functions
func AllWSFunctions(ws appdef.QName) appdef.IFilter {
	return newWSTypesFilter(ws, appdef.TypeKind_Functions.AsArray()...)
}

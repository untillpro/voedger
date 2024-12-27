/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

import (
	"errors"
	"fmt"
	"iter"
	"slices"

	slicesex "github.com/voedger/voedger/pkg/appdef/internal/slices"
)

// # Supports:
//   - IType
type typ struct {
	comment
	tags
	app  *appDef
	ws   *workspace
	name QName
	kind TypeKind
}

// Creates and returns new type.
//
// Name can be empty (NullQName), then type is anonymous.
func makeType(app *appDef, ws *workspace, name QName, kind TypeKind) typ {
	if name != NullQName {
		if ok, err := ValidQName(name); !ok {
			panic(fmt.Errorf("invalid type name «%v»: %w", name, err))
		}
	}
	find := app.Type
	if ws != nil {
		find = ws.LocalType // #2889 $VSQL_TagNonExp: only local tags can be used
	}
	t := typ{
		comment: makeComment(),
		tags:    makeTags(find),
		app:     app,
		ws:      ws,
		name:    name,
		kind:    kind,
	}
	return t
}

func (t typ) App() IAppDef {
	return t.app
}

func (t typ) IsSystem() bool {
	return t.QName().Pkg() == SysPackage
}

func (t typ) Kind() TypeKind {
	return t.kind
}

func (t typ) QName() QName {
	return t.name
}

func (t typ) String() string {
	return fmt.Sprintf("%s «%v»", t.Kind().TrimString(), t.QName())
}

func (t typ) Workspace() IWorkspace {
	return t.ws
}

// # Implements:
//   - ITypeBuilder
type typeBuilder struct {
	commentBuilder
	tagBuilder
	*typ
}

func makeTypeBuilder(typ *typ) typeBuilder {
	return typeBuilder{
		commentBuilder: makeCommentBuilder(&typ.comment),
		tagBuilder:     makeTagBuilder(&typ.tags),
		typ:            typ,
	}
}

func (t *typeBuilder) String() string { return t.typ.String() }

// List of types.
type types[T IType] struct {
	m map[QName]T
	s []T
}

// Creates and returns new types.
func newTypes[T IType]() *types[T] {
	return &types[T]{m: make(map[QName]T)}
}

func (tt *types[T]) add(t T) {
	tt.m[t.QName()] = t
	tt.s = slicesex.InsertInSort(tt.s, t, func(t1, t2 T) int { return CompareQName(t1.QName(), t2.QName()) })
}

func (tt *types[T]) clear() {
	tt.m = make(map[QName]T)
	tt.s = nil
}

func (tt types[T]) find(name QName) IType {
	if t, ok := tt.m[name]; ok {
		return t
	}
	return NullType
}

func (tt types[T]) values() iter.Seq[T] {
	return slices.Values(tt.s)
}

type typeRef struct {
	name QName
	typ  IType
}

// Returns type by reference.
//
// If type is not found then returns nil.
func (r typeRef) target(tt FindType) IType {
	switch r.name {
	case NullQName:
		return nil
	case QNameANY:
		return AnyType
	default:
		if (r.typ != nil) && (r.typ.QName() == r.name) {
			return r.typ
		}
	}
	if t := tt(r.name); t.Kind() != TypeKind_null {
		return t
	}
	return nil
}

// Sets reference name
func (r *typeRef) setName(n QName) {
	r.name = n
	r.typ = nil
}

// Returns is reference valid
func (r *typeRef) valid(tt FindType) (bool, error) {
	if (r.name == NullQName) || (r.name == QNameANY) {
		return true, nil
	}
	if t := r.target(tt); t != nil {
		if r.typ != t {
			r.typ = t
		}
		return true, nil
	}

	return false, ErrTypeNotFound(r.name)
}

// Validate specified type.
//
// # Validation:
//   - if type supports Validate() interface, then call this,
//   - if structured type has fields, validate fields,
//   - if structured type has containers, validate containers
func validateType(t IType) (err error) {
	if v, ok := t.(interface{ Validate() error }); ok {
		err = v.Validate()
	}

	if _, ok := t.(IFields); ok {
		err = errors.Join(err, validateTypeFields(t))
	}

	if _, ok := t.(IContainers); ok {
		err = errors.Join(err, validateTypeContainers(t))
	}

	return err
}

const nullTypeString = "null type"

type nullType struct {
	nullComment
	nullTags
}

func (t nullType) App() IAppDef          { return nil }
func (t nullType) IsSystem() bool        { return false }
func (t nullType) Kind() TypeKind        { return TypeKind_null }
func (t nullType) QName() QName          { return NullQName }
func (t nullType) String() string        { return nullTypeString }
func (t nullType) Workspace() IWorkspace { return nil }

type anyType struct {
	nullComment
	nullTags
	name QName
}

func newAnyType(name QName) IType { return &anyType{nullComment{}, nullTags{}, name} }

func (t anyType) App() IAppDef          { return nil }
func (t anyType) IsSystem() bool        { return true }
func (t anyType) Kind() TypeKind        { return TypeKind_Any }
func (t anyType) QName() QName          { return t.name }
func (t anyType) String() string        { return t.name.Entity() + " type" }
func (t anyType) Workspace() IWorkspace { return nil }

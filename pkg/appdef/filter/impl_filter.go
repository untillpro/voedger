/*
 * Copyright (c) 2024-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package filter

import (
	"fmt"
	"sort"

	"github.com/voedger/voedger/pkg/appdef"
)

// abstract filter.
type filter struct{}

func (filter) And() []appdef.IFilter { return nil }

func (filter) Not() appdef.IFilter { return nil }

func (filter) Or() []appdef.IFilter { return nil }

func (filter) QNames() appdef.QNames { return nil }

func (filter) Tags() []string { return nil }

func (filter) Types() appdef.TypeKindSet { return appdef.TypeKindSet{} }

// Filter results.
//
// # Supports:
//   - appdef.IWithTypes
//   - fmt.Stringer
type results struct {
	m map[appdef.QName]appdef.IType
	s []appdef.IType
}

func makeResults(t ...appdef.IType) *results {
	r := &results{m: make(map[appdef.QName]appdef.IType)}
	for _, t := range t {
		r.add(t)
	}
	return r
}

func copyResults(types appdef.IWithTypes) *results {
	r := makeResults()
	for t := range types.Types {
		r.add(t)
	}
	return r
}

func (r results) String() string {
	s := ""
	for t := range r.Types {
		if s != "" {
			s += ", "
		}
		s += fmt.Sprint(t)
	}
	return fmt.Sprintf("[%s]", s)
}

func (r results) Type(name appdef.QName) appdef.IType {
	if t, ok := r.m[name]; ok {
		return t
	}
	return appdef.NullType
}

func (r *results) Types(visit func(appdef.IType) bool) {
	if len(r.s) != len(r.m) {
		r.s = make([]appdef.IType, 0, len(r.m))
		for _, t := range r.m {
			r.s = append(r.s, t)
		}
		sort.Slice(r.s, func(i, j int) bool {
			return r.s[i].QName().String() < r.s[j].QName().String()
		})
	}
	for _, t := range r.s {
		if !visit(t) {
			break
		}
	}
}

func (r *results) add(t appdef.IType) {
	r.m[t.QName()] = t
}

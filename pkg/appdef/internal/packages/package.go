/*
 * Copyright (c) 2024-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package packages

import (
	"iter"
	"slices"

	"github.com/voedger/voedger/pkg/appdef"
)

type Packages struct {
	local       []string
	localByPath map[string]string
	pathByLocal map[string]string
}

func NewPackages() *Packages {
	return &Packages{
		local:       make([]string, 0),
		localByPath: make(map[string]string),
		pathByLocal: make(map[string]string),
	}
}

func (p *Packages) Add(local, path string) {
	if ok, err := appdef.ValidIdent(local); !ok {
		panic(err)
	}
	if p, ok := p.pathByLocal[local]; ok {
		panic(appdef.ErrAlreadyExists("package local name «%s» already used for «%s»", local, p))
	}

	if path == "" {
		panic(appdef.ErrMissed("package «%s» path", local))
	}
	if l, ok := p.localByPath[path]; ok {
		panic(appdef.ErrAlreadyExists("package path «%s» already used for «%s»", path, l))
	}

	p.local = append(p.local, local)
	slices.Sort(p.local)

	p.localByPath[path] = local
	p.pathByLocal[local] = path
}

func (p Packages) FullQName(n appdef.QName) appdef.FullQName {
	if path, ok := p.pathByLocal[n.Pkg()]; ok {
		return appdef.NewFullQName(path, n.Entity())
	}
	return appdef.NullFullQName
}

func (p Packages) LocalNameByPath(path string) string { return p.localByPath[path] }

func (p Packages) LocalQName(n appdef.FullQName) appdef.QName {
	if pkg, ok := p.localByPath[n.PkgPath()]; ok {
		return appdef.NewQName(pkg, n.Entity())
	}
	return appdef.NullQName
}

func (p Packages) PathByLocalName(local string) string { return p.pathByLocal[local] }

func (p Packages) PackageLocalNames() iter.Seq[string] { return slices.Values(p.local) }

func (p Packages) Packages() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		for _, local := range p.local {
			if !yield(local, p.pathByLocal[local]) {
				break
			}
		}
	}
}

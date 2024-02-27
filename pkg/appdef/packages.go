/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

import (
	"fmt"
	"slices"
)

type packages struct {
	local       []string
	localByPath map[string]string
	pathByLocal map[string]string
}

func newPackages() *packages {
	return &packages{
		local:       make([]string, 0),
		localByPath: make(map[string]string),
		pathByLocal: make(map[string]string),
	}
}

func (p *packages) add(local, path string) {
	if ok, err := ValidIdent(local); !ok {
		panic(err)
	}
	if p, ok := p.pathByLocal[local]; ok {
		panic(fmt.Errorf("package local name «%s» already used for «%s»: %w", local, p, ErrNameUniqueViolation))
	}

	if path == "" {
		panic(fmt.Errorf("package «%s» has empty path: %w", local, ErrNameMissed))
	}
	if l, ok := p.localByPath[path]; ok {
		panic(fmt.Errorf("package path «%s» already used for «%s»: %w", path, l, ErrNameUniqueViolation))
	}

	p.local = append(p.local, local)
	slices.Sort(p.local)

	p.localByPath[path] = local
	p.pathByLocal[local] = path
}

func (p *packages) forEach(cb func(local, path string)) {
	for _, local := range p.local {
		cb(local, p.pathByLocal[local])
	}
}

func (p *packages) localNameByPath(path string) string {
	return p.localByPath[path]
}

func (p *packages) pathByLocalName(local string) string {
	return p.pathByLocal[local]
}

func (p *packages) localNames() []string {
	return p.local
}

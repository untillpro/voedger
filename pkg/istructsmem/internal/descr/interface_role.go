/*
 * Copyright (c) 2024-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package descr

import "github.com/voedger/voedger/pkg/appdef"

type Role struct {
	Type
	ACL []*ACLRule
}

type ACLRule struct {
	Comment string `json:",omitempty"`
	Policy  string // `Allow` or `Deny`
	Ops     []string
	On      appdef.QNames
	Fields  []string `json:",omitempty"`
}

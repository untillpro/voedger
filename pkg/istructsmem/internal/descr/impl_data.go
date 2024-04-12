/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package descr

import "github.com/voedger/voedger/pkg/appdef"

func newData() *Data {
	return &Data{
		Constraints: make(map[string]any, 0),
	}
}

func (d *Data) read(data appdef.IData) {
	d.Type.read(data)
	if data.Ancestor() != nil {
		q := data.Ancestor().QName()
		d.Ancestor = &q
	} else {
		// notest: only system data types have no ancestor
		k := data.DataKind()
		d.DataKind = &k
	}
	for k, c := range data.Constraints(false) {
		d.Constraints[k.TrimString()] = c.Value()
	}
}

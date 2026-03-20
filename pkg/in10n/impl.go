/*
 * Copyright (c) 2025-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package in10n

import (
	"bytes"

	"github.com/voedger/voedger/pkg/goutils/strconvu"
)

func (pk ProjectionKey) ToJSON() string {
	buf := bytes.NewBufferString(`{"App":"`)
	buf.WriteString(pk.App.String())
	buf.WriteString(`","Projection":"`)
	buf.WriteString(pk.Projection.String())
	buf.WriteString(`","WS":`)
	buf.WriteString(strconvu.UintToString(pk.WS))
	buf.WriteString("}")
	return buf.String()
}

func ProjectionKeysToJSON(keys []ProjectionKey) string {
	buf := bytes.NewBufferString("[")
	for i, k := range keys {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(k.ToJSON())
	}
	buf.WriteString("]")
	return buf.String()
}

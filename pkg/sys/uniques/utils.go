/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 */

package uniques

import (
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

// returns ID of the record by the unique combination defined by the doc
// NullRecordID means no records for such unique combination or the record is inactive
// type by doc.QName can not have uniques (e.g. not a table) -> error
func GetUniqueRecordID(appStructs istructs.IAppStructs, doc istructs.IRowReader, wsid istructs.WSID) (recID istructs.RecordID, err error) {
	docQName := doc.AsQName(appdef.SystemField_QName)
	uniques, ok := appStructs.AppDef().Type(docQName).(appdef.IUniques)
	if !ok {
		return istructs.NullRecordID, ErrProvidedDocCanNotHaveUniques
	}
	for _, unique := range uniques.Uniques() {
		uniqueQName := coreutils.UniqueQName(docQName, unique.Name())
		recID, exists, err := exists(doc, uniqueQName, unique.Fields(), appStructs, wsid)
		if err != nil {
			return istructs.NullRecordID, err
		}
		if exists {
			return recID, nil
		}
	}
	if uniques.UniqueField() != nil {
		uniqueQName := coreutils.UniqueQName(docQName, uniques.UniqueField().Name())
		recID, _, err := exists(doc, uniqueQName, []appdef.IField{uniques.UniqueField()}, appStructs, wsid)
		if err != nil {
			return istructs.NullRecordID, err
		}
		return recID, nil
	}
	return istructs.NullRecordID, err
}

func exists(doc istructs.IRowReader, uniqueQName appdef.QName, uniqueFields []appdef.IField, appStructs istructs.IAppStructs, wsid istructs.WSID) (recID istructs.RecordID, exists bool, err error) {
	uniqueKeyValues, err := getUniqueKeyValues(doc, uniqueFields, uniqueQName)
	if err != nil {
		return istructs.NullRecordID, false, err
	}
	return getUniqueIDByValues(appStructs, wsid, doc.AsQName(appdef.SystemField_QName), uniqueKeyValues)
}

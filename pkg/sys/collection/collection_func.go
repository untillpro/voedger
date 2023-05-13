/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
*
* @author Michael Saigachenko
*/

package collection

import (
	"context"
	"errors"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/state"
)

func collectionResultQName(args istructs.PrepareArgs) appdef.QName {
	if args.ArgumentObject == nil {
		return appdef.NullQName
	}
	qnameStr := args.ArgumentObject.AsString(field_Schema)
	qname, err := appdef.ParseQName(qnameStr)
	if err != nil {
		return appdef.NullQName // not provided or incorrect
	}
	return qname
}

func collectionFuncExec(ctx context.Context, qf istructs.IQueryFunction, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error) {
	if args.ArgumentObject == nil {
		return errors.New("ArgumentObject is not defined in PrepareArgs")
	}
	qnameStr := args.ArgumentObject.AsString(field_Schema)
	resultsQName, err := appdef.ParseQName(qnameStr)
	if err != nil {
		return err
	}

	kb, err := args.State.KeyBuilder(state.ViewRecordsStorage, QNameViewCollection)
	if err != nil {
		return err
	}
	kb.PutInt32(Field_PartKey, PartitionKeyCollection)
	kb.PutQName(Field_DocQName, resultsQName)
	id := args.ArgumentObject.AsRecordID(field_ID)
	if id != istructs.NullRecordID {
		kb.PutRecordID(field_DocID, id)
	}

	var lastDoc *collectionElement

	err = args.State.Read(kb, func(key istructs.IKey, value istructs.IStateValue) (err error) {
		rec := value.AsRecord(Field_Record)
		docId := key.AsRecordID(field_DocID)

		if lastDoc != nil && lastDoc.ID() == docId {
			lastDoc.addRawRecord(rec)
		} else {
			if lastDoc != nil {
				lastDoc.handleRawRecords()
				err = callback(lastDoc)
			}
			cobj := newCollectionElement(rec)
			lastDoc = &cobj
		}
		return
	})
	if lastDoc != nil && err == nil {
		lastDoc.handleRawRecords()
		err = callback(lastDoc)
	}
	return err
}

/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package collection

import (
	"context"
	"encoding/json"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	istructsmem "github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/state"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

func provideStateFunc(cfg *istructsmem.AppConfigType, appDefBuilder appdef.IAppDefBuilder) {
	cfg.Resources.Add(istructsmem.NewQueryFunction(
		qNameQueryState,
		// local tests -> params and result will be used as declared here
		// runtime -> params and result will be replaced with ones from sql
		appDefBuilder.AddObject(appdef.NewQName(appdef.SysPackage, "StateParams_local")).
			AddField(field_After, appdef.DataKind_int64, true).(appdef.IType).QName(),
		appDefBuilder.AddObject(appdef.NewQName(appdef.SysPackage, "StateResult_local")).
			AddField(field_State, appdef.DataKind_string, true).(appdef.IType).QName(),
		stateFuncExec(appDefBuilder)))
}

func stateFuncExec(appDef appdef.IAppDef) istructsmem.ExecQueryClosure {
	return func(ctx context.Context, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error) {
		after := args.ArgumentObject.AsInt64(field_After)

		kb, err := args.State.KeyBuilder(state.View, QNameCollectionView)
		if err != nil {
			return err
		}
		kb.PutInt32(Field_PartKey, PartitionKeyCollection)

		data := make(map[string]map[istructs.RecordID]map[string]interface{})
		if err = args.State.Read(kb, func(key istructs.IKey, value istructs.IStateValue) (err error) {
			if value.AsInt64(state.ColOffset) <= after {
				return
			}
			record := value.AsRecord(Field_Record)
			_, ok := data[record.QName().String()]
			if !ok {
				data[record.QName().String()] = make(map[istructs.RecordID]map[string]interface{})
			}
			recordData := coreutils.FieldsToMap(record, appDef, coreutils.Filter(func(name string, kind appdef.DataKind) bool {
				return name != appdef.SystemField_QName && name != appdef.SystemField_Container
			}))
			data[record.QName().String()][record.ID()] = recordData
			return err
		}); err != nil {
			return
		}
		bb, err := json.Marshal(data)
		if err != nil {
			return
		}
		return callback(&stateObject{data: string(bb)})
	}
}

type stateObject struct {
	istructs.NullObject
	data string
}

func (o stateObject) AsString(string) string { return o.data }

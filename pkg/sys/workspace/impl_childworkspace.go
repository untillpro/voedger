/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 */

package workspace

import (
	"context"
	"fmt"
	"net/http"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/sys/authnz"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

func execCmdInitChildWorkspace(args istructs.ExecCommandArgs) (err error) {
	wsName := args.ArgumentObject.AsString(authnz.Field_WSName)
	kb, err := args.State.KeyBuilder(state.View, QNameViewChildWorkspaceIdx)
	if err != nil {
		return
	}
	kb.PutInt32(field_dummy, 1)
	kb.PutString(authnz.Field_WSName, wsName)
	_, ok, err := args.State.CanExist(kb)
	if err != nil {
		return
	}

	if ok {
		return coreutils.NewHTTPErrorf(http.StatusConflict, fmt.Sprintf("child workspace with name %s already exists", wsName))
	}

	wsKind := args.ArgumentObject.AsQName(authnz.Field_WSKind)
	wsKindInitializationData := args.ArgumentObject.AsString(authnz.Field_WSKindInitializationData)
	templateName := args.ArgumentObject.AsString(field_TemplateName)
	wsClusterID := args.ArgumentObject.AsInt32(authnz.Field_WSClusterID)
	templateParams := args.ArgumentObject.AsString(Field_TemplateParams)

	// Create cdoc.sys.ChildWorkspace
	kb, err = args.State.KeyBuilder(state.Record, authnz.QNameCDocChildWorkspace)
	if err != nil {
		return
	}
	cdocChildWS, err := args.Intents.NewValue(kb)
	if err != nil {
		return
	}
	cdocChildWS.PutRecordID(appdef.SystemField_ID, 1)
	cdocChildWS.PutString(authnz.Field_WSName, wsName)
	cdocChildWS.PutQName(authnz.Field_WSKind, wsKind)
	cdocChildWS.PutString(authnz.Field_WSKindInitializationData, wsKindInitializationData)
	cdocChildWS.PutString(field_TemplateName, templateName)
	cdocChildWS.PutInt32(authnz.Field_WSClusterID, wsClusterID)
	cdocChildWS.PutString(Field_TemplateParams, templateParams)

	return err
}

var projectorChildWorkspaceIdx = func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
	return event.CUDs(func(rec istructs.ICUDRow) (err error) {
		if rec.QName() != authnz.QNameCDocChildWorkspace || !rec.IsNew() {
			return nil
		}

		kb, err := s.KeyBuilder(state.View, QNameViewChildWorkspaceIdx)
		if err != nil {
			return
		}
		kb.PutInt32(field_dummy, 1)
		wsName := rec.AsString(authnz.Field_WSName)
		kb.PutString(authnz.Field_WSName, wsName)

		vb, err := intents.NewValue(kb)
		if err != nil {
			return
		}
		vb.PutInt64(Field_ChildWorkspaceID, int64(rec.ID()))
		return
	})
}

// targetApp/parentWSID/q.sys.QueryChildWorkspaceByName
func qcwbnQryExec(_ context.Context, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) error {
	wsName := args.ArgumentObject.AsString(authnz.Field_WSName)
	kb, err := args.State.KeyBuilder(state.View, QNameViewChildWorkspaceIdx)
	if err != nil {
		return err
	}
	kb.PutInt32(field_dummy, 1)
	kb.PutString(authnz.Field_WSName, wsName)
	childWSIdx, ok, err := args.State.CanExist(kb)
	if err != nil {
		return err
	}
	if !ok {
		return coreutils.NewHTTPErrorf(http.StatusNotFound, "child workspace ", wsName, " not found")
	}
	kb, err = args.State.KeyBuilder(state.Record, appdef.NullQName)
	if err != nil {
		return err
	}
	kb.PutRecordID(state.Field_ID, istructs.RecordID(childWSIdx.AsInt64(Field_ChildWorkspaceID)))
	rec, err := args.State.MustExist(kb)
	if err != nil {
		return err
	}
	return callback(&qcwbnRR{
		wsName:                   rec.AsString(authnz.Field_WSName),
		wsKind:                   rec.AsQName(authnz.Field_WSKind),
		wsKindInitializationData: rec.AsString(authnz.Field_WSKindInitializationData),
		templateName:             rec.AsString(field_TemplateName),
		templateParams:           rec.AsString(Field_TemplateParams),
		wsid:                     rec.AsInt64(authnz.Field_WSID),
		wsError:                  rec.AsString(authnz.Field_WSError),
	})
}

// q.sys.QueryChildWorkspaceByName
type qcwbnRR struct {
	istructs.NullObject
	wsName                   string
	wsKind                   appdef.QName
	wsKindInitializationData string
	templateName             string
	templateParams           string
	wsid                     int64
	wsError                  string
}

func (q *qcwbnRR) AsInt64(string) int64 { return q.wsid }
func (q *qcwbnRR) AsString(name string) string {
	switch name {
	case authnz.Field_WSName:
		return q.wsName
	case authnz.Field_WSKindInitializationData:
		return q.wsKindInitializationData
	case field_TemplateName:
		return q.templateName
	case authnz.Field_WSError:
		return q.wsError
	case authnz.Field_WSKind:
		return q.wsKind.String()
	case Field_TemplateParams:
		return q.templateParams
	default:
		panic("unexpected field to return: " + name)
	}
}

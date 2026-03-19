/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 *
 * @author Michael Saigachenko
 */

package actualizers

import (
	"context"
	"sync"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
)

type asyncActualizerContextState struct {
	lock   sync.Mutex
	err    error
	vvmCtx context.Context
	cancel func()
}

func (s *asyncActualizerContextState) cancelWithError(err error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.err = err
	s.cancel()
}

// @ConcurrentAccess
func (s *asyncActualizerContextState) error() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.err
}

// Returns is projector triggered by event
func ProjectorEvent(prj appdef.IProjector, event istructs.IPLogEvent) (triggeredBy appdef.QName) {

	switch event.QName() {
	case istructs.QNameForError:
		if prj.WantErrors() {
			return istructs.QNameForError
		}
		return appdef.NullQName
	case istructs.QNameForCorruptedData:
		return appdef.NullQName
	}

	t := prj.App().Type(event.QName())
	if prj.Triggers(appdef.OperationKind_Execute, t) {
		return event.QName() // ON EXECUTE <Command> || ON EXECUTE <ODoc>
	}

	if arg := event.ArgumentObject().QName(); arg != appdef.NullQName {
		t := prj.App().Type(arg)
		if prj.Triggers(appdef.OperationKind_ExecuteWithParam, t) {
			return event.ArgumentObject().QName() // ON EXECUTE WITH <ODoc>; ON EXECUTE WITH <Object>
		}
	}

	for rec := range event.CUDs {
		t := prj.App().Type(rec.QName())
		if rec.IsNew() {
			if prj.Triggers(appdef.OperationKind_Insert, t) {
				return rec.QName() // AFTER INSERT <Record>
			}
		} else {
			if prj.Triggers(appdef.OperationKind_Update, t) {
				return rec.QName() // AFTER UPDATE <Record>
			}
			if rec.IsDeactivated() {
				if prj.Triggers(appdef.OperationKind_Deactivate, t) {
					return rec.QName() // AFTER DEACTIVATE <Record>
				}
			} else if rec.IsActivated() {
				if prj.Triggers(appdef.OperationKind_Activate, t) {
					return rec.QName() // AFTER ACTIVATE <Record>
				}
			}
		}
	}
	return appdef.NullQName
}

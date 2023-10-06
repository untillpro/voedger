/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package builtin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/untillpro/goutils/iterate"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	istructsmem "github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/state"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

func provideRefIntegrityValidation(cfg *istructsmem.AppConfigType) {
	cfg.AddSyncProjectors(func(partition istructs.PartitionID) istructs.Projector {
		return istructs.Projector{
			Name: appdef.NewQName(appdef.SysPackage, "ORecordsRegistryProjector"),
			Func: provideRecordsRegistryProjector(cfg),
		}
	})

	cfg.AddCUDValidators(provideRefIntegrityValidator())
	// cfg.AddEventValidators(refIntegrityValidator)
}

// func refIntegrityValidator(ctx context.Context, rawEvent istructs.IRawEvent, appStructs istructs.IAppStructs, wsid istructs.WSID) error {
// 	if coreutils.IsDummyWS(wsid) || rawEvent.QName() == QNameCommandInit {
// 		return nil
// 	}
// 	appDef := appStructs.AppDef()
// 	argType := appDef.Type(rawEvent.ArgumentObject().QName())
// 	if argType.Kind() != appdef.TypeKind_ODoc && argType.Kind() != appdef.TypeKind_ORecord {
// 		return nil
// 	}
// 	argODoc := argType.(appdef.IODoc)
// 	argODocQName := argODoc.QName()
// 	iterate.ForEachError(argODoc.RefFields, func(refField appdef.IRefField) error {
// 		targetID := istructs.RecordID(0) // TODO: !!!!!!!!!!!!
// 		if targetID == istructs.NullRecordID || targetID.IsRaw() {
// 			return nil
// 		}
// 		allowedTargetQNames := refField.Refs()
// 		refToRPossible := len(allowedTargetQNames) == 0
// 		refToOPossible := len(allowedTargetQNames) == 0
// 		for _, allowedTargetQName := range allowedTargetQNames {
// 			targetType := appDef.Type(allowedTargetQName)
// 			switch targetType.Kind() {
// 			case appdef.TypeKind_ODoc, appdef.TypeKind_ORecord:
// 				refToOPossible = true
// 			default:
// 				refToRPossible = true
// 			}
// 		}
// 		if refToRPossible {
// 			if err := checkTargetRecord(appStructs, wsid, targetID, allowedTargetQNames, refField, argODocQName); err != nil {
// 				if !refToOPossible || !errors.Is(err, ErrReferentialIntegrityViolation) {
// 					return err
// 				}
// 			}
// 		}
// 		if refToOPossible {
// 			kb := appStructs.ViewRecords().KeyBuilder(QNameViewORecordsRegistry)
// 			kb.PutRecordID(field_ID, targetID)
// 			kb.PutInt32(field_Dummy, 1)
// 			_, err := appStructs.ViewRecords().Get(wsid, kb)
// 			if err == nil {
// 				return nil
// 			}
// 			if !errors.Is(err, istructsmem.ErrRecordNotFound) {
// 				// notest
// 				return err
// 			}
// 		}

// 		return nil
// 	})
// 	return nil
// }

// func checkTargetRecord(appStructs istructs.IAppStructs, wsid istructs.WSID, targetID istructs.RecordID, allowedTargetQNames []appdef.QName, sourceRefField appdef.IRefField, sourceDocQName appdef.QName) error {
// 	targetRec, err := appStructs.Records().Get(wsid, true, targetID)
// 	if err != nil {
// 		// notest
// 		return err
// 	}
// 	if targetRec.QName() == appdef.NullQName {
// 		return fmt.Errorf("%w: record ID %d referenced by %s.%s does not exist", ErrReferentialIntegrityViolation, targetID, sourceDocQName, sourceRefField.Name())
// 	}
// 	if len(allowedTargetQNames) > 0 && !slices.Contains(allowedTargetQNames, targetRec.QName()) {
// 		return fmt.Errorf("%w: record ID %d referenced by %s.%s is of QName %s whereas %v QNames are only allowed", ErrReferentialIntegrityViolation,
// 			targetID, sourceDocQName, sourceRefField.Name(), targetRec.QName(), sourceRefField.Refs())
// 	}
// 	return nil
// }

func CheckRefIntegrity(obj istructs.IRowReader, appStructs istructs.IAppStructs, wsid istructs.WSID) (err error) {
	appDef := appStructs.AppDef()
	qName := obj.AsQName(appdef.SystemField_QName)
	t := appDef.Type(qName)
	fields := t.(appdef.IFields)
	return iterate.ForEachError(fields.RefFields, func(refField appdef.IRefField) error {
		actualRefID := obj.AsRecordID(refField.Name())
		if actualRefID == istructs.NullRecordID || actualRefID.IsRaw() {
			return nil
		}
		allowedTargetQNames := refField.Refs()

		refToRPossible := len(allowedTargetQNames) == 0
		refToOPossible := len(allowedTargetQNames) == 0
		for _, allowedTargetQName := range allowedTargetQNames {
			targetType := appDef.Type(allowedTargetQName)
			switch targetType.Kind() {
			case appdef.TypeKind_ODoc, appdef.TypeKind_ORecord:
				refToOPossible = true
			default:
				refToRPossible = true
			}
		}
		if refToRPossible {
			actualRefRec, err := appStructs.Records().Get(wsid, true, actualRefID)
			if err != nil {
				// notest
				return err
			}
			if actualRefRec.QName() != appdef.NullQName {
				if len(allowedTargetQNames) > 0 && !slices.Contains(allowedTargetQNames, actualRefRec.QName()) {
					return fmt.Errorf("%w: record ID %d referenced by %s.%s is of QName %s whereas %v QNames are only allowed", ErrReferentialIntegrityViolation,
						actualRefID, qName, refField.Name(), actualRefRec.QName(), refField.Refs())
				}
				return nil
			}
		}
		if refToOPossible {
			kb := appStructs.ViewRecords().KeyBuilder(QNameViewORecordsRegistry)
			kb.PutRecordID(field_ID, actualRefID)
			kb.PutInt32(field_Dummy, 1)
			_, err := appStructs.ViewRecords().Get(wsid, kb)
			if err == nil {
				return nil
			}
			if !errors.Is(err, istructsmem.ErrRecordNotFound) {
				// notest
				return err
			}
		}
		return fmt.Errorf("%w: record ID %d referenced by %s.%s does not exist", ErrReferentialIntegrityViolation, actualRefID, qName, refField.Name())
	})
}

func provideRecordsRegistryProjector(cfg *istructsmem.AppConfigType) func(event istructs.IPLogEvent, st istructs.IState, intents istructs.IIntents) (err error) {
	return func(event istructs.IPLogEvent, st istructs.IState, intents istructs.IIntents) (err error) {
		argType := cfg.AppDef.Type(event.ArgumentObject().QName())
		event.ArgumentObject().
		if !event.ArgumentObject().AsRecordID(appdef.SystemField_ID).IsRaw() || (argType.Kind() != appdef.TypeKind_ODoc && argType.Kind() != appdef.TypeKind_ORecord) {
			return nil
		}
		return writeElementsToORegistry(event.ArgumentObject(), cfg.AppDef, st, intents, event.WLogOffset())
	}
}

func writeElementsToORegistry(root istructs.IElement, appDef appdef.IAppDef, st istructs.IState, intents istructs.IIntents, wLogOffsetToStore istructs.Offset) error {
	if err := writeORegistry(st, intents, root.AsRecordID(appdef.SystemField_ID), wLogOffsetToStore); err != nil {
		// notest
		return err
	}
	return iterate.ForEachError(root.Containers, func(container string) (err error) {
		root.Elements(container, func(el istructs.IElement) {
			if err != nil {
				// notest
				return
			}
			elType := appDef.Type(el.QName())
			if elType.Kind() != appdef.TypeKind_ODoc && elType.Kind() != appdef.TypeKind_ORecord {
				return
			}
			err = writeORegistry(st, intents, el.AsRecordID(appdef.SystemField_ID), wLogOffsetToStore)
		})
		return err
	})
}

func writeORegistry(st istructs.IState, intents istructs.IIntents, idToStore istructs.RecordID, wLogOffsetToStore istructs.Offset) error {
	kb, err := st.KeyBuilder(state.ViewRecordsStorage, QNameViewORecordsRegistry)
	if err != nil {
		// notest
		return err
	}
	kb.PutRecordID(field_ID, idToStore)
	recordsRegistryRecBuilder, err := intents.NewValue(kb)
	if err != nil {
		// notest
		return err
	}
	recordsRegistryRecBuilder.PutInt32(field_Dummy, 1)
	recordsRegistryRecBuilder.PutInt64(field_WLogOffset, int64(wLogOffsetToStore))
	return nil
}

func provideRefIntegrityValidator() istructs.CUDValidator {
	return istructs.CUDValidator{
		MatchFunc: func(qName appdef.QName, wsid istructs.WSID, cmdQName appdef.QName) bool {
			return !coreutils.IsDummyWS(wsid) && cmdQName != QNameCommandInit
		},
		Validate: func(ctx context.Context, appStructs istructs.IAppStructs, cudRow istructs.ICUDRow, wsid istructs.WSID, cmdQName appdef.QName) (err error) {
			if err = CheckRefIntegrity(cudRow, appStructs, wsid); err == nil {
				return nil
			}
			if errors.Is(err, ErrReferentialIntegrityViolation) {
				return coreutils.WrapSysError(err, http.StatusBadRequest)
			}
			// notest
			return coreutils.WrapSysError(err, http.StatusInternalServerError)
		},
	}
}

// func CheckRefIntegrity(obj istructs.IRowReader, appStructs istructs.IAppStructs, wsid istructs.WSID) (err error) {
// 	appDef := appStructs.AppDef()
// 	qName := obj.AsQName(appdef.SystemField_QName)
// 	t := appDef.Type(qName)
// 	fields, ok := t.(appdef.IFields)
// 	if !ok {
// 		return nil
// 	}
// 	return iterate.ForEachError(fields.RefFields, func(refField appdef.IRefField) error {
// 		actualRefID := obj.AsRecordID(refField.Name())
// 		if actualRefID == istructs.NullRecordID || actualRefID.IsRaw() {
// 			return nil
// 		}
// 		allowedTargetQNames := refField.Refs()

// 		refToRPossible := len(allowedTargetQNames) == 0
// 		refToOPossible := len(allowedTargetQNames) == 0
// 		for _, allowedTargetQName := range allowedTargetQNames {
// 			targetType := appDef.Type(allowedTargetQName)
// 			switch targetType.Kind() {
// 			case appdef.TypeKind_ODoc, appdef.TypeKind_ORecord:
// 				refToOPossible = true
// 			default:
// 				refToRPossible = true
// 			}
// 		}
// 		if refToRPossible {
// 			actualRefRec, err := appStructs.Records().Get(wsid, true, actualRefID)
// 			if err != nil {
// 				// notest
// 				return err
// 			}
// 			if actualRefRec.QName() != appdef.NullQName {
// 				if len(allowedTargetQNames) > 0 && !slices.Contains(allowedTargetQNames, actualRefRec.QName()) {
// 					return fmt.Errorf("%w: record ID %d referenced by %s.%s is of QName %s whereas %v QNames are only allowed", ErrReferentialIntegrityViolation,
// 						actualRefID, qName, refField.Name(), actualRefRec.QName(), refField.Refs())
// 				}
// 				return nil
// 			}
// 		}
// 		if refToOPossible {
// 			kb := appStructs.ViewRecords().KeyBuilder(QNameViewORecordsRegistry)
// 			kb.PutRecordID(field_ID, actualRefID)
// 			kb.PutInt32(field_Dummy, 1)
// 			_, err := appStructs.ViewRecords().Get(wsid, kb)
// 			if err == nil {
// 				return nil
// 			}
// 			if !errors.Is(err, istructsmem.ErrRecordNotFound) {
// 				// notest
// 				return err
// 			}
// 		}
// 		return fmt.Errorf("%w: record ID %d referenced by %s.%s does not exist", ErrReferentialIntegrityViolation, actualRefID, qName, refField.Name())
// 	})
// }

/*
 * Copyright (c) 2020-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package uniques

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"

	"github.com/voedger/voedger/pkg/sys"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"

	"github.com/voedger/voedger/pkg/coreutils"
)

func applyUniques(event istructs.IPLogEvent, st istructs.IState, intents istructs.IIntents) (err error) {
	for rec := range event.CUDs {
		appDef := st.AppStructs().AppDef()
		iUniques, ok := appDef.Type(rec.QName()).(appdef.IWithUniques)
		if !ok {
			continue
		}
		for _, unique := range iUniques.Uniques() {
			if err := handleCUD(rec, st, intents, unique.Fields(), unique.Name()); err != nil {
				return err
			}
		}
		if iUniques.UniqueField() != nil {
			uniqueQName := rec.QName()
			if err := handleCUD(rec, st, intents, []appdef.IField{iUniques.UniqueField()}, uniqueQName); err != nil {
				return err
			}
		}
	}
	return nil
}

func handleCUD(cud istructs.ICUDRow, st istructs.IState, intents istructs.IIntents, uniqueFields []appdef.IField, uniqueQName appdef.QName) error {
	if cud.IsNew() {
		return insert(st, cud, intents, uniqueFields, uniqueQName)
	}
	return update(st, cud, intents, uniqueFields, uniqueQName)
}

type uniqueViewRecord struct {
	refRecordID istructs.RecordID
}

func update(st istructs.IState, rec istructs.ICUDRow, intents istructs.IIntents, uniqueFields []appdef.IField, uniqueQName appdef.QName) error {
	// check modified fields
	// case when we're updating unique fields is already dropped by the validator
	// so came here -> we're updating anything but unique fields
	// let's check activation\deactivation

	kb, err := st.KeyBuilder(sys.Storage_Record, rec.QName())
	if err != nil {
		return err
	}
	kb.PutRecordID(sys.Storage_Record_Field_ID, rec.ID())
	currentRecord, err := st.MustExist(kb)
	if err != nil {
		return err
	}

	// we're updating -> unique view record exists
	uniqueViewRecord, uniqueViewKB, ok, err := getUniqueViewRecord(st, currentRecord, uniqueFields, uniqueQName)
	if err != nil {
		return err
	}
	if !ok {
		// was no unique, insert a record, define a unique, update the record -> no record in the view -> fo nothing to keep backward compatibility
		// new unique will work starting from the next new record
		// https://github.com/voedger/voedger/issues/1408
		return nil
	}
	refIDToSet := istructs.NullRecordID
	uniqueViewRecordID := uniqueViewRecord.AsRecordID(field_ID)
	if rec.AsBool(appdef.SystemField_IsActive) {
		if uniqueViewRecordID == istructs.NullRecordID {
			// activating the record whereas previous combination was deactivated -> allow, update the view
			refIDToSet = rec.ID()
		} else {
			// activating the already activated record, unique combination exists for that record -> allow, nothing to do
			// note: case when uniqueViewRecordID != rec.ID() is handled already by the validator, so nothing to do here
			return nil
		}
	} else {
		if rec.ID() != uniqueViewRecordID {
			// deactivating a record whereas unique combination exists for another record -> allow, nothing to do
			return nil
		}
	}
	uniqueViewUpdater, err := intents.UpdateValue(uniqueViewKB, uniqueViewRecord)
	if err != nil {
		return err
	}
	uniqueViewUpdater.PutRecordID(field_ID, refIDToSet)
	return nil
}

func insert(state istructs.IState, rec istructs.IRowReader, intents istructs.IIntents, uniqueFields []appdef.IField, uniqueQName appdef.QName) error {
	uniqueViewRecord, uniqueViewKB, uniqueViewRecordExists, err := getUniqueViewRecord(state, rec, uniqueFields, uniqueQName)
	if err != nil {
		return err
	}
	// no scenario whe we're inserting a deactivated record
	var uniqueViewRecordBuilder istructs.IStateValueBuilder
	if uniqueViewRecordExists {
		// the olny possible case here - we're inserting a new record, the view record exists for this combination and it is relates to an inactive record
		// case when it relates to an active record is already dropped by the validator
		// so just update the existing view record
		uniqueViewRecordBuilder, err = intents.UpdateValue(uniqueViewKB, uniqueViewRecord)
	} else {
		uniqueViewRecordBuilder, err = intents.NewValue(uniqueViewKB)
	}

	if err == nil {
		uniqueViewRecordBuilder.PutRecordID(field_ID, rec.AsRecordID(appdef.SystemField_ID))
	}
	return err
}

func getUniqueViewRecord(st istructs.IState, rec istructs.IRowReader, uniqueFields []appdef.IField, uniqueQName appdef.QName) (istructs.IStateValue, istructs.IStateKeyBuilder, bool, error) {
	uniqueKeyValues, err := getUniqueKeyValuesFromRec(rec, uniqueFields, uniqueQName)
	if err != nil {
		return nil, nil, false, err
	}
	uniqueViewRecordBuilder, err := st.KeyBuilder(sys.Storage_View, qNameViewUniques)
	if err != nil {
		// notest
		return nil, nil, false, err
	}
	buildUniqueViewKeyByValues(uniqueViewRecordBuilder, uniqueQName, uniqueKeyValues)
	sv, ok, err := st.CanExist(uniqueViewRecordBuilder)
	return sv, uniqueViewRecordBuilder, ok, err
}

// new uniques -> QName of the unique, old uniques -> QName of the doc
func buildUniqueViewKeyByValues(kb istructs.IKeyBuilder, qName appdef.QName, uniqueKeyValues []byte) {
	kb.PutQName(field_QName, qName)
	kb.PutInt64(field_ValuesHash, coreutils.HashBytes(uniqueKeyValues))
	kb.PutBytes(field_Values, uniqueKeyValues)
}

func GetUniqueKeyValuesFromMap(values map[string]interface{}, uniqueFields []appdef.IField, uniqueQName appdef.QName) (res []byte, err error) {
	buf := bytes.NewBuffer(nil)
	for _, uniqueField := range uniqueFields {
		val := values[uniqueField.Name()]
		if err := coreutils.CheckValueByKind(val, uniqueField.DataKind()); err != nil {
			return nil, err
		}
		writeUniqueKeyValue(uniqueField, val, buf, uniqueFields)
	}
	return buf.Bytes(), checkUniqueKeyLen(buf, uniqueQName)
}

// uniqueFields is provided just to determine if should handle backward compatibility
func writeUniqueKeyValue(uniqueField appdef.IField, value interface{}, buf *bytes.Buffer, uniqueFields []appdef.IField) {
	switch uniqueField.DataKind() {
	case appdef.DataKind_string:
		if len(uniqueFields) > 1 {
			// backward compatibility
			buf.WriteByte(zeroByte)
		}
		buf.WriteString(value.(string))
	case appdef.DataKind_bytes:
		if len(uniqueFields) > 1 {
			// backward compatibility
			buf.WriteByte(zeroByte)
		}
		buf.Write(value.([]byte))
	case appdef.DataKind_QName:
		buf.WriteByte(zeroByte)
		qNameStr := ""
		if qName, ok := value.(appdef.QName); ok {
			qNameStr = qName.String()
		} else {
			qNameStr = value.(string)
		}
		buf.WriteString(qNameStr)
	default:
		binary.Write(buf, binary.BigEndian, value) // nolint
	}
}

func checkUniqueKeyLen(buf *bytes.Buffer, uniqueQName appdef.QName) error {
	if buf.Len() > int(appdef.MaxFieldLength) {
		return fmt.Errorf(`%w: resulting len of the unique combination "%s" is %d, max %d is allowed. Decrease len of values of unique fields`,
			ErrUniqueValueTooLong, uniqueQName, buf.Len(), appdef.MaxFieldLength)
	}
	return nil
}

func getUniqueKeyValuesFromRec(rec istructs.IRowReader, uniqueFields []appdef.IField, uniqueQName appdef.QName) (res []byte, err error) {
	buf := bytes.NewBuffer(nil)
	for _, uniqueField := range uniqueFields {
		val := coreutils.ReadByKind(uniqueField.Name(), uniqueField.DataKind(), rec)
		writeUniqueKeyValue(uniqueField, val, buf, uniqueFields)
	}
	return buf.Bytes(), checkUniqueKeyLen(buf, uniqueQName)
}

func getCurrentUniqueViewRecord(uniquesState map[appdef.QName]map[appdef.QName]map[string]*uniqueViewRecord,
	cudQName appdef.QName, uniqueKeyValues []byte, appStructs istructs.IAppStructs, wsid istructs.WSID, uniqueQName appdef.QName) (*uniqueViewRecord, error) {
	// why to accumulate in a map?
	//         id:  field: IsActive: Result:
	// stored: 111: xxx    -
	// …
	// cud(I): 222: xxx    +         - should be ok to insert new record
	// …
	// cud(J): 111:        +         - should be denied to restore old record
	cudQNameUniques, ok := uniquesState[cudQName]
	if !ok {
		cudQNameUniques = map[appdef.QName]map[string]*uniqueViewRecord{}
		uniquesState[cudQName] = cudQNameUniques
	}
	uniqueViewRecords, ok := cudQNameUniques[uniqueQName]
	if !ok {
		uniqueViewRecords = map[string]*uniqueViewRecord{}
		cudQNameUniques[uniqueQName] = uniqueViewRecords
	}
	currentUniqueViewRecord, ok := uniqueViewRecords[string(uniqueKeyValues)]
	if !ok {
		currentUniqueRecordID, _, err := getUniqueIDByValues(appStructs, wsid, uniqueQName, uniqueKeyValues)
		if err != nil {
			return nil, err
		}
		currentUniqueViewRecord = &uniqueViewRecord{
			refRecordID: currentUniqueRecordID,
		}
		uniqueViewRecords[string(uniqueKeyValues)] = currentUniqueViewRecord
	}
	return currentUniqueViewRecord, nil
}

func getUniqueIDByValues(appStructs istructs.IAppStructs, wsid istructs.WSID, uniqueQName appdef.QName, uniqueKeyValues []byte) (istructs.RecordID, bool, error) {
	kb := appStructs.ViewRecords().KeyBuilder(qNameViewUniques)
	buildUniqueViewKeyByValues(kb, uniqueQName, uniqueKeyValues)
	val, err := appStructs.ViewRecords().Get(wsid, kb)
	if err == nil {
		return val.AsRecordID(field_ID), true, nil
	}
	if errors.Is(err, istructs.ErrRecordNotFound) {
		err = nil
	}
	return istructs.NullRecordID, false, err
}

func validateCUD(cudRec istructs.ICUDRow, appStructs istructs.IAppStructs, wsid istructs.WSID, uniqueFields []appdef.IField, uniqueQName appdef.QName, uniquesState map[appdef.QName]map[appdef.QName]map[string]*uniqueViewRecord) (err error) {
	var uniqueKeyValues []byte
	var rowSource istructs.IRowReader
	cudQName := cudRec.QName()
	if cudRec.IsNew() {
		// insert -> will get existing values from the current CUD
		rowSource = cudRec
	} else {
		// update -> will get existing values from the stored record
		rowSource, err = appStructs.Records().Get(wsid, true, cudRec.ID())
		if err != nil {
			// notest
			return err
		}
	}
	uniqueKeyValues, err = getUniqueKeyValuesFromRec(rowSource, uniqueFields, uniqueQName)
	if err != nil {
		return err
	}
	// uniqueViewRecord - is for unique combination from current cudRec
	uniqueViewRecord, err := getCurrentUniqueViewRecord(uniquesState, cudQName, uniqueKeyValues, appStructs, wsid, uniqueQName)
	if err != nil {
		return err
	}
	if cudRec.IsNew() {
		// !IsActive is impossible for new records anymore
		if uniqueViewRecord.refRecordID == istructs.NullRecordID {
			// inserting a new active record, the doc record according to this combination is inactive or does not exist -> allow, update its ID in map
			uniqueViewRecord.refRecordID = cudRec.ID()
		} else {
			// inserting a new active record, the doc record according to this combination is active -> deny
			return conflict(cudQName, uniqueViewRecord.refRecordID, uniqueQName)
		}
	} else {
		// update
		// unique view record exists because all unique fields are required.
		// let's deny to update unique fields and handle IsActive state
		for cudModifiedField, newValue := range cudRec.SpecifiedValues {
			for _, uniqueField := range uniqueFields {
				if uniqueField.Name() == cudModifiedField.Name() {
					return fmt.Errorf("%v: unique field «%s» can not be changed: %w", cudQName, uniqueField.Name(), ErrUniqueFieldUpdateDeny)
				}
			}
			if cudModifiedField.Name() == appdef.SystemField_IsActive {
				// we're updating IsActive field here.
				if newValue.(bool) {
					// activating
					if uniqueViewRecord.refRecordID == istructs.NullRecordID {
						// doc rec for this combination does not exist or is inactive (no matter for this cudRec or any other rec),
						// we're activating now -> set current unique combination ref to the cudRec
						uniqueViewRecord.refRecordID = cudRec.ID()
					} else if uniqueViewRecord.refRecordID != cudRec.ID() {
						// we're activating, doc rec for this combination exists, it is active and it is the another rec (not the one we're updating by the current CUD) -> deny
						return conflict(cudQName, uniqueViewRecord.refRecordID, uniqueQName)
					}
				} else {
					// deactivating
					uniqueViewRecord.refRecordID = istructs.NullRecordID
				}
			}
		}
	}
	return nil
}

func eventUniqueValidator(ctx context.Context, rawEvent istructs.IRawEvent, appStructs istructs.IAppStructs, wsid istructs.WSID) error {
	//                    cudQName       uniqueQName  unique-key-bytes
	uniquesState := map[appdef.QName]map[appdef.QName]map[string]*uniqueViewRecord{}

	for cudRec := range rawEvent.CUDs {
		cudUniques, ok := appStructs.AppDef().Type(cudRec.QName()).(appdef.IWithUniques)
		if !ok {
			continue
		}
		for _, unique := range cudUniques.Uniques() {
			if err := validateCUD(cudRec, appStructs, wsid, unique.Fields(), unique.Name(), uniquesState); err != nil {
				return err
			}
		}
		if cudUniques.UniqueField() != nil {
			uniqueQName := cudRec.QName()
			if err := validateCUD(cudRec, appStructs, wsid, []appdef.IField{cudUniques.UniqueField()}, uniqueQName, uniquesState); err != nil {
				return err
			}
		}
	}
	return nil
}

func conflict(docQName appdef.QName, conflictingWithID istructs.RecordID, uniqueQName appdef.QName) error {
	return coreutils.NewHTTPError(http.StatusConflict, fmt.Errorf(`%s: "%s" %w with ID %d`, docQName, uniqueQName, ErrUniqueConstraintViolation, conflictingWithID))
}

/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package uniques

import (
	"errors"
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
)

// returns ID of the record identified by the the provided unique combination
// returns NullRecordID if there is no record by the provided unique combination or if the record is inactive
func GetRecordIDByUniqueCombination(wsid istructs.WSID, tableQName appdef.QName, as istructs.IAppStructs, values map[string]interface{}) (istructs.RecordID, error) {
	tableType := as.AppDef().Type(tableQName)
	if tableType.Kind() == appdef.TypeKind_null {
		return 0, appdef.ErrNotFound("type %q", tableQName)
	}
	table, ok := tableType.(appdef.IDoc) //nnv: why IDoc but not IRecord ??
	if !ok {
		return 0, fmt.Errorf("%q is not a table: %w", tableQName, appdef.ErrInvalidError)
	}
	// let's find the unique by set of field names
	// matchedIUnique := appdef.IUnique(nil)
	matchedUniqueQName := appdef.NullQName
	matchedUniqueFields := []appdef.IField{}
	for _, iUnique := range table.Uniques() {
		fields := iUnique.Fields()
		if len(values) != len(fields) {
			continue
		}
		matchedFieldsCount := 0
		for providedFieldName := range values {
			for _, uniqueField := range fields {
				if uniqueField.Name() == providedFieldName {
					matchedFieldsCount++
					break
				}
			}
		}
		if matchedFieldsCount == len(fields) {
			matchedUniqueQName = iUnique.Name()
			matchedUniqueFields = fields
			break
		}
	}
	if matchedUniqueQName == appdef.NullQName && table.UniqueField() != nil && len(values) == 1 {
		providedUniqueFieldName := ""
		for n := range values {
			providedUniqueFieldName = n
		}
		if table.UniqueField().Name() == providedUniqueFieldName {
			matchedUniqueQName = table.QName()
			matchedUniqueFields = append(matchedUniqueFields, table.UniqueField())
		}
	}
	if matchedUniqueQName == appdef.NullQName {
		return 0, fmt.Errorf("provided set of fields does not match any known unique of %s: %w", tableType.QName(), ErrUniqueNotExist)
	}

	// build unique field values stream
	uniqueKeyValues, err := GetUniqueKeyValuesFromMap(values, matchedUniqueFields, matchedUniqueQName)
	if err != nil {
		return 0, err
	}

	uniqueViewRecordBuilder := as.ViewRecords().KeyBuilder(qNameViewUniques)
	buildUniqueViewKeyByValues(uniqueViewRecordBuilder, matchedUniqueQName, uniqueKeyValues)
	uniqueViewRecord, err := as.ViewRecords().Get(wsid, uniqueViewRecordBuilder)
	if err != nil {
		if errors.Is(err, istructs.ErrRecordNotFound) {
			return 0, nil
		}
		// notest
		return 0, err
	}
	return uniqueViewRecord.AsRecordID(field_ID), nil
}

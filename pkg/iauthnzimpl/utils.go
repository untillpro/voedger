/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package iauthnzimpl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/untillpro/voedger/pkg/iauthnz"
	"github.com/untillpro/voedger/pkg/istructs"
	payloads "github.com/untillpro/voedger/pkg/itokens-payloads"
	"golang.org/x/exp/slices"
)

var TestSubjectRolesGetter = func(context.Context, string, istructs.IAppStructs, istructs.WSID) ([]istructs.QName, error) {
	return nil, nil
}

func IssueAPIToken(appTokens istructs.IAppTokens, duration time.Duration, roles []istructs.QName, wsid istructs.WSID, currentPrincipalPayload payloads.PrincipalPayload) (token string, err error) {
	if wsid == istructs.NullWSID {
		return "", ErrPersonalAccessTokenOnNullWSID
	}
	for _, roleQName := range roles {
		if iauthnz.IsSystemRole(roleQName) {
			return "", ErrPersonalAccessTokenOnSystemRole
		}
		currentPrincipalPayload.Roles = append(currentPrincipalPayload.Roles, payloads.RoleType{
			WSID:  wsid,
			QName: roleQName,
		})
	}
	currentPrincipalPayload.IsAPIToken = true
	return appTokens.IssueToken(duration, &currentPrincipalPayload)
}

func GetComputersRecByDeviceProfileWSID(as istructs.IAppStructs, requestWSID istructs.WSID, deviceProfileWSID istructs.WSID) (computersRec istructs.IRecord, restaurantComputersRec istructs.IRecord, err error) {
	kb := as.ViewRecords().KeyBuilder(qNameViewDeviceProfileWSIDIdx)
	kb.PartitionKey().PutInt64(field_DeviceProfileWSID, int64(deviceProfileWSID))
	kb.ClusteringColumns().PutInt32(field_dummy, 1)
	batchItems := []istructs.ViewRecordGetBatchItem{{Key: kb}}
	if err := as.ViewRecords().GetBatch(requestWSID, batchItems); err != nil {
		return nil, nil, err
	}
	if !batchItems[0].Ok {
		return &istructs.NullObject{}, &istructs.NullObject{}, nil
	}
	view := batchItems[0].Value
	cID := view.AsRecordID(field_ComputersID)
	rcID := view.AsRecordID(field_RestaurantComputersID)

	if computersRec, err = as.Records().Get(requestWSID, true, cID); err != nil {
		return nil, nil, err
	}
	restaurantComputersRec, err = as.Records().Get(requestWSID, true, rcID)
	return
}

func matchOrNotSpecified_Principals(pattern [][]iauthnz.Principal, actualPrns []iauthnz.Principal) (ok bool) {
	if len(pattern) == 0 {
		return true
	}
	for _, prnsAND := range pattern {
		matchedCount := 0
		for _, prnAND := range prnsAND {
			// среди actualPrns должны быть все prnsAND
			isMatched := false
			for _, actualPrn := range actualPrns {
				// тут мы должны найти prnAnd
				if actualPrn.Kind != prnAND.Kind {
					continue
				}
				isMatched = func() bool {
					if prnAND.ID > 0 && prnAND.ID != actualPrn.ID {
						return false
					}
					if len(prnAND.Name) > 0 && prnAND.Name != actualPrn.Name {
						return false
					}
					if prnAND.QName != istructs.NullQName && prnAND.QName != actualPrn.QName {
						return false
					}
					if prnAND.WSID > 0 && prnAND.WSID != actualPrn.WSID {
						return false
					}
					return true
				}()
				if isMatched {
					break
				}
			}
			if isMatched {
				matchedCount++
			}
		}
		if len(prnsAND) == matchedCount {
			return true
		}
	}
	return false
}

func matchOrNotSpecified_OpKinds(arr []iauthnz.OperationKindType, toFind iauthnz.OperationKindType) bool {
	return len(arr) == 0 || slices.Contains(arr, toFind)
}

func matchOrNotSpecified_QNames(arr []istructs.QName, toFind istructs.QName) bool {
	return len(arr) == 0 || slices.Contains(arr, toFind)
}

func authNZToString(req iauthnz.AuthzRequest) string {
	res := strings.Builder{}
	switch req.OperationKind {
	case iauthnz.OperationKind_INSERT:
		res.WriteString("INSERT")
	case iauthnz.OperationKind_UPDATE:
		res.WriteString("UPDATE")
	case iauthnz.OperationKind_EXECUTE:
		res.WriteString("EXECUTE")
	case iauthnz.OperationKind_SELECT:
		res.WriteString("SELECT")
	default:
		res.WriteString("<unknown>")
	}
	res.WriteString(" ")
	res.WriteString(req.Resource.String())
	if len(req.Fields) > 0 {
		res.WriteString(" [" + req.Fields[0])
		for _, fld := range req.Fields[1:] {
			res.WriteString(", " + fld)
		}
		res.WriteString("]")
	}
	return res.String()
}

func prnsToString(prns []iauthnz.Principal) string {
	if len(prns) == 0 {
		return "<no principals>"
	}
	res := strings.Builder{}
	res.WriteString("[")
	for i := 0; i < len(prns); i++ {
		prn := prns[i]
		switch prn.Kind {
		case iauthnz.PrincipalKind_Host:
			res.WriteString("Host")
		case iauthnz.PrincipalKind_User:
			res.WriteString("User")
		case iauthnz.PrincipalKind_Role:
			res.WriteString("Role")
		case iauthnz.PrincipalKind_Group:
			res.WriteString("Group")
		case iauthnz.PrincipalKind_Device:
			res.WriteString("Device")
		default:
			res.WriteString("<unknown>")
		}
		if prn.QName != istructs.NullQName {
			res.WriteString(" " + prn.QName.String())
		} else {
			res.WriteString(" " + prn.Name)
		}
		if prn.ID > 0 {
			res.WriteString(fmt.Sprintf(",ID %d", prn.ID))
		}
		if prn.WSID > 0 {
			res.WriteString(fmt.Sprintf(",WSID %d", prn.WSID))
		}
		if i != len(prns)-1 {
			res.WriteString(";")
		}
	}
	res.WriteString("]")
	return res.String()
}

/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package dml

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

func ParseQuery(query string) (op Op, err error) {
	const (
		// 0 is original query
		operationIdx int = 1 + iota
		operationUpdateIdx
		operationInsertIdx
		operationSelectIdx
		appIdx
		workspaceIdx
		workspaceWSIDOrPartitionIDIdx
		workspaceAppWSNumIdx
		workspaceLoginIdx
		qNameIdx
		offsetOrIDIdx
		parsIdx
	)

	parts := opRegexp.FindStringSubmatch(query)
	if len(parts) == 0 {
		return op, fmt.Errorf("invalid query format: %s", query)
	}

	if appName := parts[appIdx]; appName != "" {
		appName = appName[:len(parts[appIdx])-1]
		owner, app, err := appdef.ParseQualifiedName(appName, `.`)
		if err != nil {
			// notest: avoided already by regexp
			return op, err
		}
		op.AppQName = appdef.NewAppQName(owner, app)
	}

	if workspaceStr := parts[workspaceIdx]; workspaceStr != "" {
		workspaceStr = workspaceStr[:len(parts[workspaceIdx])-1]
		op.Workspace, err = parseWorkspace(workspaceStr)
		if err != nil {
			return op, err
		}
	}

	qNameStr := parts[qNameIdx]
	op.QName, err = appdef.ParseQName(qNameStr)
	if err != nil {
		// notest: avoided already by regexp
		return op, fmt.Errorf("invalid QName %s: %w", qNameStr, err)
	}

	if offsetStr := parts[offsetOrIDIdx]; len(offsetStr) > 0 {
		offsetStr = offsetStr[1:]
		offsetInt, err := strconv.Atoi(offsetStr)
		if err != nil {
			return op, err
		}
		op.EntityID = istructs.IDType(offsetInt)
	}
	pars := strings.TrimSpace(parts[parsIdx])

	operationStr := strings.TrimSpace(parts[operationIdx])
	operationStrLowered := strings.ToLower(operationStr)
	opSQL := "update"
	switch operationStrLowered {
	case "update":
		op.Kind = OpKind_UpdateTable
	case "direct update":
		op.Kind = OpKind_DirectUpdate
	case "update corrupted":
		op.Kind = OpKind_UpdateCorrupted
	case "direct insert":
		op.Kind = OpKind_DirectInsert
	case "insert":
		op.Kind = OpKind_InsertTable
	default:
		if strings.HasPrefix(operationStrLowered, "select") {
			opSQL = operationStr
			op.Kind = OpKind_Select
		} else {
			// notest: avoided already by regexp
			return op, fmt.Errorf(`wrong dml operation kind "%s"`, operationStr)
		}
	}
	if len(pars) > 0 || op.Kind == OpKind_Select {
		op.CleanSQL = strings.TrimSpace(fmt.Sprintf("%s %s %s", opSQL, qNameStr, pars))
	}
	return op, nil
}

func parseWorkspace(workspaceStr string) (workspace Workspace, err error) {
	switch workspaceStr[:1] {
	case "a":
		appWSNumStr := workspaceStr[1:]
		workspace.ID, err = strconv.ParseUint(appWSNumStr, 0, 0)
		workspace.Kind = WorkspaceKind_AppWSNum
	case `"`:
		login := workspaceStr[1 : len(workspaceStr)-1]
		workspace.ID = uint64(coreutils.GetPseudoWSID(istructs.NullWSID, login, istructs.MainClusterID))
		workspace.Kind = WorkspaceKind_PseudoWSID
	default:
		workspace.ID, err = strconv.ParseUint(workspaceStr, 0, 0)
		workspace.Kind = WorkspaceKind_WSID
	}
	return workspace, err
}

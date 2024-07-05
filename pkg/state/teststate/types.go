/*
 * Copyright (c) 2024-present unTill Software Development Group B. V.
 * @author Michael Saigachenko
 */
package teststate

import (
	"github.com/voedger/voedger/pkg/appdef"
	"io"
	"time"

	"github.com/voedger/voedger/pkg/istructs"
)

type TestWorkspace struct {
	WorkspaceDescriptor string
	WSID                istructs.WSID
}

type TestViewValue struct {
	wsid istructs.WSID
	vr   istructs.IViewRecords
	Key  istructs.IKeyBuilder
	Val  istructs.IValueBuilder
}

type HttpRequest struct {
	Timeout time.Duration
	Method  string
	URL     string
	Body    io.Reader
	Headers map[string]string
}

type HttpResponse struct {
	Status  int
	Body    []byte
	Headers map[string][]string
}

type recordItem struct {
	entity       appdef.IFullQName
	id           int
	keyValueList []any
}

type argumentItem struct {
	path         string
	keyValueList []any
}

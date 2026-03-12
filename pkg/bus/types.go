/*
 * Copyright (c) 2025-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package bus

import (
	"context"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/goutils/timeu"
	"github.com/voedger/voedger/pkg/istructs"
)

type Request struct {
	Method   string
	WSID     istructs.WSID // as it came in the request, could be pseudo
	Header   map[string]string
	Resource string
	Body     []byte
	AppQName appdef.AppQName
	Host     string // used by authenticator to emit Host principal
	IsN10N   bool

	// apiV2
	Query          map[string]string
	QName          appdef.QName // e.g. DocName, extension QName, role Qname, view to subscribe on
	WorkspaceQName appdef.QName // actually wsKind
	IsAPIV2        bool
	DocID          istructs.IDType
	APIPath        int
}

type ResponseMeta struct {
	ContentType string
	StatusCode  int
	mode        RespondMode
}

type RespondMode int

const (
	RespondMode_StreamJSON RespondMode = iota
	RespondMode_Single
	RespondMode_StreamEvents
)

type implIRequestSender struct {
	tm             timeu.ITime
	requestHandler RequestHandler
}

type implResponseWriter struct {
	ch        chan any
	clientCtx context.Context
	tm        timeu.ITime
	resultErr *error
}

type implIResponder struct {
	respWriter     *implResponseWriter
	responseMetaCh chan ResponseMeta
	started        bool
}

type IRequestSenderPtr *IRequestSender

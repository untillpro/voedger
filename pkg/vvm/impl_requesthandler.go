/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package vvm

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	ibus "github.com/untillpro/airs-ibus"
	"github.com/untillpro/goutils/logger"
	"github.com/untillpro/ibusmem"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/iprocbus"
	"github.com/voedger/voedger/pkg/istructs"
	commandprocessor "github.com/voedger/voedger/pkg/processors/command"
	queryprocessor "github.com/voedger/voedger/pkg/processors/query"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

func provideIBus(asp istructs.IAppStructsProvider, procbus iprocbus.IProcBus,
	cpchIdx CommandProcessorsChannelGroupIdxType, qpcgIdx QueryProcessorsChannelGroupIdxType,
	cpAmount CommandProcessorsAmountType, vvmApps VVMApps) ibus.IBus {
	var bus ibus.IBus
	bus = ibusmem.Provide(func(requestCtx context.Context, sender interface{}, request ibus.Request) {
		// Handling Command/Query messages
		// router -> SendRequest2(ctx, ...) -> requestHandler(ctx, ... ) - вот этот контекст. Если connection gracefully closed, то этот ctx.Done()
		// т.е. надо этот контекст пробрасывать далее

		if len(request.Resource) <= ShortestPossibleFunctionNameLen {
			coreutils.ReplyBadRequest(bus, sender, "wrong function name: "+request.Resource)
			return
		}
		qName, err := appdef.ParseQName(request.Resource[2:])
		if err != nil {
			coreutils.ReplyBadRequest(bus, sender, "wrong function name: "+request.Resource)
			return
		}
		if logger.IsVerbose() {
			// FIXME: eliminate this. Unlogged params are logged
			logger.Verbose("request body:\n", string(request.Body))
		}
		funcKindMark := request.Resource[:1]
		if funcKindMark != "c" && funcKindMark != "q" {
			coreutils.ReplyBadRequest(bus, sender, "unknown command kind: "+request.Resource)
			return
		}
		appQName, err := istructs.ParseAppQName(request.AppQName)
		if err != nil {
			// protected by router already
			coreutils.ReplyBadRequest(bus, sender, fmt.Sprintf("failed to parse app qualified name %s: %s", request.AppQName, err.Error()))
			return
		}
		if !vvmApps.Exists(appQName) {
			coreutils.ReplyBadRequest(bus, sender, fmt.Sprintf("unknown app %s", request.AppQName))
			return
		}
		as, err := asp.AppStructs(appQName)
		if err != nil {
			coreutils.ReplyInternalServerError(bus, sender, "failed to get appStructs", err)
			return
		}
		resource, isHandled := getResource(as, qName, bus, sender, funcKindMark)
		if isHandled {
			return
		}

		token, err := getPrincipalToken(request)
		if err != nil {
			coreutils.ReplyAccessDeniedUnauthorized(bus, sender, err.Error())
			return
		}

		deliverToProcessors(request, requestCtx, appQName, sender, resource, procbus, bus, token, cpchIdx, qpcgIdx, cpAmount)
	})
	return bus
}

func deliverToProcessors(request ibus.Request, requestCtx context.Context, appQName istructs.AppQName, sender interface{}, resource istructs.IResource,
	procbus iprocbus.IProcBus, bus ibus.IBus, token string, cpchIdx CommandProcessorsChannelGroupIdxType, qpcgIdx QueryProcessorsChannelGroupIdxType,
	cpAmount CommandProcessorsAmountType) {
	switch request.Resource[:1] {
	case "q":
		iqm := queryprocessor.NewQueryMessage(requestCtx, appQName, istructs.WSID(request.WSID), sender, request.Body, resource, request.Host, token)
		if !procbus.Submit(int(qpcgIdx), 0, iqm) {
			coreutils.ReplyErrf(bus, sender, http.StatusServiceUnavailable, "no query processors available")
		}
	case "c":
		channelIdx := request.WSID % int64(cpAmount)
		partitionID := istructs.PartitionID(channelIdx)
		icm := commandprocessor.NewCommandMessage(requestCtx, request.Body, appQName, istructs.WSID(request.WSID), sender, partitionID, resource, token, request.Host)
		if !procbus.Submit(int(cpchIdx), int(channelIdx), icm) {
			coreutils.ReplyErrf(bus, sender, http.StatusServiceUnavailable, fmt.Sprintf("command processor of partition %d is busy", partitionID))
		}
	}
}

func getResource(as istructs.IAppStructs, qName appdef.QName, bus ibus.IBus, sender interface{}, funcKindMark string) (istructs.IResource, bool) {
	resource := as.Resources().QueryResource(qName)
	if resource.Kind() == istructs.ResourceKind_null {
		coreutils.ReplyBadRequest(bus, sender, "unknown function "+qName.String())
		return nil, true
	}
	if _, ok := resource.(istructs.IFunction); !ok ||
		(funcKindMark == "q" && resource.Kind() != istructs.ResourceKind_QueryFunction) ||
		(funcKindMark == "c" && resource.Kind() != istructs.ResourceKind_CommandFunction) {
		coreutils.ReplyBadRequest(bus, sender, fmt.Sprintf(`wrong function kind "%s" for function %s`, funcKindMark, qName.String()))
		return nil, true
	}
	return resource, false
}

func getPrincipalToken(request ibus.Request) (token string, err error) {
	authHeaders := request.Header[coreutils.Authorization]
	if len(authHeaders) == 0 {
		return "", nil
	}
	authHeader := authHeaders[0]
	if strings.HasPrefix(authHeader, coreutils.BearerPrefix) {
		return strings.ReplaceAll(authHeader, coreutils.BearerPrefix, ""), nil
	}
	if strings.HasPrefix(authHeader, "Basic ") {
		return getBasicAuthToken(authHeader)
	}
	return "", errors.New("unsupported Authorization header: " + authHeader)
}

func getBasicAuthToken(authHeader string) (token string, err error) {
	headerValue := strings.ReplaceAll(authHeader, "Basic ", "")
	headerValueBytes, err := base64.StdEncoding.DecodeString(headerValue)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 Basic Authorization header value: %w", err)
	}
	headerValue = string(headerValueBytes)
	if strings.Count(headerValue, ":") != 1 {
		return "", errors.New("unexpected Basic Authorization header format")
	}
	return strings.ReplaceAll(headerValue, ":", ""), nil
}

func (rs *resultSenderErrorFirst) checkRS() {
	if rs.rs == nil {
		rs.rs = rs.bus.SendParallelResponse2(rs.sender)
	}
}

func (rs *resultSenderErrorFirst) StartArraySection(sectionType string, path []string) {
	rs.checkRS()
	rs.rs.StartArraySection(sectionType, path)
}
func (rs *resultSenderErrorFirst) StartMapSection(sectionType string, path []string) {
	rs.checkRS()
	rs.rs.StartMapSection(sectionType, path)
}
func (rs *resultSenderErrorFirst) ObjectSection(sectionType string, path []string, element interface{}) (err error) {
	rs.checkRS()
	return rs.rs.ObjectSection(sectionType, path, element)
}
func (rs *resultSenderErrorFirst) SendElement(name string, element interface{}) (err error) {
	rs.checkRS()
	return rs.rs.SendElement(name, element)
}
func (rs *resultSenderErrorFirst) Close(err error) {
	defer func() {
		if err != nil {
			logger.Error(err)
		}
	}()
	if rs.rs != nil {
		rs.rs.Close(err)
		return
	}
	if err != nil {
		coreutils.ReplyErr(rs.bus, rs.sender, err)
		return
	}
	coreutils.ReplyJSON(rs.bus, rs.sender, http.StatusOK, "{}")
}

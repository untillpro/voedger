/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package commandprocessor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/processors/actualizers"
	"github.com/voedger/voedger/pkg/processors/oldacl"
	"github.com/voedger/voedger/pkg/sys"
	"golang.org/x/exp/maps"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/pipeline"
	"github.com/voedger/voedger/pkg/processors"
	"github.com/voedger/voedger/pkg/sys/authnz"
	"github.com/voedger/voedger/pkg/sys/blobber"
	"github.com/voedger/voedger/pkg/sys/builtin"
	workspacemgmt "github.com/voedger/voedger/pkg/sys/workspace"
)

func (cm *implICommandMessage) Body() []byte                      { return cm.body }
func (cm *implICommandMessage) AppQName() appdef.AppQName         { return cm.appQName }
func (cm *implICommandMessage) WSID() istructs.WSID               { return cm.wsid }
func (cm *implICommandMessage) Responder() bus.IResponder         { return cm.responder }
func (cm *implICommandMessage) PartitionID() istructs.PartitionID { return cm.partitionID }
func (cm *implICommandMessage) RequestCtx() context.Context       { return cm.requestCtx }
func (cm *implICommandMessage) QName() appdef.QName               { return cm.qName }
func (cm *implICommandMessage) Token() string                     { return cm.token }
func (cm *implICommandMessage) Host() string                      { return cm.host }
func (cm *implICommandMessage) APIPath() processors.APIPath       { return cm.apiPath }
func (cm *implICommandMessage) DocID() istructs.RecordID          { return cm.docID }
func (cm *implICommandMessage) Method() string                    { return cm.method }
func (cm *implICommandMessage) Origin() string                    { return cm.origin }

func NewCommandMessage(requestCtx context.Context, body []byte, appQName appdef.AppQName, wsid istructs.WSID,
	responder bus.IResponder, partitionID istructs.PartitionID, qName appdef.QName, token string, host string, apiPath processors.APIPath,
	docID istructs.RecordID, method string, origin string) ICommandMessage {
	return &implICommandMessage{
		body:        body,
		appQName:    appQName,
		wsid:        wsid,
		responder:   responder,
		partitionID: partitionID,
		requestCtx:  requestCtx,
		qName:       qName,
		token:       token,
		host:        host,
		apiPath:     apiPath,
		docID:       docID,
		method:      method,
		origin:      origin,
	}
}

// used in projectors.newSyncBranch()
func (c *cmdWorkpiece) AppPartition() appparts.IAppPartition {
	return c.appPart
}

// used in c.cluster.VSqlUpdate to determinate partitionID by WSID
// used in c.registry.CreateLogin to dtermine if the target app is deployed
func (c *cmdWorkpiece) AppPartitions() appparts.IAppPartitions {
	return c.appParts
}

// need for sync projectors which are using wsid.GetNextWSID()
// need for sync projectors for logging
func (c *cmdWorkpiece) Context() context.Context {
	return c.cmdMes.RequestCtx()
}

// used in projectors.NewSyncActualizerFactoryFactory
func (c *cmdWorkpiece) Event() istructs.IPLogEvent {
	return c.pLogEvent
}

// need for update corrupted in c.cluster.VSqlUpdate and for various funcs of sys package
func (c *cmdWorkpiece) GetAppStructs() istructs.IAppStructs {
	return c.appStructs
}

// need for sync projectors for logging
func (c *cmdWorkpiece) PLogOffset() istructs.Offset {
	return c.pLogOffset
}

// https://github.com/voedger/voedger/issues/3163
func (c *cmdWorkpiece) GetUserPrincipalName() string {
	for _, prn := range c.principals {
		if prn.Kind == iauthnz.PrincipalKind_User {
			return prn.Name
		}
	}
	return ""
}

// borrows app partition for command
func (c *cmdWorkpiece) borrow() (err error) {
	if c.appPart, err = c.appParts.Borrow(c.cmdMes.AppQName(), c.cmdMes.PartitionID(), appparts.ProcessorKind_Command); err != nil {
		if errors.Is(err, appparts.ErrNotFound) || errors.Is(err, appparts.ErrNotAvailableEngines) { // partition is not deployed yet -> ErrNotFound
			return coreutils.NewHTTPError(http.StatusServiceUnavailable, err)
		}
		// notest
		return err
	}
	c.appStructs = c.appPart.AppStructs()
	return nil
}

func (c *cmdWorkpiece) SetPrincipals(prns []iauthnz.Principal) {
	c.principals = prns
}

// releases resources:
//   - borrowed app partition
//   - plog event
func (c *cmdWorkpiece) Release() {
	if ev := c.pLogEvent; ev != nil {
		c.pLogEvent = nil
		ev.Release()
	}
	if ap := c.appPart; ap != nil {
		c.appStructs = nil
		c.appPart = nil
		ap.Release()
	}
}

func borrowAppPart(_ context.Context, cmd *cmdWorkpiece) error {
	return cmd.borrow()
}

func (ap *appPartition) getWorkspace(wsid istructs.WSID) *workspace {
	ws, ok := ap.workspaces[wsid]
	if !ok {
		ws = &workspace{
			NextWLogOffset: istructs.FirstOffset,
			idGenerator:    istructsmem.NewIDGenerator(),
		}
		ap.workspaces[wsid] = ws
	}
	return ws
}

func (cmdProc *cmdProc) getAppPartition(ctx context.Context, cmd *cmdWorkpiece) (err error) {
	appPartitions, ok := cmdProc.appsPartitions[cmd.cmdMes.AppQName()]
	if !ok {
		appPartitions = map[istructs.PartitionID]*appPartition{}
		cmdProc.appsPartitions[cmd.cmdMes.AppQName()] = appPartitions
	}
	appPartition, ok := appPartitions[cmd.cmdMes.PartitionID()]
	if !ok {
		if appPartition, err = cmdProc.recovery(ctx, cmd); err != nil {
			return fmt.Errorf("partition %d recovery failed: %w", cmd.cmdMes.PartitionID(), err)
		}
		appPartitions[cmd.cmdMes.PartitionID()] = appPartition
	}
	cmd.appPartition = appPartition
	return nil
}

func getIWorkspace(_ context.Context, cmd *cmdWorkpiece) (err error) {
	switch cmd.cmdQName {
	case workspacemgmt.QNameCommandCreateWorkspace:
		// cmd.iWorkspace should be nil
	default:
		ws := cmd.wsDesc.AsQName(authnz.Field_WSKind)
		if cmd.iWorkspace = cmd.appStructs.AppDef().WorkspaceByDescriptor(ws); cmd.iWorkspace == nil {
			panic(fmt.Errorf("workspace %s does not exist", ws))
		}
	}

	return nil
}
func getCmdQName(_ context.Context, cmd *cmdWorkpiece) (err error) {
	if cmd.cmdMes.APIPath() == processors.APIPath_Docs {
		cmd.cmdQName = istructs.QNameCommandCUD
	} else {
		cmd.cmdQName = cmd.cmdMes.QName()
	}
	return nil
}

func getICommand(_ context.Context, cmd *cmdWorkpiece) (err error) {
	var cmdType appdef.IType
	if cmd.iWorkspace == nil {
		// DummyWS or c.sys.CreateWorkspace
		cmdType = cmd.appStructs.AppDef().Type(cmd.cmdQName)
	} else {
		if cmdType = cmd.iWorkspace.Type(cmd.cmdQName); cmdType.Kind() == appdef.TypeKind_null {
			return coreutils.NewHTTPErrorf(http.StatusNotFound, fmt.Errorf("command %s does not exist in workspace %s", cmd.cmdQName, cmd.iWorkspace.QName()))
		}
	}
	ok := false
	cmd.iCommand, ok = cmdType.(appdef.ICommand)
	if !ok {
		return fmt.Errorf("%s is not a command", cmd.cmdQName)
	}
	return nil
}

func (cmdProc *cmdProc) getCmdResultBuilder(_ context.Context, cmd *cmdWorkpiece) (err error) {
	cmdResultType := cmd.iCommand.Result()
	if cmdResultType != nil {
		cmd.cmdResultBuilder = cmd.appStructs.ObjectBuilder(cmdResultType.QName())
	}
	return nil
}

func (cmdProc *cmdProc) buildCommandArgs(_ context.Context, cmd *cmdWorkpiece) (err error) {
	cmd.eca.CommandPrepareArgs = istructs.CommandPrepareArgs{
		PrepareArgs: istructs.PrepareArgs{
			ArgumentObject: cmd.argsObject,
			WSID:           cmd.cmdMes.WSID(),
			Workpiece:      cmd,
			Workspace:      cmd.iWorkspace,
		},
		ArgumentUnloggedObject: cmd.unloggedArgsObject,
	}
	return nil
}

func (cmdProc *cmdProc) getHostState(_ context.Context, cmd *cmdWorkpiece) (err error) {
	hs := cmd.hostStateProvider.get(cmd.appStructs, cmd.cmdMes.WSID(), cmd.reb.CUDBuilder(),
		cmd.principals, cmd.cmdMes.Token(), cmd.cmdResultBuilder, cmd.eca.CommandPrepareArgs, cmd.workspace.NextWLogOffset,
		cmd.argsObject, cmd.unloggedArgsObject, cmd.cmdMes.PartitionID(), cmd.cmdMes.Origin())
	hs.ClearIntents()
	cmd.eca.State = hs
	cmd.eca.Intents = hs
	return nil
}

func updateIDGeneratorFromO(root istructs.IObject, findType appdef.FindType, idGen istructs.IIDGenerator) {
	// new IDs only here because update is not allowed for ODocs in Args
	idGen.UpdateOnSync(root.AsRecordID(appdef.SystemField_ID))
	for container := range root.Containers {
		// order of containers here is the order in the schema
		// but order in the request could be different
		// that is not a problem because for ODocs/ORecords ID generator will bump next ID only if syncID is actually next
		for c := range root.Children(container) {
			updateIDGeneratorFromO(c, findType, idGen)
		}
	}
}

func (cmdProc *cmdProc) recovery(ctx context.Context, cmd *cmdWorkpiece) (*appPartition, error) {
	ap := &appPartition{
		workspaces:     map[istructs.WSID]*workspace{},
		nextPLogOffset: istructs.FirstOffset,
	}
	var lastPLogEvent istructs.IPLogEvent
	var lastPLogOffset istructs.Offset
	cb := func(plogOffset istructs.Offset, event istructs.IPLogEvent) (err error) {
		ws := ap.getWorkspace(event.Workspace())

		for rec := range event.CUDs {
			if rec.IsNew() {
				ws.idGenerator.UpdateOnSync(rec.ID())
			}
		}
		ao := event.ArgumentObject()
		if cmd.appStructs.AppDef().Type(ao.QName()).Kind() == appdef.TypeKind_ODoc {
			updateIDGeneratorFromO(ao, cmd.appStructs.AppDef().Type, ws.idGenerator)
		}
		ws.NextWLogOffset = event.WLogOffset() + 1
		ap.nextPLogOffset = plogOffset + 1
		if lastPLogEvent != nil {
			lastPLogEvent.Release() // TODO: eliminate if there will be a better solution, see https://github.com/voedger/voedger/issues/1348
		}
		lastPLogEvent = event
		lastPLogOffset = plogOffset
		return nil
	}

	if err := cmd.appStructs.Events().ReadPLog(ctx, cmd.cmdMes.PartitionID(), istructs.FirstOffset, istructs.ReadToTheEnd, cb); err != nil {
		return nil, err
	}

	if lastPLogEvent != nil {
		// re-apply the last event
		cmd.pLogEvent = lastPLogEvent
		cmd.workspace = ap.getWorkspace(lastPLogEvent.Workspace())
		cmd.workspace.NextWLogOffset-- // cmdProc.storeOp will bump it
		cmd.reapplier = cmd.appStructs.GetEventReapplier(cmd.pLogEvent)
		cmd.pLogOffset = lastPLogOffset // need to get PLogOffset in sync projectors on logging
		if err := cmdProc.storeOp.DoSync(ctx, cmd); err != nil {
			return nil, err
		}
		cmd.pLogEvent = nil
		cmd.workspace = nil
		cmd.reapplier = nil
		cmd.pLogOffset = istructs.NullOffset
		lastPLogEvent.Release() // TODO: eliminate if there will be a better solution, see https://github.com/voedger/voedger/issues/1348
	}

	worskapcesJSON, err := json.Marshal(ap.workspaces)
	if err != nil {
		// notest
		return nil, err
	}
	logger.InfoCtx(cmd.cmdMes.RequestCtx(), "partition ", cmd.cmdMes.PartitionID(), " recovered: nextPLogOffset ", ap.nextPLogOffset, ", workspaces ", string(worskapcesJSON))
	return ap, nil
}

func getIDGenerator(_ context.Context, cmd *cmdWorkpiece) (err error) {
	cmd.idGeneratorReporter = &implIDGeneratorReporter{
		IIDGenerator: cmd.workspace.idGenerator,
		generatedIDs: map[istructs.RecordID]istructs.RecordID{},
	}
	return nil
}

func (cmdProc *cmdProc) putPLog(_ context.Context, cmd *cmdWorkpiece) (err error) {
	if cmd.pLogEvent, err = cmd.appStructs.Events().PutPlog(cmd.rawEvent, nil, cmd.idGeneratorReporter); err != nil {
		cmd.appPartitionRestartScheduled = true
	} else {
		cmd.appPartition.nextPLogOffset++
	}
	return
}

func logEventAndCUDs(_ context.Context, cmd *cmdWorkpiece) (err error) {
	oldRecs := make(map[istructs.RecordID]istructs.IRecord, len(cmd.parsedCUDs))
	for _, pc := range cmd.parsedCUDs {
		if pc.existingRecord != nil {
			oldRecs[istructs.RecordID(pc.id)] = pc.existingRecord // nolint G115
		}
	}
	enrichedLogCtx, err := processors.LogEventAndCUDs(
		cmd.cmdMes.RequestCtx(),
		cmd.pLogEvent,
		cmd.rawEvent.PLogOffset(),
		cmd.appStructs.AppDef(),
		0,
		func(cud istructs.ICUDRow) (bool, string, error) {
			if oldRec, ok := oldRecs[cud.ID()]; ok {
				oldFields, err := json.Marshal(coreutils.FieldsToMap(oldRec, cmd.appStructs.AppDef()))
				if err != nil {
					// notest
					return false, "", err
				}
				return true, fmt.Sprintf("oldfields=%s", oldFields), nil
			}
			return true, "oldfields={}", nil
		},
		"",
	)

	// will not use the ctx enriched by woffset, poffset, evqname for now
	_ = enrichedLogCtx
	return err
}

func getWSDesc(_ context.Context, cmd *cmdWorkpiece) (err error) {
	cmd.wsDesc, err = cmd.appStructs.Records().GetSingleton(cmd.cmdMes.WSID(), authnz.QNameCDocWorkspaceDescriptor)
	return err
}

func checkWSInitialized(_ context.Context, cmd *cmdWorkpiece) (err error) {
	wsDesc := cmd.wsDesc
	if cmd.cmdQName == workspacemgmt.QNameCommandCreateWorkspace || cmd.cmdQName == builtin.QNameCommandInit { // nolint SA1019
		return nil
	}
	if wsDesc.QName() != appdef.NullQName {
		if cmd.cmdQName == blobber.QNameCommandUploadBLOBHelper {
			return nil
		}
		if wsDesc.AsInt64(workspacemgmt.Field_InitCompletedAtMs) > 0 && len(wsDesc.AsString(workspacemgmt.Field_InitError)) == 0 {
			cmd.wsInitialized = true
			return nil
		}
		if cmd.cmdQName == istructs.QNameCommandCUD {
			if iauthnz.IsSystemPrincipal(cmd.principals, cmd.cmdMes.WSID()) {
				// system -> allow any CUD to upload template, see https://github.com/voedger/voedger/issues/648
				return nil
			}
		}
	}
	return processors.ErrWSNotInited
}

func checkWSActive(_ context.Context, cmd *cmdWorkpiece) (err error) {
	if iauthnz.IsSystemPrincipal(cmd.principals, cmd.cmdMes.WSID()) {
		// system -> allow to work in any case
		return nil
	}
	if cmd.wsDesc.QName() == appdef.NullQName {
		return nil
	}
	if cmd.wsDesc.AsInt32(authnz.Field_Status) == int32(authnz.WorkspaceStatus_Active) {
		return nil
	}
	return processors.ErrWSInactive
}

func limitCallRate(_ context.Context, cmd *cmdWorkpiece) (err error) {
	if cmd.appStructs.IsFunctionRateLimitsExceeded(cmd.cmdQName, cmd.cmdMes.WSID()) {
		return coreutils.NewHTTPErrorf(http.StatusTooManyRequests)
	}
	return nil
}

func (cmdProc *cmdProc) authenticate(_ context.Context, cmd *cmdWorkpiece) (err error) {
	if processors.SetPrincipalsForAnonymousOnlyFunc(cmd.appStructs.AppDef(), cmd.cmdQName, cmd.cmdMes.WSID(), cmd) {
		// grant to anonymous -> set token == "" to avoid validating an expired token accidentally kept in cookies
		return nil
	}
	req := iauthnz.AuthnRequest{
		Host:        cmd.cmdMes.Host(),
		RequestWSID: cmd.cmdMes.WSID(),
		Token:       cmd.cmdMes.Token(),
	}
	if cmd.principals, _, err = cmdProc.authenticator.Authenticate(cmd.cmdMes.RequestCtx(), cmd.appStructs,
		cmd.appStructs.AppTokens(), req); err != nil {
		return coreutils.NewHTTPError(http.StatusUnauthorized, err)
	}
	return
}

func getPrincipalsRoles(_ context.Context, cmd *cmdWorkpiece) (err error) {
	cmd.roles = processors.GetRoles(cmd.principals)
	return nil
}

func (cmdProc *cmdProc) authorizeRequest(ctx context.Context, cmd *cmdWorkpiece) (err error) {
	ws := cmd.iWorkspace
	if ws == nil {
		// c.sys.CreateWorkspace
		ws = cmd.iCommand.Workspace()
	}

	newACLOk, err := cmd.appPart.IsOperationAllowed(ws, appdef.OperationKind_Execute, cmd.cmdQName, nil, cmd.roles)
	if err != nil {
		return err
	}
	// TODO: temporary solution. To be eliminated after implementing ACL in VSQL for Air
	oldACLOk := oldacl.IsOperationAllowed(appdef.OperationKind_Execute, cmd.cmdQName, nil, oldacl.EnrichPrincipals(cmd.principals, cmd.cmdMes.WSID()))
	if !newACLOk && !oldACLOk {
		return coreutils.NewHTTPErrorf(http.StatusForbidden)
	}
	if !newACLOk && oldACLOk && logger.IsVerbose() {
		logger.VerboseCtx(cmd.cmdMes.RequestCtx(), "newACL not ok, but oldACL ok. ", appdef.OperationKind_Execute, cmd.cmdQName, cmd.roles)
	}
	return nil
}

func unmarshalRequestBody(_ context.Context, cmd *cmdWorkpiece) (err error) {
	if cmd.iCommand.Param() != nil && cmd.iCommand.Param().QName() == istructs.QNameRaw {
		cmd.requestData["args"] = map[string]interface{}{
			processors.Field_RawObject_Body: string(cmd.cmdMes.Body()),
		}
	} else if err = coreutils.JSONUnmarshal(cmd.cmdMes.Body(), &cmd.requestData); err != nil {
		err = fmt.Errorf("failed to unmarshal request body: %w\n%s", err, cmd.cmdMes.Body())
	}
	return
}

func (cmdProc *cmdProc) getWorkspace(_ context.Context, cmd *cmdWorkpiece) (err error) {
	cmd.workspace = cmd.appPartition.getWorkspace(cmd.cmdMes.WSID())
	return nil
}

func setPLogOffset(_ context.Context, cmd *cmdWorkpiece) (err error) {
	cmd.pLogOffset = cmd.appPartition.nextPLogOffset
	return nil
}

func (cmdProc *cmdProc) getRawEventBuilder(_ context.Context, cmd *cmdWorkpiece) (err error) {
	grebp := istructs.GenericRawEventBuilderParams{
		HandlingPartition: cmd.cmdMes.PartitionID(),
		Workspace:         cmd.cmdMes.WSID(),
		QName:             cmd.cmdQName,
		RegisteredAt:      istructs.UnixMilli(cmdProc.time.Now().UnixMilli()),
		PLogOffset:        cmd.pLogOffset,
		WLogOffset:        cmd.workspace.NextWLogOffset,
	}

	switch cmd.cmdQName {
	case builtin.QNameCommandInit: // nolint SA1019. kept to not to break existing events only
		cmd.reb = cmd.appStructs.Events().GetSyncRawEventBuilder(
			istructs.SyncRawEventBuilderParams{
				SyncedAt:                     istructs.UnixMilli(cmdProc.time.Now().UnixMilli()),
				GenericRawEventBuilderParams: grebp,
			},
		)
	default:
		cmd.reb = cmd.appStructs.Events().GetNewRawEventBuilder(
			istructs.NewRawEventBuilderParams{
				GenericRawEventBuilderParams: grebp,
			},
		)
	}
	return nil
}

func getArgsObject(_ context.Context, cmd *cmdWorkpiece) (err error) {
	if cmd.iCommand.Param() == nil {
		return nil
	}
	aob := cmd.reb.ArgumentObjectBuilder()
	args, exists, err := cmd.requestData.AsObject("args")
	if err != nil {
		return err
	}
	if exists {
		aob.FillFromJSON(args)
	}
	if cmd.argsObject, err = aob.Build(); err != nil {
		err = fmt.Errorf("argument object build failed: %w", err)
	}
	return
}

func getUnloggedArgsObject(_ context.Context, cmd *cmdWorkpiece) (err error) {
	if cmd.iCommand.UnloggedParam() == nil {
		return nil
	}
	auob := cmd.reb.ArgumentUnloggedObjectBuilder()
	unloggedArgs, exists, err := cmd.requestData.AsObject("unloggedArgs")
	if err != nil {
		return err
	}
	if exists {
		auob.FillFromJSON(unloggedArgs)
	}
	if cmd.unloggedArgsObject, err = auob.Build(); err != nil {
		err = fmt.Errorf("unlogged argument object build failed: %w", err)
	}
	return
}

func (xp xPath) Errorf(mes string, args ...interface{}) error {
	return fmt.Errorf(string(xp)+": "+mes, args...)
}

func (xp xPath) Error(err error) error {
	return xp.Errorf("%w", err)
}

func execCommand(ctx context.Context, cmd *cmdWorkpiece) (err error) {
	begin := time.Now()

	err = cmd.appPart.Invoke(ctx, cmd.cmdQName, cmd.eca.State, cmd.eca.Intents)

	cmd.metrics.increase(ExecSeconds, time.Since(begin).Seconds())
	return err
}

// [~server.blobs/cmp.UpdateBLOBOwnership~impl]
// [~server.blobs/tuc.HandleBLOBReferences~impl]
func appendBLOBOwnershipUpdaters(ctx context.Context, cmd *cmdWorkpiece) (err error) {
	for _, cmdParsedCUD := range cmd.parsedCUDs {
		cudSchemaType := cmd.appStructs.AppDef().Type(cmdParsedCUD.qName)
		cudSchema := cudSchemaType.(appdef.IWithFields)
		for cudFieldName, cudFieldValue := range cmdParsedCUD.fields {
			cudSchemaField := cudSchema.Field(cudFieldName)
			refSchemaField, ok := cudSchemaField.(appdef.IRefField)
			if !ok || len(refSchemaField.Refs()) == 0 || !refSchemaField.Ref(blobber.QNameWDocBLOB) {
				continue
			}
			blobIDJSONNumber := cudFieldValue.(json.Number)
			blobIDIntf, err := coreutils.ClarifyJSONNumber(blobIDJSONNumber, appdef.DataKind_RecordID)
			if err != nil {
				// notest
				return err
			}
			blobID := blobIDIntf.(istructs.RecordID)
			blobRecord, err := cmd.appStructs.Records().Get(cmd.cmdMes.WSID(), true, blobID)
			if err != nil {
				// notest
				return err
			}
			cmd.parsedCUDs = append(cmd.parsedCUDs, parsedCUD{
				opKind:         appdef.OperationKind_Update,
				existingRecord: blobRecord,
				id:             int64(blobID), // nolint G115
				qName:          blobber.QNameWDocBLOB,
				fields: coreutils.MapObject{
					blobber.Field_OwnerRecordID: cmdParsedCUD.id,
				},
			})
		}
	}
	return nil
}

func checkResponseIntent(_ context.Context, cmd *cmdWorkpiece) (err error) {
	return processors.CheckResponseIntent(cmd.hostStateProvider.state)
}

func buildRawEvent(_ context.Context, cmd *cmdWorkpiece) (err error) {
	cmd.rawEvent, err = cmd.reb.BuildRawEvent()
	status := http.StatusBadRequest
	if errors.Is(err, istructsmem.ErrRecordIDUniqueViolationError) {
		status = http.StatusConflict
	}
	err = coreutils.WrapSysError(err, status)
	return
}

func validateCmdResult(ctx context.Context, cmd *cmdWorkpiece) (err error) {
	if cmd.cmdResultBuilder != nil {
		cmdResult, err := cmd.cmdResultBuilder.Build()
		if err != nil {
			return err
		}
		cmd.cmdResult = cmdResult
	}
	return nil
}

func (cmdProc *cmdProc) eventValidators(ctx context.Context, cmd *cmdWorkpiece) (err error) {
	for _, appEventValidator := range cmd.appStructs.EventValidators() {
		if err = appEventValidator(ctx, cmd.rawEvent, cmd.appStructs, cmd.cmdMes.WSID()); err != nil {
			return coreutils.WrapSysError(err, http.StatusForbidden)
		}
	}
	return nil
}

func (cmdProc *cmdProc) cudsValidators(ctx context.Context, cmd *cmdWorkpiece) (err error) {
	for _, appCUDValidator := range cmd.appStructs.CUDValidators() {
		for rec := range cmd.rawEvent.CUDs {
			if appCUDValidator.Match(rec, cmd.cmdMes.WSID(), cmd.cmdQName) {
				if err := appCUDValidator.Validate(ctx, cmd.appStructs, rec, cmd.cmdMes.WSID(), cmd.cmdQName, cmd.commandCtxStorage); err != nil {
					return coreutils.WrapSysError(err, http.StatusForbidden)
				}
			}
		}
	}
	return nil
}

func getCommandCtxStorage(ctx context.Context, cmd *cmdWorkpiece) (err error) {
	skbCommandContext, err := cmd.eca.State.KeyBuilder(sys.Storage_CommandContext, sys.Storage_CommandContext)
	if err != nil {
		// notest
		return err
	}
	cmd.commandCtxStorage, err = cmd.eca.State.MustExist(skbCommandContext)
	return err
}

func (cmdProc *cmdProc) validateCUDsQNames(ctx context.Context, cmd *cmdWorkpiece) (err error) {
	if cmd.iWorkspace == nil {
		// c.sys.CreateWorkspace
		return nil
	}
	for cud := range cmd.rawEvent.CUDs {
		if cmd.iWorkspace.Type(cud.QName()) == appdef.NullType {
			return coreutils.NewHTTPErrorf(http.StatusBadRequest, fmt.Errorf("doc %s mentioned in resulting CUDs does not exist in the workspace %s",
				cud.QName(), cmd.wsDesc.AsQName(authnz.Field_WSKind)))
		}
	}
	return nil
}

func parseCUDs(_ context.Context, cmd *cmdWorkpiece) (err error) {
	if cmd.cmdMes.APIPath() == processors.APIPath_Docs {
		return parseCUDs_v2(cmd)
	}
	cuds, _, err := cmd.requestData.AsObjects("cuds")
	if err != nil {
		return err
	}
	if len(cuds) > builtin.MaxCUDs {
		return coreutils.NewHTTPErrorf(http.StatusBadRequest, "too many cuds: ", len(cuds), " is in the request, max is ", builtin.MaxCUDs)
	}
	for cudNumber, cudIntf := range cuds {
		cudXPath := xPath("cuds[" + strconv.Itoa(cudNumber) + "]")
		cudDataMap, ok := cudIntf.(map[string]interface{})
		if !ok {
			return cudXPath.Errorf("not an object")
		}
		cudData := coreutils.MapObject(cudDataMap)

		parsedCUD := parsedCUD{}

		parsedCUD.fields, ok, err = cudData.AsObject("fields")
		if err != nil {
			return cudXPath.Error(err)
		}
		if !ok {
			return cudXPath.Errorf(`"fields" missing`)
		}
		// sys.ID inside -> create, outside -> update
		var idToUpdate int64
		if idToUpdate, _, err = cudData.AsInt64(appdef.SystemField_ID); err != nil {
			return cudXPath.Error(err)
		}
		var rawID int64
		if rawID, _, err = parsedCUD.fields.AsInt64(appdef.SystemField_ID); err != nil {
			return cudXPath.Error(err)
		}

		// update should have priority to e.g. return error if we trying to modify sys.ID
		if idToUpdate > 0 {
			parsedCUD.id = idToUpdate
			if parsedCUD.existingRecord, err = cmd.appStructs.Records().Get(cmd.cmdMes.WSID(), true, istructs.RecordID(parsedCUD.id)); err != nil { // nolint G115
				return
			}
			if parsedCUD.qName = parsedCUD.existingRecord.QName(); parsedCUD.qName == appdef.NullQName {
				return coreutils.NewHTTPError(http.StatusNotFound, cudXPath.Errorf("record with queried id %d does not exist", parsedCUD.id))
			}
			// check for activate\deactivate
			providedIsActiveVal, isActiveModifying, err := parsedCUD.fields.AsBoolean(appdef.SystemField_IsActive)
			if err != nil {
				return err
			}
			// sys.IsActive is modifying -> other fields are not allowed, see [checkIsActiveInCUDs]
			if isActiveModifying {
				parsedCUD.opKind = appdef.OperationKind_Deactivate
				if providedIsActiveVal {
					parsedCUD.opKind = appdef.OperationKind_Activate
				}
			} else {
				parsedCUD.opKind = appdef.OperationKind_Update
			}
		} else if rawID > 0 {
			// create
			parsedCUD.id = rawID
			parsedCUD.opKind = appdef.OperationKind_Insert
			qNameStr, _, err := parsedCUD.fields.AsString(appdef.SystemField_QName)
			if err != nil {
				return cudXPath.Error(err)
			}
			if parsedCUD.qName, err = appdef.ParseQName(qNameStr); err != nil {
				return cudXPath.Error(fmt.Errorf("failed to parse sys.QName: %w", err))
			}
		} else {
			return cudXPath.Error(fmt.Errorf(`"sys.ID" field missing`))
		}

		parsedCUD.xPath = xPath(fmt.Sprintf("%s %s %s", cudXPath, parsedCUD.opKind, parsedCUD.qName))

		cmd.parsedCUDs = append(cmd.parsedCUDs, parsedCUD)
	}
	return err
}

func checkCUDsAllowedInCUDCmdOnly(_ context.Context, cmd *cmdWorkpiece) (err error) {
	if len(cmd.parsedCUDs) > 0 && cmd.cmdQName != istructs.QNameCommandCUD && cmd.cmdQName != builtin.QNameCommandInit { // nolint SA1019
		return errors.New("CUDs allowed for c.sys.CUD command only")
	}
	return nil
}

func checkArgsRefIntegrity(_ context.Context, cmd *cmdWorkpiece) (err error) {
	if cmd.argsObject != nil {
		if err = builtin.CheckRefIntegrity(cmd.argsObject, cmd.appStructs, cmd.cmdMes.WSID()); err != nil {
			return err
		}
	}
	if cmd.unloggedArgsObject != nil {
		return builtin.CheckRefIntegrity(cmd.unloggedArgsObject, cmd.appStructs, cmd.cmdMes.WSID())
	}
	return nil
}

func getStatusCodeOfSuccess(_ context.Context, cmd *cmdWorkpiece) (err error) {
	cmd.statusCodeOfSuccess = http.StatusOK
	if cmd.cmdMes.APIPath() == processors.APIPath_Docs {
		switch cmd.cmdMes.Method() {
		case http.MethodPost:
			cmd.statusCodeOfSuccess = http.StatusCreated
		}
	}
	return nil
}

// not a validator due of https://github.com/voedger/voedger/issues/1125
func checkIsActiveInCUDs(_ context.Context, cmd *cmdWorkpiece) (err error) {
	for _, cud := range cmd.parsedCUDs {
		if cud.opKind != appdef.OperationKind_Update && cud.opKind != appdef.OperationKind_Activate && cud.opKind != appdef.OperationKind_Deactivate {
			continue
		}
		hasOnlySystemFields := true
		sysIsActiveUpdating := false
		isActiveAndOtherFieldsMixedOnUpdate := false
		for fieldName := range cud.fields {
			if !appdef.IsSysField(fieldName) {
				hasOnlySystemFields = false
			} else if fieldName == appdef.SystemField_IsActive {
				sysIsActiveUpdating = true
			}
			if isActiveAndOtherFieldsMixedOnUpdate = sysIsActiveUpdating && !hasOnlySystemFields; isActiveAndOtherFieldsMixedOnUpdate {
				break
			}
		}
		if isActiveAndOtherFieldsMixedOnUpdate {
			return coreutils.NewHTTPError(http.StatusForbidden, errors.New("updating other fields is not allowed if sys.IsActive is updating"))
		}
	}
	return nil
}

func (cmdProc *cmdProc) authorizeRequestCUDs(ctx context.Context, cmd *cmdWorkpiece) (err error) {
	ws := cmd.iWorkspace
	if ws == nil {
		// c.sys.CreateWorkspace
		ws = cmd.iCommand.Workspace()
	}
	for _, parsedCUD := range cmd.parsedCUDs {
		fields := maps.Keys(parsedCUD.fields)

		newACLOk, err := cmd.appPart.IsOperationAllowed(ws, parsedCUD.opKind, parsedCUD.qName, fields, cmd.roles)
		if err != nil {
			return err
		}
		// TODO: temporary solution. To be eliminated after implementing ACL in VSQL for Air
		oldACLOk := oldacl.IsOperationAllowed(parsedCUD.opKind, parsedCUD.qName, fields, oldacl.EnrichPrincipals(cmd.principals, cmd.cmdMes.WSID()))
		if !newACLOk && !oldACLOk {
			return coreutils.NewHTTPError(http.StatusForbidden, parsedCUD.xPath.Errorf("operation forbidden"))
		}
		if !newACLOk && oldACLOk && logger.IsVerbose() {
			logger.VerboseCtx(cmd.cmdMes.RequestCtx(), "newACL not ok, but oldACL ok. ", parsedCUD.opKind, parsedCUD.qName, cmd.roles)
		}
	}
	return
}

func (cmdProc *cmdProc) writeCUDs(_ context.Context, cmd *cmdWorkpiece) (err error) {
	for _, parsedCUD := range cmd.parsedCUDs {
		var cud istructs.IRowWriter
		if parsedCUD.opKind == appdef.OperationKind_Insert {
			cud = cmd.reb.CUDBuilder().Create(parsedCUD.qName)
			cud.PutRecordID(appdef.SystemField_ID, istructs.RecordID(parsedCUD.id)) // nolint G115
		} else {
			cud = cmd.reb.CUDBuilder().Update(parsedCUD.existingRecord)
		}
		if err := coreutils.MapToObject(parsedCUD.fields, cud); err != nil {
			return parsedCUD.xPath.Error(err)
		}
	}
	return nil
}

func (osp *wrongArgsCatcher) OnErr(err error, _ interface{}, _ pipeline.IWorkpieceContext) (newErr error) {
	return coreutils.WrapSysError(err, http.StatusBadRequest)
}

func (cmdProc *cmdProc) notifyAsyncActualizers(ctx context.Context, cmd *cmdWorkpiece) (err error) {
	cmdProc.n10nBroker.Update(in10n.ProjectionKey{
		App:        cmd.cmdMes.AppQName(),
		Projection: actualizers.PLogUpdatesQName,
		WS:         istructs.WSID(cmd.cmdMes.PartitionID()),
	}, cmd.rawEvent.PLogOffset())
	if logger.IsVerbose() {
		logger.VerboseCtx(cmd.cmdMes.RequestCtx(), "async actualizers are notified: offset ", cmd.rawEvent.PLogOffset(), ", pnumber ", cmd.cmdMes.PartitionID())
	}
	return nil
}

func sendResponse(cmd *cmdWorkpiece, handlingError error) {
	if handlingError != nil {
		cmd.metrics.increase(ErrorsTotal, 1.0)
		//if error occurred somewhere in syncProjectors we have to measure elapsed time
		if !cmd.syncProjectorsStart.IsZero() {
			cmd.metrics.increase(ProjectorsSeconds, time.Since(cmd.syncProjectorsStart).Seconds())
		}
		bus.ReplyErr(cmd.cmdMes.Responder(), handlingError)
		return
	}
	body := bytes.NewBufferString(fmt.Sprintf(`{"CurrentWLogOffset":%d`, cmd.pLogEvent.WLogOffset()))
	if len(cmd.idGeneratorReporter.generatedIDs) > 0 {
		body.WriteString(`,"NewIDs":{`)
		for rawID, generatedID := range cmd.idGeneratorReporter.generatedIDs {
			fmt.Fprintf(body, `"%d":%d,`, rawID, generatedID)
		}
		body.Truncate(body.Len() - 1)
		body.WriteString("}")
	}
	if cmd.cmdResult != nil {
		cmdResult := coreutils.ObjectToMap(cmd.cmdResult, cmd.appStructs.AppDef())
		cmdResultBytes, err := json.Marshal(cmdResult)
		if err != nil {
			// notest
			logger.ErrorCtx(cmd.cmdMes.RequestCtx(), "failed to marshal response: "+err.Error(), ", response: ", cmdResult)
			return
		}
		body.WriteString(`,"Result":`)
		body.Write(cmdResultBytes)
	}
	body.WriteString("}")
	res := body.String()
	if cmd.cmdMes.APIPath() != 0 {
		// TODO: temporary solution. Eliminate after switching to APIv2
		pascalCasedResMap := map[string]interface{}{}
		if err := json.Unmarshal([]byte(res), &pascalCasedResMap); err != nil {
			// notest
			panic(err)
		}
		camelCasedResMap := map[string]interface{}{}
		for pascalCasedKey, val := range pascalCasedResMap {
			camelCasedKey := strings.ToLower(pascalCasedKey[:1]) + pascalCasedKey[1:]
			camelCasedResMap[camelCasedKey] = val
		}
		camelCasedResBytes, err := json.Marshal(&camelCasedResMap)
		if err != nil {
			// notest
			panic(err)
		}
		res = string(camelCasedResBytes)
	}
	cmd.cmdResToLog = res
	bus.ReplyJSON(cmd.cmdMes.Responder(), cmd.statusCodeOfSuccess, res)
}

func (idGen *implIDGeneratorReporter) NextID(rawID istructs.RecordID) (storageID istructs.RecordID, err error) {
	storageID, err = idGen.IIDGenerator.NextID(rawID)
	idGen.generatedIDs[rawID] = storageID
	return
}

/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package apps

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/untillpro/goutils/logger"

	sysmonitor "github.com/voedger/voedger/pkg/apps/sys.monitor"
	"github.com/voedger/voedger/pkg/ihttpctl"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istorageimpl/istoragecas"
	"github.com/voedger/voedger/pkg/istructs"
	coreutils "github.com/voedger/voedger/pkg/utils"
	ibus "github.com/voedger/voedger/staging/src/github.com/untillpro/airs-ibus"
)

func NewStaticEmbeddedResources() []ihttpctl.StaticResourcesType {
	return []ihttpctl.StaticResourcesType{
		sysmonitor.New(),
	}
}

func NewRedirectionRoutes() ihttpctl.RedirectRoutes {
	return ihttpctl.RedirectRoutes{
		"(https?://[^/]*)/grafana($|/.*)":    fmt.Sprintf("http://127.0.0.1:%d$2", defaultGrafanaPort),
		"(https?://[^/]*)/prometheus($|/.*)": fmt.Sprintf("http://127.0.0.1:%d$2", defaultPrometheusPort),
	}
}

func NewDefaultRedirectionRoute() ihttpctl.DefaultRedirectRoute {
	return nil
}

func NewAppStorageFactory(params CLIParams) (istorage.IAppStorageFactory, error) {
	if len(params.Storage) == 0 {
		params.Storage = storageTypeCas3
	}
	casParams := defaultCasParams
	switch params.Storage {
	case storageTypeCas1:
		casParams.Hosts = "db-node-1"
		casParams.KeyspaceWithReplication = cas1ReplicationStrategy
	case storageTypeCas3:
		casParams.Hosts = "db-node-1,db-node-2,db-node-3"
		casParams.KeyspaceWithReplication = cas3ReplicationStrategy
	case storageTypeMem:
		return istorage.ProvideMem(), nil
	default:
		return nil, fmt.Errorf("unable to define replication strategy")
	}
	return istoragecas.Provide(casParams)
}

func NewSysRouterRequestHandler(_ context.Context, sender ibus.ISender, request ibus.Request) {
	go func() {
		queryParamsBytes, err := json.Marshal(request.Query)
		if err != nil {
			coreutils.ReplyBadRequest(sender, err.Error())
			return
		}

		switch request.Resource {
		case "c.EchoCommand":
			sender.SendResponse(ibus.Response{
				ContentType: "text/plain",
				StatusCode:  200,
				Data:        []byte(fmt.Sprintf("Hello, %s, %s", string(request.Body), string(queryParamsBytes))),
			})
		case "q.EchoQuery":
			rs := sender.SendParallelResponse()
			rs.StartArraySection("", []string{})
			err := rs.SendElement("Result", []byte(fmt.Sprintf("Hello, %s, %s", string(request.Body), string(queryParamsBytes))))
			if err != nil {
				logger.Error(err)
			}
			rs.Close(nil)
		default:
			coreutils.ReplyBadRequest(sender, fmt.Sprintf("unknown func: %s", request.Resource))
		}
	}()
}

func NewAppRequestHandlers() ihttpctl.AppRequestHandlers {
	return ihttpctl.AppRequestHandlers{
		{
			AppQName:      istructs.AppQName_sys_router,
			NumPartitions: 1,
			Handlers: map[istructs.PartitionID]ibus.RequestHandler{
				istructs.PartitionID(0): NewSysRouterRequestHandler,
			},
		},
	}
}

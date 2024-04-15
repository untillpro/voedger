/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package apps

import (
	"github.com/voedger/voedger/pkg/cluster"
	"github.com/voedger/voedger/pkg/extensionpoints"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/itokens"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/parser"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

type APIs struct {
	itokens.ITokens
	istructs.IAppStructsProvider
	istructsmem.AppConfigsType
	istorage.IAppStorageProvider
	payloads.IAppTokensFactory
	coreutils.IFederation
	coreutils.TimeFunc
	NumCommandProcessors coreutils.CommandProcessorsCount
	NumQueryProcessors   coreutils.QueryProcessorsCount
	//appparts.IAppPartitions
}

type AppBuilder func(apis APIs, cfg *istructsmem.AppConfigType, ep extensionpoints.IExtensionPoint) BuiltInAppDef
type SchemasExportedContent map[string]map[string][]byte // packageName->schemaFilePath->content
type CLIParams struct {
	Storage string
}
type BuiltInAppDef struct {
	cluster.AppDeploymentDescriptor
	AppQName istructs.AppQName
	Packages []parser.PackageFS
}

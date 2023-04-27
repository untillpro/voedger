/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 *
 * @author Michael Saigachenko
 */

package projectors

import "github.com/voedger/voedger/pkg/appdef"

func ProvideAsyncActualizerFactory() AsyncActualizerFactory {
	return asyncActualizerFactory
}

func ProvideSyncActualizerFactory() SyncActualizerFactory {
	return syncActualizerFactory
}

func ProvideOffsetsSchema(appDef appdef.IAppDefBuilder) {
	provideOffsetsSchemaImpl(appDef)
}

func ProvideViewSchema(appDef appdef.IAppDefBuilder, qname appdef.QName, buildFunc BuildViewSchemaFunc) {
	provideViewSchemaImpl(appDef, qname, buildFunc)
}

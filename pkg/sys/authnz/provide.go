/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package authnz

import (
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/apps"
	"github.com/voedger/voedger/pkg/extensionpoints"
)

func Provide(appDefBuilder appdef.IAppDefBuilder, ep extensionpoints.IExtensionPoint) {
	apps.Parse(schemasFS, appdef.SysPackage, ep)
	// appDefBuilder.AddSingleton(QNameCDoc_WorkspaceKind_DeviceProfile)

	// appDefBuilder.AddSingleton(QNameCDoc_WorkspaceKind_UserProfile).
	// 	AddField(Field_DisplayName, appdef.DataKind_string, false) // made not required according to https://dev.untill.com/projects/#!613071

	// appDefBuilder.AddSingleton(QNameCDoc_WorkspaceKind_AppWorkspace)
}

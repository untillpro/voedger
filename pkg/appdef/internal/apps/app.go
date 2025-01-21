/*
 * Copyright (c) 2023-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package apps

import (
	"errors"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appdef/internal/acl"
	"github.com/voedger/voedger/pkg/appdef/internal/comments"
	"github.com/voedger/voedger/pkg/appdef/internal/containers"
	"github.com/voedger/voedger/pkg/appdef/internal/datas"
	"github.com/voedger/voedger/pkg/appdef/internal/fields"
	"github.com/voedger/voedger/pkg/appdef/internal/packages"
	"github.com/voedger/voedger/pkg/appdef/internal/types"
	"github.com/voedger/voedger/pkg/appdef/internal/workspaces"
)

// # Supports:
//   - appdef.IAppDef
type AppDef struct {
	comments.WithComments
	packages.WithPackages
	workspaces.WithWorkspaces
	acl.WithACL
	types.WithTypes
	sysWS *workspaces.Workspace
}

func NewAppDef() *AppDef {
	app := AppDef{
		WithComments:   comments.MakeWithComments(),
		WithPackages:   packages.MakeWithPackages(),
		WithWorkspaces: workspaces.MakeWithWorkspaces(),
		WithACL:        acl.MakeWithACL(),
		WithTypes:      types.MakeWithTypes(),
	}
	app.makeSysPackage()
	return &app
}

func (app *AppDef) AppendType(t appdef.IType) {
	app.WithTypes.AppendType(t)
	app.Changed()
}

func (app *AppDef) Changed() { app.WithWorkspaces.Changed() }

func (app *AppDef) build() (err error) {
	for _, t := range app.Types() {
		err = errors.Join(err, app.validateType(t))
	}

	if err != nil {
		app.Changed()
	}

	return err
}

// Makes system package.
//
// Should be called after appDef is created.
func (app *AppDef) makeSysPackage() {
	packages.AddPackage(&app.WithPackages, appdef.SysPackage, appdef.SysPackagePath)
	app.makeSysWorkspace()
}

// Makes system workspace.
func (app *AppDef) makeSysWorkspace() {
	app.sysWS = workspaces.NewWorkspace(app, appdef.SysWorkspaceQName)
	app.WithWorkspaces.AppendWorkspace(app.sysWS)

	app.makeSysDataTypes()
	app.makeSysStructures()
}

// Makes system data types.
func (app *AppDef) makeSysDataTypes() {
	for k := appdef.DataKind_null + 1; k < appdef.DataKind_FakeLast; k++ {
		_ = datas.NewSysData(app.sysWS, k)
	}
}

func (app *AppDef) makeSysStructures() {
	wsb := workspaces.NewWorkspaceBuilder(app.sysWS)

	// TODO: move this code to sys.vsql (for projectors)
	viewProjectionOffsets := wsb.AddView(appdef.NewQName(appdef.SysPackage, "projectionOffsets"))
	viewProjectionOffsets.Key().PartKey().AddField("partition", appdef.DataKind_int32)
	viewProjectionOffsets.Key().ClustCols().AddField("projector", appdef.DataKind_QName)
	viewProjectionOffsets.Value().AddField("offset", appdef.DataKind_int64, true)

	// TODO: move this code to sys.vsql (for child workspaces)
	viewNextBaseWSID := wsb.AddView(appdef.NewQName(appdef.SysPackage, "NextBaseWSID"))
	viewNextBaseWSID.Key().PartKey().AddField("dummy1", appdef.DataKind_int32)
	viewNextBaseWSID.Key().ClustCols().AddField("dummy2", appdef.DataKind_int32)
	viewNextBaseWSID.Value().AddField("NextBaseWSID", appdef.DataKind_int64, true)
}

func (app *AppDef) validateType(t appdef.IType) (err error) {
	if v, ok := t.(interface{ Validate() error }); ok {
		err = v.Validate()
	}

	err = errors.Join(err,
		fields.ValidateTypeFields(t),
		containers.ValidateTypeContainers(t),
	)

	return err
}

// # Supports:
//   - appdef.IAppDefBuilder
type AppDefBuilder struct {
	comments.CommentBuilder
	packages.PackagesBuilder
	workspaces.WorkspacesBuilder
	app *AppDef
}

func NewAppDefBuilder(app *AppDef) *AppDefBuilder {
	return &AppDefBuilder{
		CommentBuilder:    comments.MakeCommentBuilder(&app.WithComments),
		PackagesBuilder:   packages.MakePackagesBuilder(&app.WithPackages),
		WorkspacesBuilder: workspaces.MakeWorkspacesBuilder(app, &app.WithWorkspaces),
		app:               app,
	}
}

func (ab AppDefBuilder) AppDef() appdef.IAppDef { return ab.app }

func (ab *AppDefBuilder) Build() (appdef.IAppDef, error) {
	if err := ab.app.build(); err != nil {
		return nil, err
	}
	return ab.app, nil
}

func (ab *AppDefBuilder) MustBuild() appdef.IAppDef {
	a, err := ab.Build()
	if err != nil {
		panic(err)
	}
	return a
}

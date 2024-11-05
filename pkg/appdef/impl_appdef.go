/*
 * Copyright (c) 2023-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

import (
	"errors"
	"sort"
)

// # Implements:
//   - IAppDef
type appDef struct {
	comment
	packages     *packages
	sysWS        *workspace
	acl          []*aclRule // adding order should be saved
	types        map[QName]interface{}
	typesOrdered []interface{}
	wsDesc       map[QName]IWorkspace
}

func newAppDef() *appDef {
	app := appDef{
		packages: newPackages(),
		types:    make(map[QName]interface{}),
		wsDesc:   make(map[QName]IWorkspace),
	}
	app.makeSysPackage()
	return &app
}

func (app appDef) ACL(cb func(IACLRule) bool) {
	for _, p := range app.acl {
		if !cb(p) {
			break
		}
	}
}

func (app *appDef) Extension(name QName) IExtension {
	if t := TypeByName(app, name); t != nil {
		if ex, ok := t.(IExtension); ok {
			return ex
		}
	}
	return nil
}

func (app *appDef) Extensions(visit func(IExtension) bool) {
	for t := range TypesByKinds(app, TypeKind_Extensions) {
		if !visit(t.(IExtension)) {
			break
		}
	}
}

func (app appDef) FullQName(name QName) FullQName { return app.packages.fullQName(name) }

func (app *appDef) Function(name QName) IFunction {
	if t := TypeByName(app, name); t != nil {
		if f, ok := t.(IFunction); ok {
			return f
		}
	}
	return nil
}

func (app *appDef) Functions(visit func(IFunction) bool) {
	for t := range TypesByKinds(app, TypeKind_Functions) {
		if !visit(t.(IFunction)) {
			break
		}
	}
}

func (app *appDef) Job(name QName) IJob {
	if t := TypeByNameAndKind(app, name, TypeKind_Job); t != nil {
		return t.(IJob)
	}
	return nil
}

func (app *appDef) Jobs(visit func(IJob) bool) {
	for t := range TypesByKind(app, TypeKind_Job) {
		if !visit(t.(IJob)) {
			break
		}
	}
}

func (app *appDef) Limit(name QName) ILimit {
	if t := TypeByNameAndKind(app, name, TypeKind_Limit); t != nil {
		return t.(ILimit)
	}
	return nil
}

func (app *appDef) Limits(visit func(ILimit) bool) {
	for t := range TypesByKind(app, TypeKind_Limit) {
		if !visit(t.(ILimit)) {
			break
		}
	}
}

func (app appDef) LocalQName(name FullQName) QName { return app.packages.localQName(name) }

func (app *appDef) PackageLocalName(path string) string {
	return app.packages.localNameByPath(path)
}

func (app *appDef) PackageFullPath(local string) string {
	return app.packages.pathByLocalName(local)
}

func (app *appDef) PackageLocalNames() []string {
	return app.packages.localNames()
}

func (app *appDef) Packages(cb func(local, path string) bool) {
	app.packages.forEach(cb)
}

func (app *appDef) Projector(name QName) IProjector {
	if t := TypeByNameAndKind(app, name, TypeKind_Projector); t != nil {
		return t.(IProjector)
	}
	return nil
}

func (app *appDef) Projectors(visit func(IProjector) bool) {
	for t := range TypesByKind(app, TypeKind_Projector) {
		if !visit(t.(IProjector)) {
			break
		}
	}
}

func (app *appDef) Queries(visit func(IQuery) bool) {
	for t := range TypesByKind(app, TypeKind_Query) {
		if !visit(t.(IQuery)) {
			break
		}
	}
}

func (app *appDef) Query(name QName) IQuery {
	if t := TypeByNameAndKind(app, name, TypeKind_Query); t != nil {
		return t.(IQuery)
	}
	return nil
}

func (app *appDef) Rate(name QName) IRate {
	if t := TypeByNameAndKind(app, name, TypeKind_Rate); t != nil {
		return t.(IRate)
	}
	return nil
}

func (app *appDef) Rates(visit func(IRate) bool) {
	for t := range TypesByKind(app, TypeKind_Rate) {
		if !visit(t.(IRate)) {
			break
		}
	}
}

func (app *appDef) Role(name QName) IRole {
	if t := TypeByNameAndKind(app, name, TypeKind_Role); t != nil {
		return t.(IRole)
	}
	return nil
}

func (app *appDef) Roles(visit func(IRole) bool) {
	for t := range TypesByKind(app, TypeKind_Role) {
		if !visit(t.(IRole)) {
			break
		}
	}
}

func (app *appDef) Type(name QName) IType {
	switch name {
	case NullQName:
		return NullType
	}

	if t, ok := anyTypes[name]; ok {
		return t
	}

	if t, ok := app.types[name]; ok {
		return t.(IType)
	}

	return NullType
}

func (app *appDef) Types(visit func(IType) bool) {
	if app.typesOrdered == nil {
		app.typesOrdered = make([]interface{}, 0, len(app.types))
		for _, t := range app.types {
			app.typesOrdered = append(app.typesOrdered, t)
		}
		sort.Slice(app.typesOrdered, func(i, j int) bool {
			return app.typesOrdered[i].(IType).QName().String() < app.typesOrdered[j].(IType).QName().String()
		})
	}
	for _, t := range app.typesOrdered {
		if !visit(t.(IType)) {
			break
		}
	}
}

func (app *appDef) Workspace(name QName) IWorkspace {
	if t := TypeByNameAndKind(app, name, TypeKind_Workspace); t != nil {
		return t.(IWorkspace)
	}
	return nil
}

func (app *appDef) Workspaces(visit func(IWorkspace) bool) {
	for t := range TypesByKind(app, TypeKind_Workspace) {
		if !visit(t.(IWorkspace)) {
			break
		}
	}
}

func (app *appDef) WorkspaceByDescriptor(name QName) IWorkspace {
	return app.wsDesc[name]
}

func (app *appDef) addLimit(name QName, on []QName, rate QName, comment ...string) {
	_ = newLimit(app, app.sysWS, name, on, rate, comment...)
}

func (app *appDef) addPackage(localName, path string) {
	app.packages.add(localName, path)
}

func (app *appDef) addRate(name QName, count RateCount, period RatePeriod, scopes []RateScope, comment ...string) {
	_ = newRate(app, app.sysWS, name, count, period, scopes, comment...)
}

func (app *appDef) addWorkspace(name QName) IWorkspaceBuilder {
	ws := newWorkspace(app, name)
	return newWorkspaceBuilder(ws)
}

func (app *appDef) alterWorkspace(name QName) IWorkspaceBuilder {
	w := app.Workspace(name)
	if w == nil {
		panic(ErrNotFound("workspace «%v»", name))
	}
	return newWorkspaceBuilder(w.(*workspace))
}

func (app *appDef) appendACL(p *aclRule) {
	app.acl = append(app.acl, p)
}

func (app *appDef) appendType(t interface{}) {
	typ := t.(IType)
	name := typ.QName()
	if name == NullQName {
		panic(ErrMissed("%s type name", typ.Kind().TrimString()))
	}
	if TypeByName(app, name) != nil {
		panic(ErrAlreadyExists("type «%v»", name))
	}

	app.types[name] = t
	app.typesOrdered = nil
}

func (app *appDef) build() (err error) {
	for t := range app.Types {
		err = errors.Join(err, validateType(t))
	}
	return err
}

// Makes system package.
//
// Should be called after appDef is created.
func (app *appDef) makeSysPackage() {
	app.packages.add(SysPackage, SysPackagePath)
	app.makeSysWorkspace()
}

// Makes system workspace.
func (app *appDef) makeSysWorkspace() {
	app.sysWS = newWorkspace(app, SysWorkspaceQName)

	app.makeSysDataTypes()

	app.makeSysStructures()

	// TODO: move this code to sys.vsql (for projectors)
	viewProjectionOffsets := app.sysWS.addView(NewQName(SysPackage, "projectionOffsets"))
	viewProjectionOffsets.Key().PartKey().AddField("partition", DataKind_int32)
	viewProjectionOffsets.Key().ClustCols().AddField("projector", DataKind_QName)
	viewProjectionOffsets.Value().AddField("offset", DataKind_int64, true)

	// TODO: move this code to sys.vsql (for child workspaces)
	viewNextBaseWSID := app.sysWS.addView(NewQName(SysPackage, "NextBaseWSID"))
	viewNextBaseWSID.Key().PartKey().AddField("dummy1", DataKind_int32)
	viewNextBaseWSID.Key().ClustCols().AddField("dummy2", DataKind_int32)
	viewNextBaseWSID.Value().AddField("NextBaseWSID", DataKind_int64, true)
}

// Makes system data types.
func (app *appDef) makeSysDataTypes() {
	for k := DataKind_null + 1; k < DataKind_FakeLast; k++ {
		_ = newSysData(app, app.sysWS, k)
	}
}

func (app *appDef) makeSysStructures() {

}

// # Implements:
//   - IAppDefBuilder
type appDefBuilder struct {
	commentBuilder
	app *appDef
}

func newAppDefBuilder(app *appDef) *appDefBuilder {
	return &appDefBuilder{
		commentBuilder: makeCommentBuilder(&app.comment),
		app:            app,
	}
}

func (ab *appDefBuilder) AddLimit(name QName, on []QName, rate QName, comment ...string) {
	ab.app.addLimit(name, on, rate, comment...)
}

func (ab *appDefBuilder) AddPackage(localName, path string) IAppDefBuilder {
	ab.app.addPackage(localName, path)
	return ab
}

func (ab *appDefBuilder) AddRate(name QName, count RateCount, period RatePeriod, scopes []RateScope, comment ...string) {
	ab.app.addRate(name, count, period, scopes, comment...)
}

func (ab *appDefBuilder) AddWorkspace(name QName) IWorkspaceBuilder { return ab.app.addWorkspace(name) }

func (ab *appDefBuilder) AlterWorkspace(name QName) IWorkspaceBuilder {
	return ab.app.alterWorkspace(name)
}

func (ab appDefBuilder) AppDef() IAppDef { return ab.app }

func (ab *appDefBuilder) Build() (IAppDef, error) {
	if err := ab.app.build(); err != nil {
		return nil, err
	}
	return ab.app, nil
}

func (ab *appDefBuilder) MustBuild() IAppDef {
	if err := ab.app.build(); err != nil {
		panic(err)
	}
	return ab.app
}

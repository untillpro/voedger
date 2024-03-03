/*
* Copyright (c) 2023-present unTill Pro, Ltd.
* @author Michael Saigachenko
 */
package parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"

	"github.com/voedger/voedger/pkg/appdef"
)

type iterateCtx struct {
	*basicContext
	pkg        *PackageSchemaAST
	collection IStatementCollection
	parent     *iterateCtx
}

func FindApplication(p *PackageSchemaAST) (result *ApplicationStmt, err error) {
	for _, stmt := range p.Ast.Statements {
		if stmt.Application != nil {
			if result != nil {
				return nil, fmt.Errorf("%s: %w", stmt.Application.Pos.String(), ErrApplicationRedefined)
			}
			result = stmt.Application
		}
	}
	return result, nil
}

func preAnalyse(c *basicContext, p *PackageSchemaAST) {
	iteratePackage(p, c, func(stmt interface{}, ictx *iterateCtx) {
		switch v := stmt.(type) {
		case *TableStmt:
			preAnalyseTable(v, ictx)
		}
	})
}

func analyse(c *basicContext, p *PackageSchemaAST) {
	iteratePackage(p, c, func(stmt interface{}, ictx *iterateCtx) {
		switch v := stmt.(type) {
		case *CommandStmt:
			analyzeCommand(v, ictx)
		case *QueryStmt:
			analyzeQuery(v, ictx)
		case *ProjectorStmt:
			analyseProjector(v, ictx)
		case *TableStmt:
			analyseTable(v, ictx)
		case *WorkspaceStmt:
			analyseWorkspace(v, ictx)
		case *TypeStmt:
			analyseType(v, ictx)
		case *ViewStmt:
			analyseView(v, ictx)
		case *UseTableStmt:
			analyseUseTable(v, ictx)
		case *UseWorkspaceStmt:
			analyseUseWorkspace(v, ictx)
		case *AlterWorkspaceStmt:
			analyseAlterWorkspace(v, ictx)
		case *StorageStmt:
			analyseStorage(v, ictx)
		case *RateStmt:
			analyseRate(v, ictx)
		case *LimitStmt:
			analyseLimit(v, ictx)
		case *GrantStmt:
			analyseGrant(v, ictx)
		}
	})
}

func analyseGrant(grant *GrantStmt, c *iterateCtx) {

	// To
	err := resolveInCtx(grant.To, c, func(f *RoleStmt, _ *PackageSchemaAST) error { return nil })
	if err != nil {
		c.stmtErr(&grant.To.Pos, err)
	}

	// On
	if grant.Command {
		err := resolveInCtx(grant.On, c, func(f *CommandStmt, _ *PackageSchemaAST) error { return nil })
		if err != nil {
			c.stmtErr(&grant.On.Pos, err)
		}
	}

	if grant.Query {
		err := resolveInCtx(grant.On, c, func(f *QueryStmt, _ *PackageSchemaAST) error { return nil })
		if err != nil {
			c.stmtErr(&grant.On.Pos, err)
		}
	}

	if grant.Workspace {
		err := resolveInCtx(grant.On, c, func(f *WorkspaceStmt, _ *PackageSchemaAST) error { return nil })
		if err != nil {
			c.stmtErr(&grant.On.Pos, err)
		}
	}

	if grant.AllCommandsWithTag || grant.AllQueriesWithTag || grant.AllWorkspacesWithTag || (grant.AllTablesWithTag != nil) {
		err := resolveInCtx(grant.On, c, func(f *TagStmt, _ *PackageSchemaAST) error { return nil })
		if err != nil {
			c.stmtErr(&grant.On.Pos, err)
		}
	}

	var table *TableStmt

	if grant.Table != nil {
		err := resolveInCtx(grant.On, c, func(f *TableStmt, _ *PackageSchemaAST) error { table = f; return nil })
		if err != nil {
			c.stmtErr(&grant.On.Pos, err)
		}
	}

	// Grant table actions
	if table != nil {

		checkColumn := func(column Ident) error {
			for _, f := range table.Items {
				if f.Field != nil && f.Field.Name == column {
					return nil
				}
				if f.RefField != nil && f.RefField.Name == column {
					return nil
				}
				if f.NestedTable != nil && f.NestedTable.Name == column {
					return nil
				}
			}
			return ErrUndefinedField(string(column))
		}

		if grant.Table.GrantAll != nil {
			for _, column := range grant.Table.GrantAll.Columns {
				if err := checkColumn(column.Value); err != nil {
					c.stmtErr(&column.Pos, err)
				}
			}
		}

		for _, i := range grant.Table.Items {
			for _, column := range i.Columns {
				if err := checkColumn(column.Value); err != nil {
					c.stmtErr(&column.Pos, err)
				}
			}
		}
	}
}

func analyseUseTable(u *UseTableStmt, c *iterateCtx) {

	var pkg *PackageSchemaAST
	var pkgName Ident = ""
	var err error

	if u.Package != nil {
		pkg, err = findPackage(u.Package.Value, c)
		if err != nil {
			c.stmtErr(&u.Package.Pos, err)
			return
		}
		pkgName = u.Package.Value
	}

	if u.AllTables {
		var iter func(tbl *TableStmt)
		iter = func(tbl *TableStmt) {
			if !tbl.Abstract {
				u.qNames = append(u.qNames, pkg.NewQName(tbl.Name))
			}
			for _, item := range tbl.Items {
				if item.NestedTable != nil {
					iter(&item.NestedTable.Table)
				}
			}

		}
		if pkg == nil {
			pkg = c.pkg
		}
		for _, stmt := range pkg.Ast.Statements {
			if stmt.Table != nil {
				iter(stmt.Table)
			}
		}
	} else {
		defQName := DefQName{Package: pkgName, Name: u.TableName.Value}
		err = resolveInCtx(defQName, c, func(tbl *TableStmt, pkg *PackageSchemaAST) error {
			if tbl.Abstract {
				return ErrUseOfAbstractTable(defQName.String())
			}
			u.qNames = append(u.qNames, pkg.NewQName(tbl.Name))
			return nil
		})
		if err != nil {
			c.stmtErr(&u.TableName.Pos, err)
		}
	}
}

func analyseUseWorkspace(u *UseWorkspaceStmt, c *iterateCtx) {
	resolveFunc := func(f *WorkspaceStmt, _ *PackageSchemaAST) error {
		if f.Abstract {
			return ErrUseOfAbstractWorkspace(string(u.Workspace.Value))
		}
		u.qName = appdef.NewQName(c.pkg.Name, string(u.Workspace.Value))
		return nil
	}
	err := resolveInCtx(DefQName{Package: Ident(c.pkg.Name), Name: u.Workspace.Value}, c, resolveFunc)
	if err != nil {
		c.stmtErr(&u.Workspace.Pos, err)
	}
}

func analyseAlterWorkspace(u *AlterWorkspaceStmt, c *iterateCtx) {
	resolveFunc := func(w *WorkspaceStmt, schema *PackageSchemaAST) error {
		if !w.Alterable && schema != c.pkg {
			return ErrWorkspaceIsNotAlterable(u.Name.String())
		}
		u.alteredWorkspace = w
		return nil
	}
	err := resolveInCtx(u.Name, c, resolveFunc)
	if err != nil {
		c.stmtErr(&u.Name.Pos, err)
	}
}

func analyseStorage(u *StorageStmt, c *iterateCtx) {
	if c.pkg.QualifiedPackageName != appdef.SysPackage {
		c.stmtErr(&u.Pos, ErrStorageDeclaredOnlyInSys)
	}
}

func analyseRate(r *RateStmt, c *iterateCtx) {
	if r.Value.Variable != nil {
		resolved := func(d *DeclareStmt, p *PackageSchemaAST) error {
			r.Value.variable = p.NewQName(d.Name)
			r.Value.declare = d
			return nil
		}
		if err := resolveInCtx(*r.Value.Variable, c, resolved); err != nil {
			c.stmtErr(&r.Value.Variable.Pos, err)
		}
	}
}

func analyseLimit(u *LimitStmt, c *iterateCtx) {
	err := resolveInCtx(u.RateName, c, func(l *RateStmt, schema *PackageSchemaAST) error { return nil })
	if err != nil {
		c.stmtErr(&u.RateName.Pos, err)
	}
	if u.Action.Tag != nil {
		if err = resolveInCtx(*u.Action.Tag, c, func(t *TagStmt, schema *PackageSchemaAST) error { return nil }); err != nil {
			c.stmtErr(&u.Action.Tag.Pos, err)
		}
	} else if u.Action.Command != nil {
		if err = resolveInCtx(*u.Action.Command, c, func(t *CommandStmt, schema *PackageSchemaAST) error { return nil }); err != nil {
			c.stmtErr(&u.Action.Command.Pos, err)
		}

	} else if u.Action.Query != nil {
		if err = resolveInCtx(*u.Action.Query, c, func(t *QueryStmt, schema *PackageSchemaAST) error { return nil }); err != nil {
			c.stmtErr(&u.Action.Query.Pos, err)
		}
	} else if u.Action.Table != nil {
		if err = resolveInCtx(*u.Action.Table, c, func(t *TableStmt, schema *PackageSchemaAST) error { return nil }); err != nil {
			c.stmtErr(&u.Action.Table.Pos, err)
		}
	}
}

func analyseView(view *ViewStmt, c *iterateCtx) {
	view.pkRef = nil
	fields := make(map[string]int)
	for i := range view.Items {
		fe := &view.Items[i]
		if fe.PrimaryKey != nil {
			if view.pkRef != nil {
				c.stmtErr(&fe.PrimaryKey.Pos, ErrPrimaryKeyRedefined)
			} else {
				view.pkRef = fe.PrimaryKey
			}
		}
		if fe.Field != nil {
			f := fe.Field
			if _, ok := fields[string(f.Name.Value)]; ok {
				c.stmtErr(&f.Name.Pos, ErrRedefined(string(f.Name.Value)))
			} else {
				fields[string(f.Name.Value)] = i
			}
		} else if fe.RefField != nil {
			rf := fe.RefField
			if _, ok := fields[string(rf.Name.Value)]; ok {
				c.stmtErr(&rf.Name.Pos, ErrRedefined(string(rf.Name.Value)))
			} else {
				fields[string(rf.Name.Value)] = i
			}
			for i := range rf.RefDocs {
				refDoc := &rf.RefDocs[i]
				err := resolveInCtx(*refDoc, c, func(f *TableStmt, pkg *PackageSchemaAST) error {
					if f.Abstract {
						return ErrReferenceToAbstractTable(refDoc.String())
					}
					rf.refQNames = append(rf.refQNames, pkg.NewQName(f.Name))
					return nil
				})
				if err != nil {
					c.stmtErr(&refDoc.Pos, err)
					continue
				}
			}
		}
	}
	if view.pkRef == nil {
		c.stmtErr(&view.Pos, ErrPrimaryKeyNotDefined)
		return
	}

	for _, pkf := range view.pkRef.PartitionKeyFields {
		index, ok := fields[string(pkf.Value)]
		if !ok {
			c.stmtErr(&pkf.Pos, ErrUndefinedField(string(pkf.Value)))
		}
		fld := view.Items[index].Field
		if fld != nil {
			if fld.Type.Varchar != nil {
				c.stmtErr(&pkf.Pos, ErrViewFieldVarchar(string(pkf.Value)))
			}
			if fld.Type.Bytes != nil {
				c.stmtErr(&pkf.Pos, ErrViewFieldBytes(string(pkf.Value)))
			}
		}
	}

	for ccIndex, ccf := range view.pkRef.ClusteringColumnsFields {
		fieldIndex, ok := fields[string(ccf.Value)]
		last := ccIndex == len(view.pkRef.ClusteringColumnsFields)-1
		if !ok {
			c.stmtErr(&ccf.Pos, ErrUndefinedField(string(ccf.Value)))
		}
		fld := view.Items[fieldIndex].Field
		if fld != nil {
			if fld.Type.Varchar != nil && !last {
				c.stmtErr(&ccf.Pos, ErrVarcharFieldInCC(string(ccf.Value)))
			}
			if fld.Type.Bytes != nil && !last {
				c.stmtErr(&ccf.Pos, ErrBytesFieldInCC(string(ccf.Value)))
			}
		}
	}

	// ResultOf
	var projector *ProjectorStmt
	err := resolveInCtx(view.ResultOf, c, func(f *ProjectorStmt, _ *PackageSchemaAST) error {
		projector = f
		return nil
	})
	if err != nil {
		c.stmtErr(&view.ResultOf.Pos, err)
		return
	}

	var intentForView *StateStorage
	for i := 0; i < len(projector.Intents) && intentForView == nil; i++ {
		var isView bool
		intent := projector.Intents[i]
		if err := resolveInCtx(intent.Storage, c, func(storage *StorageStmt, _ *PackageSchemaAST) error {
			isView = isView || storage.EntityView
			return nil
		}); err != nil {
			c.stmtErr(&intent.Storage.Pos, err)
		}

		if isView {
			for _, entity := range intent.Entities {
				if entity.Name == view.Name && (entity.Package == Ident(c.pkg.Name) || entity.Package == Ident("")) {
					intentForView = &projector.Intents[i]
					break
				}
			}
		}
	}
	if intentForView == nil {
		c.stmtErr(&view.ResultOf.Pos, ErrProjectorDoesNotDeclareViewIntent(projector.GetName(), view.GetName()))
		return
	}

}

func analyzeCommand(cmd *CommandStmt, c *iterateCtx) {

	resolve := func(qn DefQName) {
		typ, _, err := lookupInCtx[*TypeStmt](qn, c)
		if typ == nil && err == nil {
			tbl, _, err := lookupInCtx[*TableStmt](qn, c)
			if tbl == nil && err == nil {
				c.stmtErr(&qn.Pos, ErrUndefinedTypeOrTable(qn))
			}
		}
		if err != nil {
			c.stmtErr(&qn.Pos, err)
		}
	}

	if cmd.Param != nil && cmd.Param.Def != nil {
		resolve(*cmd.Param.Def)
	}
	if cmd.UnloggedParam != nil && cmd.UnloggedParam.Def != nil {
		typ, _, err := lookupInCtx[*TypeStmt](*cmd.UnloggedParam.Def, c)
		if typ == nil && err == nil {
			tbl, _, err := lookupInCtx[*TableStmt](*cmd.UnloggedParam.Def, c)
			if tbl == nil && err == nil {
				c.stmtErr(&cmd.UnloggedParam.Def.Pos, ErrUndefinedTypeOrTable(*cmd.UnloggedParam.Def))
			}
		}
		if err != nil {
			c.stmtErr(&cmd.UnloggedParam.Def.Pos, err)
		}
	}
	if cmd.Returns != nil && cmd.Returns.Def != nil {
		resolve(*cmd.Returns.Def)
	}
	analyseWith(&cmd.With, cmd, c)
	checkState(cmd.State, c, func(sc *StorageScope) bool { return sc.Commands })
	checkIntents(cmd.Intents, c, func(sc *StorageScope) bool { return sc.Commands })
}

func analyzeQuery(query *QueryStmt, c *iterateCtx) {
	if query.Param != nil && query.Param.Def != nil {
		if err := resolveInCtx(*query.Param.Def, c, func(*TypeStmt, *PackageSchemaAST) error { return nil }); err != nil {
			c.stmtErr(&query.Param.Def.Pos, err)
		}

	}
	if query.Returns.Def != nil {
		if err := resolveInCtx(*query.Returns.Def, c, func(*TypeStmt, *PackageSchemaAST) error { return nil }); err != nil {
			c.stmtErr(&query.Returns.Def.Pos, err)
		}
	}
	analyseWith(&query.With, query, c)
	checkState(query.State, c, func(sc *StorageScope) bool { return sc.Queries })
}

func checkStorageEntity(key *StateStorage, f *StorageStmt, c *iterateCtx) error {
	if f.EntityRecord {
		if len(key.Entities) == 0 {
			return ErrStorageRequiresEntity(key.Storage.String())
		}
		for _, entity := range key.Entities {
			resolveFunc := func(f *TableStmt, pkg *PackageSchemaAST) error {
				if f.Abstract {
					return ErrAbstractTableNotAlowedInProjectors(entity.String())
				}
				key.entityQNames = append(key.entityQNames, pkg.NewQName(entity.Name))
				return nil
			}
			if err2 := resolveInCtx(entity, c, resolveFunc); err2 != nil {
				return err2
			}
		}
	}
	if f.EntityView {
		if len(key.Entities) == 0 {
			return ErrStorageRequiresEntity(key.Storage.String())
		}
		for _, entity := range key.Entities {
			if err2 := resolveInCtx(entity, c, func(view *ViewStmt, pkg *PackageSchemaAST) error {
				key.entityQNames = append(key.entityQNames, pkg.NewQName(entity.Name))
				return nil
			}); err2 != nil {
				return err2
			}
		}
	}
	return nil
}

type checkScopeFunc func(sc *StorageScope) bool

func checkState(state []StateStorage, c *iterateCtx, scope checkScopeFunc) {
	for i := range state {
		key := &state[i]
		if err := resolveInCtx(key.Storage, c, func(f *StorageStmt, pkg *PackageSchemaAST) error {
			if e := checkStorageEntity(key, f, c); e != nil {
				return e
			}
			read := false
			for _, op := range f.Ops {
				if op.Get || op.GetBatch || op.Read {
					for i := range op.Scope {
						if scope(&op.Scope[i]) {
							read = true
							break
						}
					}
				}
			}
			if !read {
				return ErrStorageNotInState(key.Storage.String())
			}
			key.storageQName = pkg.NewQName(key.Storage.Name)
			return nil
		}); err != nil {
			c.stmtErr(&key.Storage.Pos, err)
		}
	}
}

func checkIntents(intents []StateStorage, c *iterateCtx, scope checkScopeFunc) {
	for i := range intents {
		key := &intents[i]
		if err := resolveInCtx(key.Storage, c, func(f *StorageStmt, pkg *PackageSchemaAST) error {
			if e := checkStorageEntity(key, f, c); e != nil {
				return e
			}
			read := false
			for _, op := range f.Ops {
				if op.Insert || op.Update {
					for i := range op.Scope {
						if scope(&op.Scope[i]) {
							read = true
							break
						}
					}
				}
			}
			if !read {
				return ErrStorageNotInIntents(key.Storage.String())
			}
			key.storageQName = pkg.NewQName(key.Storage.Name)
			return nil
		}); err != nil {
			c.stmtErr(&key.Storage.Pos, err)
		}
	}
}

func analyseProjector(v *ProjectorStmt, c *iterateCtx) {
	for i := range v.Triggers {
		trigger := &v.Triggers[i]
		for _, qname := range trigger.QNames {
			if len(trigger.TableActions) > 0 {

				wd, pkg, err := lookupInCtx[*WsDescriptorStmt](qname, c)
				if err != nil {
					c.stmtErr(&qname.Pos, err)
					continue
				}
				if wd != nil {
					trigger.qNames = append(trigger.qNames, pkg.NewQName(wd.Name))
					continue
				}

				resolveFunc := func(table *TableStmt, pkg *PackageSchemaAST) error {
					sysDoc := (pkg.QualifiedPackageName == appdef.SysPackage) && (table.Name == nameCRecord || table.Name == nameWRecord)
					if table.Abstract && !sysDoc {
						return ErrAbstractTableNotAlowedInProjectors(qname.String())
					}
					k, _, err := getTableTypeKind(table, pkg, c)
					if err != nil {
						return err
					}
					if k == appdef.TypeKind_ODoc {
						if trigger.activate() || trigger.deactivate() || trigger.update() {
							return ErrOnlyInsertForOdocOrORecord
						}
					}
					trigger.qNames = append(trigger.qNames, pkg.NewQName(table.Name))
					return nil
				}
				if err := resolveInCtx(qname, c, resolveFunc); err != nil {
					c.stmtErr(&qname.Pos, err)
				}
			} else { // Command
				if trigger.ExecuteAction.WithParam {
					var pkg *PackageSchemaAST
					var odoc *TableStmt
					typ, pkg, err := lookupInCtx[*TypeStmt](qname, c)
					if err != nil { // type?
						c.stmtErr(&qname.Pos, err)
						continue
					}
					if typ == nil { // ODoc?
						odoc, pkg, err = lookupInCtx[*TableStmt](qname, c)
						if err != nil {
							c.stmtErr(&qname.Pos, err)
							continue
						}
						if odoc == nil || odoc.tableTypeKind != appdef.TypeKind_ODoc {
							c.stmtErr(&qname.Pos, ErrUndefinedTypeOrOdoc(qname))
							continue
						}
					}
					trigger.qNames = append(trigger.qNames, pkg.NewQName(qname.Name))
				} else {
					err := resolveInCtx(qname, c, func(f *CommandStmt, pkg *PackageSchemaAST) error {
						trigger.qNames = append(trigger.qNames, pkg.NewQName(qname.Name))
						return nil
					})
					if err != nil {
						c.stmtErr(&qname.Pos, err)
						continue
					}
				}
			}
		}
	}
	checkState(v.State, c, func(sc *StorageScope) bool { return sc.Projectors })
	checkIntents(v.Intents, c, func(sc *StorageScope) bool { return sc.Projectors })
}

// Note: function may update with argument
func analyseWith(with *[]WithItem, statement IStatement, c *iterateCtx) {
	var comment *WithItem

	for i := range *with {
		item := &(*with)[i]
		if item.Comment != nil {
			comment = item
		}
		for j := range item.Tags {
			tag := item.Tags[j]
			if err := resolveInCtx(tag, c, func(*TagStmt, *PackageSchemaAST) error { return nil }); err != nil {
				c.stmtErr(&tag.Pos, err)
			}
		}
	}

	if comment != nil {
		statement.SetComments(strings.Split(*comment.Comment, "\n"))
	}
}

func preAnalyseTable(v *TableStmt, c *iterateCtx) {
	var err error
	v.tableTypeKind, v.singleton, err = getTableTypeKind(v, c.pkg, c)
	if err != nil {
		c.stmtErr(&v.Pos, err)
		return
	}
}

func analyseTable(v *TableStmt, c *iterateCtx) {
	analyseWith(&v.With, v, c)
	analyseNestedTables(v.Items, v.tableTypeKind, c)
	analyseFieldSets(v.Items, c)
	analyseFields(v.Items, c, true)
	if v.Inherits != nil {
		resolvedFunc := func(f *TableStmt, _ *PackageSchemaAST) error {
			if !f.Abstract {
				return ErrBaseTableMustBeAbstract
			}
			return nil
		}
		if err := resolveInCtx(*v.Inherits, c, resolvedFunc); err != nil {
			c.stmtErr(&v.Inherits.Pos, err)
		}

	}
}

func analyseType(v *TypeStmt, c *iterateCtx) {
	for _, i := range v.Items {
		if i.NestedTable != nil {
			c.stmtErr(&i.NestedTable.Pos, ErrNestedTablesNotSupportedInTypes)
		}
	}
	analyseFieldSets(v.Items, c)
	analyseFields(v.Items, c, false)
}

func analyseWorkspace(v *WorkspaceStmt, c *iterateCtx) {

	var chain []DefQName
	var checkChain func(qn DefQName) error

	checkChain = func(qn DefQName) error {
		resolveFunc := func(w *WorkspaceStmt, _ *PackageSchemaAST) error {
			if !w.Abstract {
				return ErrBaseWorkspaceMustBeAbstract
			}
			for i := range chain {
				if chain[i] == qn {
					return ErrCircularReferenceInInherits
				}
			}
			chain = append(chain, qn)
			for _, w := range w.Inherits {
				e := checkChain(w)
				if e != nil {
					return e
				}
			}
			return nil
		}
		return resolveInCtx(qn, c, resolveFunc)
	}

	for _, inherits := range v.Inherits {
		chain = make([]DefQName, 0)
		if err := checkChain(inherits); err != nil {
			c.stmtErr(&inherits.Pos, err)
		}
	}
	if v.Descriptor != nil {
		if v.Abstract {
			c.stmtErr(&v.Descriptor.Pos, ErrAbstractWorkspaceDescriptor)
		}
		analyseNestedTables(v.Descriptor.Items, appdef.TypeKind_CDoc, c)
		analyseFieldSets(v.Descriptor.Items, c)
	}
}

func analyseNestedTables(items []TableItemExpr, rootTableKind appdef.TypeKind, c *iterateCtx) {
	for i := range items {
		item := items[i]

		var nestedTable *TableStmt
		var pos *lexer.Position

		if item.NestedTable != nil {
			nestedTable = &item.NestedTable.Table
			pos = &item.NestedTable.Pos

			if nestedTable.Abstract {
				c.stmtErr(pos, ErrNestedAbstractTable(nestedTable.GetName()))
				return
			}
			if nestedTable.Inherits == nil {
				var err error
				nestedTable.tableTypeKind, err = getNestedTableKind(rootTableKind)
				if err != nil {
					c.stmtErr(pos, err)
					return
				}
			} else {
				var err error
				nestedTable.tableTypeKind, nestedTable.singleton, err = getTableTypeKind(nestedTable, c.pkg, c)
				if err != nil {
					c.stmtErr(pos, err)
					return
				}
				tk, err := getNestedTableKind(rootTableKind)
				if err != nil {
					c.stmtErr(pos, err)
					return
				}
				if nestedTable.tableTypeKind != tk {
					c.stmtErr(pos, ErrNestedTableIncorrectKind)
					return
				}
			}
			analyseNestedTables(nestedTable.Items, rootTableKind, c)
		}
	}
}

func analyseFieldSets(items []TableItemExpr, c *iterateCtx) {
	for i := range items {
		item := items[i]
		if item.FieldSet != nil {
			if err := resolveInCtx(item.FieldSet.Type, c, func(*TypeStmt, *PackageSchemaAST) error { return nil }); err != nil {
				c.stmtErr(&item.FieldSet.Type.Pos, err)
				continue
			}
		}
		if item.NestedTable != nil {
			nestedTable := &item.NestedTable.Table
			analyseFieldSets(nestedTable.Items, c)
		}
	}
}

func lookupField(items []TableItemExpr, name Ident) bool {
	for i := range items {
		item := items[i]
		if item.Field != nil {
			if item.Field.Name == name {
				return true
			}
		}
	}
	return false
}

func analyseFields(items []TableItemExpr, c *iterateCtx, isTable bool) {
	fieldsInUniques := make([]Ident, 0)
	constraintNames := make(map[string]bool)
	for i := range items {
		item := items[i]
		if item.Field != nil {
			field := item.Field
			if field.CheckRegexp != nil {
				if field.Type.DataType != nil && field.Type.DataType.Varchar != nil {
					_, err := regexp.Compile(field.CheckRegexp.Regexp)
					if err != nil {
						c.stmtErr(&field.CheckRegexp.Pos, ErrCheckRegexpErr(err))
					}
				} else {
					c.stmtErr(&field.CheckRegexp.Pos, ErrRegexpCheckOnlyForVarcharField)
				}
			}
			if field.Type.DataType != nil {
				vc := field.Type.DataType.Varchar
				if vc != nil && vc.MaxLen != nil {
					if *vc.MaxLen > uint64(appdef.MaxFieldLength) {
						c.stmtErr(&vc.Pos, ErrMaxFieldLengthTooLarge)
					}
				}
				bb := field.Type.DataType.Bytes
				if bb != nil && bb.MaxLen != nil {
					if *bb.MaxLen > uint64(appdef.MaxFieldLength) {
						c.stmtErr(&bb.Pos, ErrMaxFieldLengthTooLarge)
					}
				}
			} else {
				if !isTable { // analysing a TYPE
					err := resolveInCtx(*field.Type.Def, c, func(f *TypeStmt, pkg *PackageSchemaAST) error {
						field.Type.qName = pkg.NewQName(f.Name)
						return nil
					})
					if err != nil {
						c.stmtErr(&field.Type.Def.Pos, err)
						continue
					}
				} else { // analysing a TABLE
					err := resolveInCtx(*field.Type.Def, c, func(f *TableStmt, pkg *PackageSchemaAST) error {
						if f.Abstract {
							return ErrNestedAbstractTable(field.Type.Def.String())
						}
						if f.tableTypeKind != appdef.TypeKind_CRecord && f.tableTypeKind != appdef.TypeKind_ORecord && f.tableTypeKind != appdef.TypeKind_WRecord {
							return ErrTypeNotSupported(field.Type.Def.String())
						}
						field.Type.qName = pkg.NewQName(f.Name)
						field.Type.tableStmt = f
						field.Type.tablePkg = pkg
						return nil
					})
					if err != nil {
						if err.Error() == ErrUndefinedTable(*field.Type.Def).Error() {
							c.stmtErr(&field.Type.Def.Pos, ErrUndefinedDataTypeOrTable(*field.Type.Def))
						} else {
							c.stmtErr(&field.Type.Def.Pos, err)
						}
						continue
					}
				}
			}
		}
		if item.RefField != nil {
			rf := item.RefField
			for i := range rf.RefDocs {
				if err := resolveInCtx(rf.RefDocs[i], c, func(f *TableStmt, _ *PackageSchemaAST) error {
					if f.Abstract {
						return ErrReferenceToAbstractTable(rf.RefDocs[i].String())
					}
					return nil
				}); err != nil {
					c.stmtErr(&rf.RefDocs[i].Pos, err)
					continue
				}
			}
		}
		if item.NestedTable != nil {
			nestedTable := &item.NestedTable.Table
			analyseFields(nestedTable.Items, c, true)
		}
		if item.Constraint != nil {
			if item.Constraint.ConstraintName != "" {
				cname := string(item.Constraint.ConstraintName)
				if _, ok := constraintNames[cname]; ok {
					c.stmtErr(&item.Constraint.Pos, ErrRedefined(cname))
					continue
				}
				constraintNames[cname] = true
			}
			if item.Constraint.UniqueField != nil {
				if ok := lookupField(items, item.Constraint.UniqueField.Field); !ok {
					c.stmtErr(&item.Constraint.Pos, ErrUndefinedField(string(item.Constraint.UniqueField.Field)))
					continue
				}
			} else if item.Constraint.Unique != nil {
				for _, field := range item.Constraint.Unique.Fields {
					for _, f := range fieldsInUniques {
						if f == field {
							c.stmtErr(&item.Constraint.Pos, ErrFieldAlreadyInUnique(string(field)))
							continue
						}
					}
					if ok := lookupField(items, field); !ok {
						c.stmtErr(&item.Constraint.Pos, ErrUndefinedField(string(field)))
						continue
					}
					fieldsInUniques = append(fieldsInUniques, field)
				}
			}
		}
	}
}

type tableNode struct {
	pkg   *PackageSchemaAST
	table *TableStmt
}

func getTableInheritanceChain(table *TableStmt, c *iterateCtx) (chain []tableNode, err error) {
	chain = make([]tableNode, 0)
	refCycle := func(node tableNode) bool {
		for i := range chain {
			if (chain[i].pkg == node.pkg) && (chain[i].table.Name == node.table.Name) {
				return true
			}
		}
		return false
	}
	var vf func(t *TableStmt) error
	vf = func(t *TableStmt) error {
		if t.Inherits != nil {
			inherited := *t.Inherits
			t, pkg, err := lookupInCtx[*TableStmt](inherited, c)
			if err != nil {
				return err
			}
			if t != nil {
				node := tableNode{pkg: pkg, table: t}
				if refCycle(node) {
					return ErrCircularReferenceInInherits
				}
				chain = append(chain, node)
				return vf(t)
			}
		}
		return nil
	}
	err = vf(table)
	return
}

func getTableTypeKind(table *TableStmt, pkg *PackageSchemaAST, c *iterateCtx) (kind appdef.TypeKind, singleton bool, err error) {

	kind = appdef.TypeKind_null
	check := func(node tableNode) {
		if node.pkg.QualifiedPackageName == appdef.SysPackage {
			if node.table.Name == nameCDOC {
				kind = appdef.TypeKind_CDoc
			}
			if node.table.Name == nameODOC {
				kind = appdef.TypeKind_ODoc
			}
			if node.table.Name == nameWDOC {
				kind = appdef.TypeKind_WDoc
			}
			if node.table.Name == nameCRecord {
				kind = appdef.TypeKind_CRecord
			}
			if node.table.Name == nameORecord {
				kind = appdef.TypeKind_ORecord
			}
			if node.table.Name == nameWRecord {
				kind = appdef.TypeKind_WRecord
			}
			if (node.table.Name == nameSingletonDeprecated) || (node.table.Name == nameCSingleton) {
				kind = appdef.TypeKind_CDoc
				singleton = true
			}
			if node.table.Name == nameWSingleton {
				kind = appdef.TypeKind_WDoc
				singleton = true
			}
		}
	}

	check(tableNode{pkg: pkg, table: table})
	if kind != appdef.TypeKind_null {
		return kind, singleton, nil
	}

	chain, e := getTableInheritanceChain(table, c)
	if e != nil {
		return appdef.TypeKind_null, false, e
	}
	for _, t := range chain {
		check(t)
		if kind != appdef.TypeKind_null {
			return kind, singleton, nil
		}
	}
	return appdef.TypeKind_null, false, ErrUndefinedTableKind
}

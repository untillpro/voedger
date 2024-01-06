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
				if f.Field != nil && f.Field.Name == Ident(column) {
					return nil
				}
				if f.RefField != nil && f.RefField.Name == Ident(column) {
					return nil
				}
				if f.NestedTable != nil && f.NestedTable.Name == Ident(column) {
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
	if u.TableName != nil {
		n := DefQName{Package: u.Package, Name: *u.TableName}
		err := resolveInCtx(n, c, func(f *TableStmt, _ *PackageSchemaAST) error {
			if f.Abstract {
				return ErrUseOfAbstractTable(n.String())
			}
			return nil
		})
		if err != nil {
			c.stmtErr(&u.Pos, err)
		}
	} else {
		if u.Package != "" {
			_, e := findPackage(u.Package, c)
			if e != nil {
				c.stmtErr(&u.Pos, e)
				return
			}

		}
	}
}

func analyseUseWorkspace(u *UseWorkspaceStmt, c *iterateCtx) {
	resolveFunc := func(f *WorkspaceStmt, _ *PackageSchemaAST) error {
		if f.Abstract {
			return ErrUseOfAbstractWorkspace(string(u.Workspace))
		}
		return nil
	}
	err := resolveInCtx(DefQName{Package: Ident(c.pkg.Name), Name: u.Workspace}, c, resolveFunc)
	if err != nil {
		c.stmtErr(&u.Pos, err)
	}
}

func analyseAlterWorkspace(u *AlterWorkspaceStmt, c *iterateCtx) {
	resolveFunc := func(w *WorkspaceStmt, schema *PackageSchemaAST) error {
		if !w.Alterable && schema != c.pkg {
			return ErrWorkspaceIsNotAlterable(u.Name.String())
		}
		return nil
	}
	err := resolveInCtx(u.Name, c, resolveFunc)
	if err != nil {
		c.stmtErr(&u.Pos, err)
	}
}

func analyseStorage(u *StorageStmt, c *iterateCtx) {
	if c.pkg.QualifiedPackageName != appdef.SysPackage {
		c.stmtErr(&u.Pos, ErrStorageDeclaredOnlyInSys)
	}
}

func analyseLimit(u *LimitStmt, c *iterateCtx) {
	err := resolveInCtx(u.RateName, c, func(l *RateStmt, schema *PackageSchemaAST) error { return nil })
	if err != nil {
		c.stmtErr(&u.Pos, err)
	}

	if u.Action.AllCommandsWithTag != nil {
		err = resolveInCtx(*u.Action.AllCommandsWithTag, c, func(t *TagStmt, schema *PackageSchemaAST) error { return nil })
	} else if u.Action.AllQueriesWithTag != nil {
		err = resolveInCtx(*u.Action.AllQueriesWithTag, c, func(t *TagStmt, schema *PackageSchemaAST) error { return nil })
	} else if u.Action.AllWorkspacesWithTag != nil {
		err = resolveInCtx(*u.Action.AllWorkspacesWithTag, c, func(t *TagStmt, schema *PackageSchemaAST) error { return nil })
	} else if u.Action.Command != nil {
		err = resolveInCtx(*u.Action.Command, c, func(t *CommandStmt, schema *PackageSchemaAST) error { return nil })
	} else if u.Action.Query != nil {
		err = resolveInCtx(*u.Action.Query, c, func(t *QueryStmt, schema *PackageSchemaAST) error { return nil })
	} else if u.Action.Workspace != nil {
		err = resolveInCtx(*u.Action.Workspace, c, func(t *WorkspaceStmt, schema *PackageSchemaAST) error { return nil })
	}
	if err != nil {
		c.stmtErr(&u.Pos, err)
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
			if _, ok := fields[string(f.Name)]; ok {
				c.stmtErr(&f.Pos, ErrRedefined(string(f.Name)))
			} else {
				fields[string(f.Name)] = i
			}
		} else if fe.RefField != nil {
			rf := fe.RefField
			if _, ok := fields[string(rf.Name)]; ok {
				c.stmtErr(&rf.Pos, ErrRedefined(string(rf.Name)))
			} else {
				fields[string(rf.Name)] = i
			}
			for i := range rf.RefDocs {
				err := resolveInCtx(rf.RefDocs[i], c, func(f *TableStmt, _ *PackageSchemaAST) error {
					if f.Abstract {
						return ErrReferenceToAbstractTable(rf.RefDocs[i].String())
					}
					return nil
				})
				if err != nil {
					c.stmtErr(&rf.Pos, err)
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
		index, ok := fields[string(pkf)]
		if !ok {
			c.stmtErr(&view.pkRef.Pos, ErrUndefinedField(string(pkf)))
		}
		if view.Items[index].Field != nil {
			if view.Items[index].Field.Type.Varchar != nil {
				c.stmtErr(&view.pkRef.Pos, ErrViewFieldVarchar(string(pkf)))
			}
			if view.Items[index].Field.Type.Bytes != nil {
				c.stmtErr(&view.pkRef.Pos, ErrViewFieldBytes(string(pkf)))
			}
		}
	}

	for ccIndex, ccf := range view.pkRef.ClusteringColumnsFields {
		fieldIndex, ok := fields[string(ccf)]
		last := ccIndex == len(view.pkRef.ClusteringColumnsFields)-1
		if !ok {
			c.stmtErr(&view.pkRef.Pos, ErrUndefinedField(string(ccf)))
		}
		if view.Items[fieldIndex].Field != nil {
			if view.Items[fieldIndex].Field.Type.Varchar != nil && !last {
				c.stmtErr(&view.pkRef.Pos, ErrVarcharFieldInCC(string(ccf)))
			}
			if view.Items[fieldIndex].Field.Type.Bytes != nil && !last {
				c.stmtErr(&view.pkRef.Pos, ErrBytesFieldInCC(string(ccf)))
			}
		}
	}

	// ResultOf
	err := resolveInCtx(view.ResultOf, c, func(f *ProjectorStmt, _ *PackageSchemaAST) error {
		var intentForView *ProjectorStorage
		for i := 0; i < len(f.Intents) && intentForView == nil; i++ {
			var isView bool
			intent := f.Intents[i]
			if err := resolveInCtx(intent.Storage, c, func(storage *StorageStmt, _ *PackageSchemaAST) error {
				isView = isView || storage.EntityView
				return nil
			}); err != nil {
				c.stmtErr(&view.Pos, err)
			}

			if isView {
				for _, entity := range intent.Entities {
					if entity.Name == view.Name && (entity.Package == Ident(c.pkg.Name) || entity.Package == Ident("")) {
						intentForView = &f.Intents[i]
						break
					}
				}
			}
		}
		if intentForView == nil {
			return ErrProjectorDoesNotDeclareViewIntent(f.GetName(), view.GetName())
		}
		return nil
	})
	if err != nil {
		c.stmtErr(&view.Pos, err)
	}
}

func analyzeCommand(cmd *CommandStmt, c *iterateCtx) {

	resolve := func(qn DefQName) {
		typ, _, err := lookupInCtx[*TypeStmt](qn, c)
		if typ == nil && err == nil {
			tbl, _, err := lookupInCtx[*TableStmt](qn, c)
			if tbl == nil && err == nil {
				c.stmtErr(&cmd.Pos, ErrUndefinedTypeOrTable(qn))
			}
		}
		if err != nil {
			c.stmtErr(&cmd.Pos, err)
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
				c.stmtErr(&cmd.Pos, ErrUndefinedTypeOrTable(*cmd.UnloggedParam.Def))
			}
		}
		if err != nil {
			c.stmtErr(&cmd.Pos, err)
		}
	}
	if cmd.Returns != nil && cmd.Returns.Def != nil {
		resolve(*cmd.Returns.Def)
	}
	analyseWith(&cmd.With, cmd, c)
}

func analyzeQuery(query *QueryStmt, c *iterateCtx) {
	if query.Param != nil && query.Param.Def != nil {
		if err := resolveInCtx(*query.Param.Def, c, func(*TypeStmt, *PackageSchemaAST) error { return nil }); err != nil {
			c.stmtErr(&query.Pos, err)
		}

	}
	if query.Returns.Def != nil {
		if err := resolveInCtx(*query.Returns.Def, c, func(*TypeStmt, *PackageSchemaAST) error { return nil }); err != nil {
			c.stmtErr(&query.Pos, err)
		}
	}
	analyseWith(&query.With, query, c)

}

func analyseProjector(v *ProjectorStmt, c *iterateCtx) {
	for _, trigger := range v.Triggers {
		for _, qname := range trigger.QNames {
			if len(trigger.TableActions) > 0 {
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
					return nil
				}
				if err := resolveInCtx(qname, c, resolveFunc); err != nil {
					c.stmtErr(&v.Pos, err)
				}
			} else { // Command
				if trigger.ExecuteAction.WithParam {
					cmd, _, err := lookupInCtx[*TypeStmt](qname, c)
					if err != nil { // type?
						c.stmtErr(&v.Pos, err)
						continue
					}
					if cmd == nil { // ODoc?
						odoc, _, err := lookupInCtx[*TableStmt](qname, c)
						if err != nil {
							c.stmtErr(&v.Pos, err)
							continue
						}
						if odoc == nil || odoc.tableTypeKind != appdef.TypeKind_ODoc {
							c.stmtErr(&v.Pos, ErrUndefinedTypeOrOdoc(qname))
							continue
						}
					}
				} else {
					cmd, _, err := lookupInCtx[*CommandStmt](qname, c)
					if err != nil {
						c.stmtErr(&v.Pos, err)
						continue
					}
					if cmd == nil {
						c.stmtErr(&v.Pos, ErrUndefinedCommand(qname))
						continue
					}
				}
			}
		}
	}

	checkEntity := func(key ProjectorStorage, f *StorageStmt) error {
		if f.EntityRecord {
			if len(key.Entities) == 0 {
				return ErrStorageRequiresEntity(key.Storage.String())
			}
			for _, entity := range key.Entities {
				resolveFunc := func(f *TableStmt, _ *PackageSchemaAST) error {
					if f.Abstract {
						return ErrAbstractTableNotAlowedInProjectors(entity.String())
					}
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
				if err2 := resolveInCtx(entity, c, func(*ViewStmt, *PackageSchemaAST) error { return nil }); err2 != nil {
					return err2
				}
			}
		}
		return nil
	}

	for _, key := range v.State {
		if err := resolveInCtx(key.Storage, c, func(f *StorageStmt, _ *PackageSchemaAST) error {
			if e := checkEntity(key, f); e != nil {
				return e
			}
			read := false
			for _, op := range f.Ops {
				if op.Get || op.GetBatch || op.Read {
					for _, sc := range op.Scope {
						if sc.Projectors {
							read = true
							break
						}
					}
				}
			}
			if !read {
				return ErrStorageNotInProjectorState(key.Storage.String())
			}
			return nil
		}); err != nil {
			c.stmtErr(&v.Pos, err)
		}
	}

	for _, key := range v.Intents {
		if err := resolveInCtx(key.Storage, c, func(f *StorageStmt, _ *PackageSchemaAST) error {
			if e := checkEntity(key, f); e != nil {
				return e
			}
			read := false
			for _, op := range f.Ops {
				if op.Insert || op.Update {
					for _, sc := range op.Scope {
						if sc.Projectors {
							read = true
							break
						}
					}
				}
			}
			if !read {
				return ErrStorageNotInProjectorIntents(key.Storage.String())
			}
			return nil
		}); err != nil {
			c.stmtErr(&v.Pos, err)
		}
	}

}

// Note: function may update with argument
func analyseWith(with *[]WithItem, statement IStatement, c *iterateCtx) {
	var comment *WithItem

	for i := range *with {
		item := &(*with)[i]
		if item.Comment != nil {
			comment = item
		} else if item.Rate != nil {
			if err := resolveInCtx(*item.Rate, c, func(*RateStmt, *PackageSchemaAST) error { return nil }); err != nil {
				c.stmtErr(statement.GetPos(), err)
			}
		}
		for j := range item.Tags {
			tag := item.Tags[j]
			if err := resolveInCtx(tag, c, func(*TagStmt, *PackageSchemaAST) error { return nil }); err != nil {
				c.stmtErr(statement.GetPos(), err)
			}
		}
	}

	if comment != nil {
		statement.SetComments(strings.Split(*comment.Comment, "\n"))
	}
}

func preAnalyseTable(v *TableStmt, c *iterateCtx) {
	var err error
	v.tableTypeKind, v.singletone, err = getTableTypeKind(v, c.pkg, c)
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
			c.stmtErr(&v.Pos, err)
		}

	}
}

func analyseType(v *TypeStmt, c *iterateCtx) {
	for _, i := range v.Items {
		if i.NestedTable != nil {
			c.stmtErr(&v.Pos, ErrNestedTablesNotSupportedInTypes)
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
			c.stmtErr(&v.Pos, err)
		}
	}
	if v.Descriptor != nil {
		if v.Abstract {
			c.stmtErr(&v.Pos, ErrAbstractWorkspaceDescriptor)
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
		} else if item.Field != nil && item.Field.Type.Def != nil {
			tbl, _, err := lookupInCtx[*TableStmt](*item.Field.Type.Def, c)
			if err != nil {
				c.stmtErr(&item.Field.Pos, err)
				continue
			}
			if tbl == nil {
				c.stmtErr(&item.Field.Pos, ErrUndefinedTable(*item.Field.Type.Def))
				continue
			}
			nestedTable = tbl
			pos = &item.Field.Pos
		}

		if nestedTable != nil {
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
				nestedTable.tableTypeKind, nestedTable.singletone, err = getTableTypeKind(nestedTable, c.pkg, c)
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
				c.stmtErr(&item.FieldSet.Pos, err)
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
					_, err := regexp.Compile(*field.CheckRegexp)
					if err != nil {
						c.stmtErr(&field.Pos, ErrCheckRegexpErr(err))
					}
				} else {
					c.stmtErr(&field.Pos, ErrRegexpCheckOnlyForVarcharField)
				}
			}
			if field.Type.DataType != nil {
				if field.Type.DataType.Varchar != nil && field.Type.DataType.Varchar.MaxLen != nil {
					if *field.Type.DataType.Varchar.MaxLen > uint64(appdef.MaxFieldLength) {
						c.stmtErr(&field.Pos, ErrMaxFieldLengthTooLarge)
					}
				}
				if field.Type.DataType.Bytes != nil && field.Type.DataType.Bytes.MaxLen != nil {
					if *field.Type.DataType.Bytes.MaxLen > uint64(appdef.MaxFieldLength) {
						c.stmtErr(&field.Pos, ErrMaxFieldLengthTooLarge)
					}
				}
			} else {
				if !isTable { // analysing a TYPE
					typ, _, err := lookupInCtx[*TypeStmt](*field.Type.Def, c)
					if err != nil { // type?
						c.stmtErr(&field.Pos, err)
						continue
					}
					if typ == nil {
						c.stmtErr(&field.Pos, ErrUndefinedType(*field.Type.Def))
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
					c.stmtErr(&rf.Pos, err)
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

func getTableTypeKind(table *TableStmt, pkg *PackageSchemaAST, c *iterateCtx) (kind appdef.TypeKind, singletone bool, err error) {

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
			if node.table.Name == nameSingleton {
				kind = appdef.TypeKind_CDoc
				singletone = true
			}
		}
	}

	check(tableNode{pkg: pkg, table: table})
	if kind != appdef.TypeKind_null {
		return kind, singletone, nil
	}

	chain, e := getTableInheritanceChain(table, c)
	if e != nil {
		return appdef.TypeKind_null, false, e
	}
	for _, t := range chain {
		check(t)
		if kind != appdef.TypeKind_null {
			return kind, singletone, nil
		}
	}
	return appdef.TypeKind_null, false, ErrUndefinedTableKind
}

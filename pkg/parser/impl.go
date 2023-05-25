/*
* Copyright (c) 2023-present unTill Pro, Ltd.
* @author Michael Saigachenko
 */
package parser

import (
	"embed"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/voedger/voedger/pkg/appdef"
)

func parseImpl(fileName string, content string) (*SchemaAST, error) {
	var basicLexer = lexer.MustSimple([]lexer.SimpleRule{

		{Name: "Comment", Pattern: `--.*`},
		{Name: "Array", Pattern: `\[\]`},
		{Name: "Float", Pattern: `[-+]?\d+\.\d+`},
		{Name: "Int", Pattern: `[-+]?\d+`},
		{Name: "Operators", Pattern: `<>|!=|<=|>=|[-+*/%,()=<>]`}, //( '<>' | '<=' | '>=' | '=' | '<' | '>' | '!=' )"
		{Name: "Punct", Pattern: `[;\[\].]`},
		{Name: "Keywords", Pattern: `ON|AND|OR`},
		{Name: "DEFAULTNEXTVAL", Pattern: `DEFAULT[ \r\n\t]+NEXTVAL`},
		{Name: "NOTNULL", Pattern: `NOT[ \r\n\t]+NULL`},
		{Name: "EXTENSIONENGINE", Pattern: `EXTENSION[ \r\n\t]+ENGINE`},
		{Name: "PRIMARYKEY", Pattern: `PRIMARY[ \r\n\t]+KEY`},
		{Name: "String", Pattern: `("(\\"|[^"])*")|('(\\'|[^'])*')`},
		{Name: "Ident", Pattern: `[a-zA-Z_]\w*`},
		{Name: "Whitespace", Pattern: `[ \r\n\t]+`},
	})

	parser := participle.MustBuild[SchemaAST](
		participle.Lexer(basicLexer),
		participle.Elide("Whitespace", "Comment"),
		participle.Unquote("String"),
	)
	return parser.ParseString(fileName, content)
}

func mergeSchemas(mergeFrom, mergeTo *SchemaAST) {
	// imports
	mergeTo.Imports = append(mergeTo.Imports, mergeFrom.Imports...)

	// statements
	mergeTo.Statements = append(mergeTo.Statements, mergeFrom.Statements...)
}

func parseFSImpl(fs IReadFS, dir string) ([]*FileSchemaAST, error) {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	schemas := make([]*FileSchemaAST, 0)
	for _, entry := range entries {
		if strings.ToLower(filepath.Ext(entry.Name())) == ".sql" {
			var fpath string
			if _, ok := fs.(embed.FS); ok {
				fpath = fmt.Sprintf("%s/%s", dir, entry.Name()) // The path separator is a forward slash, even on Windows systems
			} else {
				fpath = filepath.Join(dir, entry.Name())
			}
			bytes, err := fs.ReadFile(fpath)
			if err != nil {
				return nil, err
			}
			schema, err := parseImpl(entry.Name(), string(bytes))
			if err != nil {
				return nil, err
			}
			schemas = append(schemas, &FileSchemaAST{
				FileName: entry.Name(),
				Ast:      schema,
			})
		}
	}
	if len(schemas) == 0 {
		return nil, ErrDirContainsNoSchemaFiles
	}
	return schemas, nil
}

func mergeFileSchemaASTsImpl(qualifiedPackageName string, asts []*FileSchemaAST) (*PackageSchemaAST, error) {
	if len(asts) == 0 {
		return nil, nil
	}
	headAst := asts[0].Ast
	// TODO: do we need to check that last element in qualifiedPackageName path corresponds to f.Ast.Package?
	for i := 1; i < len(asts); i++ {
		f := asts[i]
		if f.Ast.Package != headAst.Package {
			return nil, ErrUnexpectedSchema(f.FileName, f.Ast.Package, headAst.Package)
		}
		mergeSchemas(f.Ast, headAst)
	}

	errs := make([]error, 0)
	errs = analyseDuplicateNames(headAst, errs)
	errs = analyseViews(headAst, errs)
	cleanupComments(headAst)
	cleanupImports(headAst)
	// TODO: unable to specify different base tables (CDOC, WDOC, ...) in the table inheritace chain
	// TODO: Type cannot have nested tables

	return &PackageSchemaAST{
		QualifiedPackageName: qualifiedPackageName,
		Ast:                  headAst,
	}, errors.Join(errs...)
}

func analyseViews(schema *SchemaAST, errs []error) []error {
	iterate(schema, func(stmt interface{}) {
		if view, ok := stmt.(*ViewStmt); ok {
			view.pkRef = nil
			fields := make(map[string]int)
			for i := range view.Fields {
				fe := view.Fields[i]
				if fe.PrimaryKey != nil {
					if view.pkRef != nil {
						errs = append(errs, errorAt(ErrPrimaryKeyRedeclared, &fe.Pos))
					} else {
						view.pkRef = fe.PrimaryKey
					}
				}
				if fe.Field != nil {
					if _, ok := fields[fe.Field.Name]; ok {
						errs = append(errs, errorAt(ErrRedeclared(fe.Field.Name), &fe.Pos))
					} else {
						fields[fe.Field.Name] = i
					}
				}
			}
			if view.pkRef == nil {
				errs = append(errs, errorAt(ErrPrimaryKeyNotDeclared, &view.Pos))
			}
		}
	})
	return errs
}

func analyseDuplicateNames(schema *SchemaAST, errs []error) []error {
	namedIndex := make(map[string]interface{})

	iterate(schema, func(stmt interface{}) {
		if named, ok := stmt.(INamedStatement); ok {
			name := named.GetName()
			if name == "" {
				_, isProjector := stmt.(*ProjectorStmt)
				if isProjector {
					return // skip anonymous projectors
				}
			}
			if _, ok := namedIndex[name]; ok {
				s := stmt.(IStatement)
				errs = append(errs, errorAt(ErrRedeclared(name), s.GetPos()))
			} else {
				namedIndex[name] = stmt
			}
		}
	})
	return errs
}

func cleanupComments(schema *SchemaAST) {
	iterate(schema, func(stmt interface{}) {
		if s, ok := stmt.(IStatement); ok {
			comments := *s.GetComments()
			for i := 0; i < len(comments); i++ {
				comments[i], _ = strings.CutPrefix(comments[i], "--")
				comments[i] = strings.TrimSpace(comments[i])
			}
		}
	})
}

func cleanupImports(schema *SchemaAST) {
	for i := 0; i < len(schema.Imports); i++ {
		imp := &schema.Imports[i]
		imp.Name = strings.Trim(imp.Name, "\"")
	}
}

func mergePackageSchemasImpl(packages []*PackageSchemaAST) (map[string]*PackageSchemaAST, error) {
	pkgmap := make(map[string]*PackageSchemaAST)
	for _, p := range packages {
		if _, ok := pkgmap[p.QualifiedPackageName]; ok {
			return nil, ErrPackageRedeclared(p.QualifiedPackageName)
		}
		pkgmap[p.QualifiedPackageName] = p
	}

	c := basicContext{
		pkg:    nil,
		pkgmap: pkgmap,
		errs:   make([]error, 0),
	}

	for _, p := range packages {
		c.pkg = p
		analyseRefs(&c)
	}
	return pkgmap, errors.Join(c.errs...)
}

type basicContext struct {
	pkg    *PackageSchemaAST
	pkgmap map[string]*PackageSchemaAST
	pos    *lexer.Position
	errs   []error
}

func analyzeWithRefs(c *basicContext, with []WithItem) {
	for i := range with {
		wi := &with[i]
		if wi.Comment != nil {
			resolve(*wi.Comment, c, func(f *CommentStmt) error { return nil })
		} else if wi.Rate != nil {
			resolve(*wi.Rate, c, func(f *RateStmt) error { return nil })
		}
		for j := range wi.Tags {
			tag := wi.Tags[j]
			resolve(tag, c, func(f *TagStmt) error { return nil })
		}
	}
}

func analyseRefs(c *basicContext) {
	iterate(c.pkg.Ast, func(stmt interface{}) {
		switch v := stmt.(type) {
		case *CommandStmt:
			c.pos = &v.Pos
			analyzeWithRefs(c, v.With)
		case *QueryStmt:
			c.pos = &v.Pos
			analyzeWithRefs(c, v.With)
		case *ProjectorStmt:
			c.pos = &v.Pos
			// Check targets
			for _, target := range v.Triggers {
				if v.On.Activate || v.On.Deactivate || v.On.Insert || v.On.Update {
					resolve(target, c, func(f *TableStmt) error { return nil })
				} else if v.On.Command {
					resolve(target, c, func(f *CommandStmt) error { return nil })
				} else if v.On.CommandArgument {
					resolve(target, c, func(f *TypeStmt) error { return nil })
				}
			}
		case *TableStmt:
			c.pos = &v.Pos
			analyzeWithRefs(c, v.With)
			if v.Inherits != nil {
				resolve(*v.Inherits, c, func(f *TableStmt) error { return nil })
			}
			for _, of := range v.Of {
				resolve(of, c, func(f *TypeStmt) error { return nil })
			}
		}
	})
}

type defBuildContext struct {
	defBuilder appdef.IDefBuilder
	qname      appdef.QName
	kind       appdef.DefKind
	names      map[string]bool
}

func (c *defBuildContext) checkName(name string, pos *lexer.Position) error {
	if _, ok := c.names[name]; ok {
		return errorAt(ErrRedeclared(name), pos)
	}
	c.names[name] = true
	return nil
}

type buildContext struct {
	basicContext
	builder appdef.IAppDefBuilder
	defs    []defBuildContext
}

func (c *buildContext) newSchema(schema *PackageSchemaAST) {
	c.pkg = schema
	c.defs = make([]defBuildContext, 0)
}

func (c *buildContext) pushDef(name string, kind appdef.DefKind) {
	qname := appdef.NewQName(c.pkg.Ast.Package, name)
	c.defs = append(c.defs, defBuildContext{
		defBuilder: c.builder.AddStruct(qname, kind),
		kind:       kind,
		qname:      qname,
		names:      make(map[string]bool),
	})
}

func (c *buildContext) popDef() {
	c.defs = c.defs[:len(c.defs)-1]
}

func (c *buildContext) defCtx() *defBuildContext {
	return &c.defs[len(c.defs)-1]
}

func newBuildContext(packages map[string]*PackageSchemaAST, builder appdef.IAppDefBuilder) buildContext {
	return buildContext{
		basicContext: basicContext{
			pkg:    nil,
			pkgmap: packages,
			errs:   make([]error, 0),
		},
		builder: builder,
	}
}

func buildAppDefs(packages map[string]*PackageSchemaAST, builder appdef.IAppDefBuilder) error {
	ctx := newBuildContext(packages, builder)

	if err := buildTypes(&ctx); err != nil {
		return err
	}
	if err := buildTables(&ctx); err != nil {
		return err
	}
	if err := buildViews(&ctx); err != nil {
		return err
	}
	return nil
}

func buildTypes(ctx *buildContext) error {
	for _, schema := range ctx.pkgmap {
		iterateStmt(schema.Ast, func(typ *TypeStmt) {
			ctx.newSchema(schema)
			ctx.pushDef(typ.Name, appdef.DefKind_Object)
			addFieldsOf(typ.Of, ctx)
			addTableItems(typ.Items, ctx)
			ctx.popDef()
		})
	}
	return nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func buildViews(ctx *buildContext) error {
	for _, schema := range ctx.pkgmap {
		iterateStmt(schema.Ast, func(view *ViewStmt) {
			ctx.newSchema(schema)

			qname := appdef.NewQName(ctx.pkg.Ast.Package, view.Name)
			vb := ctx.builder.AddView(qname)
			for i := range view.Fields {
				f := &view.Fields[i]
				if f.Field != nil {
					datakind := viewFieldDataKind(f.Field)
					if contains(view.pkRef.ClusteringColumnsFields, f.Field.Name) {
						vb.AddClustColumn(f.Field.Name, datakind)
					} else if contains(view.pkRef.PartitionKeyFields, f.Field.Name) {
						vb.AddPartField(f.Field.Name, datakind)
					} else {
						vb.AddValueField(f.Field.Name, datakind, f.Field.NotNull)
					}
				}
			}
		})
	}
	return nil
}

func fillTable(ctx *buildContext, table *TableStmt) {
	if table.Inherits != nil {
		resolve(*table.Inherits, &ctx.basicContext, func(t *TableStmt) error {
			fillTable(ctx, t)
			return nil
		})
	}
	addFieldsOf(table.Of, ctx)
	addTableItems(table.Items, ctx)
}

func buildTables(ctx *buildContext) error {
	for _, schema := range ctx.pkgmap {
		iterateStmt(schema.Ast, func(table *TableStmt) {
			ctx.newSchema(schema)
			if isPredefinedSysTable(table, ctx) {
				return
			}
			tableType, singletone := getTableDefKind(table, ctx)
			if tableType == appdef.DefKind_null {
				ctx.errs = append(ctx.errs, errorAt(ErrUndefinedTableKind, &table.Pos))
			} else {
				ctx.pushDef(table.Name, tableType)
				fillTable(ctx, table)
				if singletone {
					ctx.defCtx().defBuilder.SetSingleton()
				}
				ctx.popDef()
			}
		})
	}
	return errors.Join(ctx.errs...)
}

func addTableItems(items []TableItemExpr, ctx *buildContext) {
	for _, item := range items {
		if item.Field != nil {
			sysDataKind := getTypeDataKind(*item.Field.Type) // TODO: handle 'reference ...'
			if sysDataKind != appdef.DataKind_null {
				if item.Field.Type.IsArray {
					ctx.errs = append(ctx.errs, errorAt(ErrArrayFieldsNotSupportedHere, &item.Field.Pos))
					continue
				}
				if err := ctx.defCtx().checkName(item.Field.Name, &item.Field.Pos); err != nil {
					ctx.errs = append(ctx.errs, err)
					continue
				}
				if item.Field.Verifiable {
					// TODO: Support different verification kindsbuilder, &c
					ctx.defCtx().defBuilder.AddVerifiedField(item.Field.Name, sysDataKind, item.Field.NotNull, appdef.VerificationKind_EMail)
				} else {
					ctx.defCtx().defBuilder.AddField(item.Field.Name, sysDataKind, item.Field.NotNull)
				}
			} else {
				ctx.errs = append(ctx.errs, errorAt(ErrTypeNotSupported(item.Field.Type.String()), &item.Field.Pos))
			}
		} else if item.Constraint != nil {
			// TODO: constraint checks, e.g. same field cannot be used twice
			if item.Constraint.Unique != nil {
				name := item.Constraint.ConstraintName
				if name == "" {
					name = genUniqueName(ctx.defCtx().qname.Entity(), ctx.defCtx().defBuilder)
				}
				if err := ctx.defCtx().checkName(name, &item.Constraint.Pos); err != nil {
					ctx.errs = append(ctx.errs, err)
					continue
				}
				ctx.defCtx().defBuilder.AddUnique(name, item.Constraint.Unique.Fields)
				//} else if item.Constraint.Check != nil {
				// TODO: implement Table Check Constraint
			}
		} else if item.Table != nil {
			// Add nested table
			kind, singletone := getTableDefKind(item.Table, ctx)
			if kind != appdef.DefKind_null || singletone {
				ctx.errs = append(ctx.errs, ErrNestedTableCannotBeDocument)
				continue
			}

			containerName := item.Table.Name // TODO: implement AS container_name
			if err := ctx.defCtx().checkName(containerName, &item.Table.Pos); err != nil {
				ctx.errs = append(ctx.errs, err)
				continue
			}
			tk := getNestedTableKind(ctx.defs[0].kind)
			ctx.pushDef(item.Table.Name, tk) // TODO: analyze for duplicates in the QNames of nested tables
			addFieldsOf(item.Table.Of, ctx)
			addTableItems(item.Table.Items, ctx)
			ctx.defCtx().defBuilder.AddContainer(containerName, ctx.defCtx().qname, 0, maxNestedTableContainerOccurences)
			ctx.popDef()

		}
	}
}

func addFieldsOf(types []DefQName, ctx *buildContext) {
	for _, of := range types {
		resolve(of, &ctx.basicContext, func(t *TypeStmt) error {
			addFieldsOf(t.Of, ctx)
			addTableItems(t.Items, ctx)
			return nil
		})
	}
}

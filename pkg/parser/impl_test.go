/*
* Copyright (c) 2023-present unTill Pro, Ltd.
* @author Michael Saigachenko
 */
package parser

import (
	"embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/appdef"
)

//go:embed sql_example_app/pmain/*.sql
var fsMain embed.FS

//go:embed sql_example_app/airsbp/*.sql
var fsAir embed.FS

//go:embed sql_example_app/untill/*.sql
var fsUntill embed.FS

//go:embed sql_example_syspkg/*.sql
var sfs embed.FS

//go:embed sql_example_app/vrestaurant/*.sql
var fsvRestaurant embed.FS

//_go:embed example_app/expectedParsed.schema
//var expectedParsedExampledSchemaStr string

func getSysPackageAST() *PackageSchemaAST {
	pkgSys, err := ParsePackageDir(appdef.SysPackage, sfs, "sql_example_syspkg")
	if err != nil {
		panic(err)
	}
	return pkgSys
}

func Test_BasicUsage(t *testing.T) {

	require := require.New(t)
	mainPkgAST, err := ParsePackageDir("github.com/untillpro/main", fsMain, "sql_example_app/pmain")
	require.NoError(err)

	airPkgAST, err := ParsePackageDir("github.com/untillpro/airsbp", fsAir, "sql_example_app/airsbp")
	require.NoError(err)

	untillPkgAST, err := ParsePackageDir("github.com/untillpro/untill", fsUntill, "sql_example_app/untill")
	require.NoError(err)

	// := repr.String(pkgExample, repr.Indent(" "), repr.IgnorePrivate())
	//fmt.Println(parsedSchemaStr)

	packages, err := MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		mainPkgAST,
		airPkgAST,
		untillPkgAST,
	})
	require.NoError(err)

	builder := appdef.New()
	err = BuildAppDefs(packages, builder)
	require.NoError(err)

	// table
	cdoc := builder.CDoc(appdef.NewQName("main", "TablePlan"))
	require.NotNil(cdoc)
	require.Equal(appdef.TypeKind_CDoc, cdoc.Kind())
	require.Equal(appdef.DataKind_int32, cdoc.Field("FState").DataKind())
	require.Equal("Backoffice Table", cdoc.Comment())

	// TODO: sf := cdoc.Field("CheckedField").(appdef.IStringField)
	// TODO: require.Equal(uint16(8), sf.Restricts().MaxLen())
	// TODO: require.NotNil(sf.Restricts().Pattern())

	// container of the table
	container := cdoc.Container("TableItems")
	require.Equal("TableItems", container.Name())
	require.Equal(appdef.NewQName("main", "TablePlanItem"), container.QName())
	require.Equal(appdef.Occurs(0), container.MinOccurs())
	require.Equal(appdef.Occurs(maxNestedTableContainerOccurrences), container.MaxOccurs())
	require.Equal(appdef.TypeKind_CRecord, container.Type().Kind())
	require.Equal(2+5 /*system fields*/, container.Type().(appdef.IFields).FieldCount())
	require.Equal(appdef.DataKind_int32, container.Type().(appdef.IFields).Field("TableNo").DataKind())
	require.Equal(appdef.DataKind_int32, container.Type().(appdef.IFields).Field("Chairs").DataKind())

	// child table
	crec := builder.CRecord(appdef.NewQName("main", "TablePlanItem"))
	require.NotNil(crec)
	require.Equal(appdef.TypeKind_CRecord, crec.Kind())
	require.Equal(appdef.DataKind_int32, crec.Field("TableNo").DataKind())

	crec = builder.CRecord(appdef.NewQName("main", "NestedWithName"))
	require.NotNil(crec)
	require.True(crec.Abstract())
	field := crec.Field("ItemName")
	require.NotNil(field)
	require.Equal("Field is added to any table inherited from NestedWithName\nThe current comment is also added to scheme for this field", field.Comment())

	// multinine comments
	singleton := builder.CDoc(appdef.NewQName("main", "SubscriptionProfile"))
	require.Equal("Singletons are always CDOC. Error is thrown on attempt to declare it as WDOC or ODOC\nThese comments are included in the statement type, but may be overridden with `WITH Comment=...`", singleton.Comment())

	cmd := builder.Command(appdef.NewQName("main", "Orders"))
	require.Equal("Commands can only be declared in workspaces\nCommand can have optional argument and/or unlogged argument\nCommand can return TYPE", cmd.Comment())

	// type
	obj := builder.Object(appdef.NewQName("main", "SubscriptionEvent"))
	require.Equal(appdef.TypeKind_Object, obj.Kind())
	require.Equal(appdef.DataKind_string, obj.Field("Origin").DataKind())

	// view
	view := builder.View(appdef.NewQName("main", "XZReports"))
	require.NotNil(view)
	require.Equal(appdef.TypeKind_ViewRecord, view.Kind())
	require.Equal("VIEWs generated by the PROJECTOR.\nPrimary Key must be declared in View.", view.Comment())

	require.Equal(2, view.Value().UserFieldCount())
	require.Equal(1, view.Key().Partition().FieldCount())
	require.Equal(4, view.Key().ClustCols().FieldCount())

	// workspace descriptor
	descr := builder.CDoc(appdef.NewQName("main", "MyWorkspaceDescriptor"))
	require.NotNil(descr)
	require.Equal(appdef.TypeKind_CDoc, descr.Kind())
	require.Equal(appdef.DataKind_string, descr.Field("Name").DataKind())
	require.Equal(appdef.DataKind_string, descr.Field("Country").DataKind())

	// fieldsets
	cdoc = builder.CDoc(appdef.NewQName("main", "WsTable"))
	require.Equal(appdef.DataKind_string, cdoc.Field("Name").DataKind())

	crec = builder.CRecord(appdef.NewQName("main", "Child"))
	require.Equal(appdef.DataKind_int32, crec.Field("Kind").DataKind())

	// QUERY
	q1 := builder.Query(appdef.NewQName("main", "_Query1"))
	require.NotNil(q1)
	require.Equal(appdef.TypeKind_Query, q1.Kind())
}

func Test_Refs_NestedTables(t *testing.T) {

	require := require.New(t)

	fs, err := ParseFile("file1.sql", `SCHEMA test;
	TABLE table1 INHERITS CDoc (
		items TABLE inner1 (
			table1 ref,
			ref1 ref(table3),
			urg_number int32
		)
	);
	TABLE table2 INHERITS CRecord (
	);
	TABLE table3 INHERITS CDoc (
		items table2
	);
	`)
	require.NoError(err)
	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.NoError(err)

	packages, err := MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg,
	})
	require.NoError(err)
	adf := appdef.New()
	require.NoError(BuildAppDefs(packages, adf))
	inner1 := adf.Type(appdef.NewQName("test", "inner1"))
	ref1 := inner1.(appdef.IFields).RefField("ref1")
	require.Len(ref1.Refs(), 1)
	require.Equal(appdef.NewQName("test", "table3"), ref1.Refs()[0])

}

func Test_CircularReferences(t *testing.T) {

	require := require.New(t)

	// Tables
	fs, err := ParseFile("file1.sql", `SCHEMA untill;
	TABLE table2 INHERITS table2 ();
	ABSTRACT TABLE table3 INHERITS table3 ();
	ABSTRACT TABLE table4 INHERITS table5 ();
	ABSTRACT TABLE table5 INHERITS table6 ();
	ABSTRACT TABLE table6 INHERITS table4 ();
	`)
	require.NoError(err)
	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.NoError(err)

	_, err = MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg,
	})

	require.EqualError(err, strings.Join([]string{
		"file1.sql:2:2: circular reference in INHERITS",
		"file1.sql:3:2: circular reference in INHERITS",
		"file1.sql:4:2: circular reference in INHERITS",
		"file1.sql:5:2: circular reference in INHERITS",
		"file1.sql:6:2: circular reference in INHERITS",
	}, "\n"))

	// Workspaces
	fs, err = ParseFile("file1.sql", `SCHEMA untill;
	ABSTRACT WORKSPACE w1();
	ABSTRACT WORKSPACE w2 INHERITS w1,w2(
		TABLE table4 INHERITS CDoc();
	);
	ABSTRACT WORKSPACE w3 INHERITS w4();
	ABSTRACT WORKSPACE w4 INHERITS w5();
	ABSTRACT WORKSPACE w5 INHERITS w3();
	`)
	require.NoError(err)
	pkg, err = MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.NoError(err)

	_, err = MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg,
	})

	require.EqualError(err, strings.Join([]string{
		"file1.sql:3:2: circular reference in INHERITS",
		"file1.sql:6:2: circular reference in INHERITS",
		"file1.sql:7:2: circular reference in INHERITS",
		"file1.sql:8:2: circular reference in INHERITS",
	}, "\n"))
}

func Test_Workspace_Defs(t *testing.T) {

	require := require.New(t)

	fs1, err := ParseFile("file1.sql", `SCHEMA myschema;
		ABSTRACT WORKSPACE AWorkspace(
			TABLE table1 INHERITS CDoc (a ref);		
		);
	`)
	require.NoError(err)
	fs2, err := ParseFile("file2.sql", `SCHEMA myschema;
		ALTER WORKSPACE AWorkspace(
			TABLE table2 INHERITS CDoc (a ref);		
		);
		WORKSPACE MyWorkspace INHERITS AWorkspace();
		WORKSPACE MyWorkspace2 INHERITS AWorkspace();
		ALTER WORKSPACE sys.Profile(
			USE WORKSPACE MyWorkspace;
		);
	`)
	require.NoError(err)
	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs1, fs2})
	require.NoError(err)

	packages, err := MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg,
	})
	require.NoError(err)
	builder := appdef.New()
	require.NoError(BuildAppDefs(packages, builder))
	ws := builder.Workspace(appdef.NewQName("myschema", "MyWorkspace"))

	require.Equal(appdef.TypeKind_CDoc, ws.Type(appdef.NewQName("myschema", "table1")).Kind())
	require.Equal(appdef.TypeKind_CDoc, ws.Type(appdef.NewQName("myschema", "table2")).Kind())
	require.Equal(appdef.TypeKind_Command, ws.Type(appdef.NewQName("sys", "CreateLogin")).Kind())

	wsProfile := builder.Workspace(appdef.NewQName("sys", "Profile"))

	require.Equal(appdef.TypeKind_Workspace, wsProfile.Type(appdef.NewQName("myschema", "MyWorkspace")).Kind())
	require.Nil(wsProfile.Type(appdef.NewQName("myschema", "MyWorkspace2")))
}

func Test_Alter_Workspace(t *testing.T) {

	require := require.New(t)

	fs1, err := ParseFile("file1.sql", `SCHEMA pkg1;
		ABSTRACT WORKSPACE AWorkspace(
			TABLE table1 INHERITS CDoc (a ref);		
		);
	`)
	require.NoError(err)
	pkg1, err := MergeFileSchemaASTs("org/pkg1", []*FileSchemaAST{fs1})
	require.NoError(err)

	fs2, err := ParseFile("file2.sql", `SCHEMA pkg2;
		IMPORT SCHEMA 'org/pkg1'
		ALTER WORKSPACE pkg1.AWorkspace(
			TABLE table2 INHERITS CDoc (a ref);		
		);
	`)
	require.NoError(err)
	pkg2, err := MergeFileSchemaASTs("org/pkg2", []*FileSchemaAST{fs2})
	require.NoError(err)

	_, err = MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg1,
		pkg2,
	})
	require.EqualError(err, strings.Join([]string{
		"file2.sql:3:3: workspace pkg1.AWorkspace is not alterable",
	}, "\n"))
}

func Test_DupFieldsInTypes(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("file1.sql", `SCHEMA test;
	TYPE RootType (
		Id int32
	);
	TYPE BaseType(
		RootType,
		baseField int
	);
	TYPE BaseType2 (
		someField int
	);
	TYPE MyType(
		BaseType,
		BaseType2,
		field varchar,
		field varchar,
		baseField varchar,
		someField int,
		Id varchar
	)
	`)
	require.NoError(err)
	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.NoError(err)

	packages, err := MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg,
	})
	require.NoError(err)

	err = BuildAppDefs(packages, appdef.New())
	require.EqualError(err, strings.Join([]string{
		"file1.sql:16:3: field redeclared",
		"file1.sql:17:3: baseField redeclared",
		"file1.sql:18:3: someField redeclared",
		"file1.sql:19:3: Id redeclared",
	}, "\n"))

}

func Test_Varchar(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("file1.sql", `SCHEMA test;
	TYPE RootType (
		Oversize varchar(1025)
	);
	TYPE CDoc1 (
		Oversize varchar(1025)
	);
	`)
	require.NoError(err)
	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.NoError(err)

	_, err = MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg,
	})
	require.EqualError(err, strings.Join([]string{
		"file1.sql:3:3: maximum field length is 1024",
		"file1.sql:6:3: maximum field length is 1024",
	}, "\n"))

}

func Test_DupFieldsInTables(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("file1.sql", `SCHEMA test;
	TYPE RootType (
		Kind int32
	);
	TYPE BaseType(
		RootType,
		baseField int
	);
	TYPE BaseType2 (
		someField int
	);
	ABSTRACT TABLE ByBaseTable INHERITS CDoc (
		Name varchar,
		Code varchar
	);
	TABLE MyTable INHERITS ByBaseTable(
		BaseType,
		BaseType2,
		newField varchar,
		field varchar,
		field varchar, 		-- duplicated in the this table
		baseField varchar,		-- duplicated in the first OF
		someField int,		-- duplicated in the second OF
		Kind int,			-- duplicated in the first OF (2nd level)
		Name int,			-- duplicated in the inherited table
		ID varchar
	)
	`)
	require.NoError(err)
	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.NoError(err)

	packages, err := MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg,
	})
	require.NoError(err)

	err = BuildAppDefs(packages, appdef.New())
	require.EqualError(err, strings.Join([]string{
		"file1.sql:21:3: field redeclared",
		"file1.sql:22:3: baseField redeclared",
		"file1.sql:23:3: someField redeclared",
		"file1.sql:24:3: Kind redeclared",
		"file1.sql:25:3: Name redeclared",
	}, "\n"))

}

func Test_AbstractTables(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("file1.sql", `SCHEMA test;
	TABLE ByBaseTable INHERITS CDoc (
		Name varchar
	);
	TABLE MyTable INHERITS ByBaseTable(		-- NOT ALLOWED
	);

	TABLE My1 INHERITS CRecord(
		f1 ref(AbstractTable)				-- NOT ALLOWED
	);

	ABSTRACT TABLE AbstractTable INHERITS CDoc(
	);

	WORKSPACE MyWorkspace1(
		EXTENSION ENGINE BUILTIN (

			PROJECTOR proj1
            AFTER INSERT ON AbstractTable 	-- NOT ALLOWED
            INTENTS(SendMail);

			SYNC PROJECTOR proj2
            AFTER INSERT ON My1
            INTENTS(Record AbstractTable);	-- NOT ALLOWED

			PROJECTOR proj3
            AFTER INSERT ON My1
			STATE(Record AbstractTable)		-- NOT ALLOWED
            INTENTS(SendMail);
		);
		TABLE My2 INHERITS CRecord(
			nested AbstractTable			-- NOT ALLOWED
		);
		USE TABLE AbstractTable;			-- NOT ALLOWED
		TABLE My3 INHERITS CRecord(
			f int,
			items ABSTRACT TABLE Nested()	-- NOT ALLOWED
		);
	)
	`)
	require.NoError(err)
	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.NoError(err)

	_, err = MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg,
	})
	require.EqualError(err, strings.Join([]string{
		"file1.sql:5:2: base table must be abstract",
		"file1.sql:9:3: reference to abstract table AbstractTable",
		"file1.sql:18:4: projector refers to abstract table AbstractTable",
		"file1.sql:22:4: projector refers to abstract table AbstractTable",
		"file1.sql:26:4: projector refers to abstract table AbstractTable",
		"file1.sql:34:3: use of abstract table AbstractTable",
		"file1.sql:37:10: nested abstract table Nested",
	}, "\n"))

}

func Test_AbstractTables2(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("file1.sql", `SCHEMA test;
	ABSTRACT TABLE AbstractTable INHERITS CDoc(
	);

	WORKSPACE MyWorkspace1(
		TABLE My2 INHERITS CRecord(
			nested AbstractTable			-- NOT ALLOWED
		);
	);
	`)
	require.NoError(err)
	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.NoError(err)

	packages, err := MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg,
	})
	require.NoError(err)

	err = BuildAppDefs(packages, appdef.New())
	require.EqualError(err, strings.Join([]string{
		"file1.sql:7:4: nested abstract table AbstractTable",
	}, "\n"))

}

func Test_WorkspaceDescriptors(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("file1.sql", `SCHEMA test;
	ROLE R1;
	WORKSPACE W1(
		DESCRIPTOR(); -- gets name W1Descriptor
	);
	WORKSPACE W2(
		DESCRIPTOR W2D(); -- gets name W2D
	);
	WORKSPACE W3(
		DESCRIPTOR R1(); -- duplicated name
	);
	ROLE W2D; -- duplicated name
	`)
	require.NoError(err)
	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.EqualError(err, strings.Join([]string{
		"file1.sql:10:14: R1 redeclared",
		"file1.sql:12:2: W2D redeclared",
	}, "\n"))

	require.Equal(Ident("W1Descriptor"), pkg.Ast.Statements[1].Workspace.Descriptor.Name)
	require.Equal(Ident("W2D"), pkg.Ast.Statements[2].Workspace.Descriptor.Name)
}
func Test_PanicUnknownFieldType(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("file1.sql", `SCHEMA test;
	TABLE MyTable INHERITS CDoc (
		Name asdasd,
		Code varchar
	);
	`)
	require.NoError(err)
	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.NoError(err)

	packages, err := MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg,
	})
	require.NoError(err)

	err = BuildAppDefs(packages, appdef.New())
	require.EqualError(err, strings.Join([]string{
		"file1.sql:3:3: asdasd type not supported",
	}, "\n"))

}

func Test_Expressions(t *testing.T) {
	require := require.New(t)

	_, err := ParseFile("file1.sql", `SCHEMA test;
	TABLE MyTable(
		Int1 varchar DEFAULT 1 CHECK(Int1 > Int2),
		Int1 int DEFAULT 1 CHECK(Text != 'asd'),
		Int1 int DEFAULT 1 CHECK(Int2 > -5),
		Int1 int DEFAULT 1 CHECK(TextField > 'asd' AND (SomeFloat/3.2)*4 != 5.003),
		Int1 int DEFAULT 1 CHECK(SomeFunc('a', TextField) AND BoolField=FALSE),

		CHECK(MyRowValidator(this))
	)
	`)
	require.NoError(err)

}

func Test_Duplicates(t *testing.T) {
	require := require.New(t)

	ast1, err := ParseFile("file1.sql", `SCHEMA test;
	EXTENSION ENGINE BUILTIN (
		FUNCTION MyTableValidator() RETURNS void;
		FUNCTION MyTableValidator(TableRow) RETURNS string;
		FUNCTION MyFunc2() RETURNS void;
	);
	TABLE Rec1 INHERITS CRecord();
	`)
	require.NoError(err)

	ast2, err := ParseFile("file2.sql", `SCHEMA test;
	WORKSPACE ChildWorkspace (
		TAG MyFunc2; -- redeclared
		EXTENSION ENGINE BUILTIN (
			FUNCTION MyFunc3() RETURNS void;
			FUNCTION MyFunc4() RETURNS void;
		);
		WORKSPACE InnerWorkspace (
			ROLE MyFunc4; -- redeclared
		);
		TABLE Doc1 INHERITS ODoc(
			nested1 Rec1,
			nested2 TABLE Rec1() -- redeclared
		)
	)
	`)
	require.NoError(err)

	_, err = MergeFileSchemaASTs("", []*FileSchemaAST{ast1, ast2})

	require.EqualError(err, strings.Join([]string{
		"file1.sql:4:3: MyTableValidator redeclared",
		"file2.sql:3:3: MyFunc2 redeclared",
		"file2.sql:9:4: MyFunc4 redeclared",
		"file2.sql:13:12: Rec1 redeclared",
	}, "\n"))

}

func Test_DuplicatesInViews(t *testing.T) {
	require := require.New(t)

	ast, err := ParseFile("file2.sql", `SCHEMA test;
	WORKSPACE Workspace (
		VIEW test(
			field1 int,
			field2 int,
			field1 varchar,
			PRIMARY KEY(field1),
			PRIMARY KEY(field2)
		) AS RESULT OF Proj1;
	)
	`)
	require.NoError(err)

	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{ast})
	require.NoError(err)

	_, err = MergePackageSchemas([]*PackageSchemaAST{
		pkg,
	})

	require.EqualError(err, strings.Join([]string{
		"file2.sql:6:4: field1 redeclared",
		"file2.sql:8:16: primary key redeclared",
	}, "\n"))

}
func Test_Views(t *testing.T) {
	require := require.New(t)

	f := func(sql string, expectErrors ...string) {
		ast, err := ParseFile("file2.sql", sql)
		require.NoError(err)
		pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{ast})
		require.NoError(err)

		_, err = MergePackageSchemas([]*PackageSchemaAST{
			getSysPackageAST(),
			pkg,
		})
		require.EqualError(err, strings.Join(expectErrors, "\n"))
	}

	f(`SCHEMA test; WORKSPACE Workspace (
			VIEW test(
				field1 int,
				PRIMARY KEY(field2)
			) AS RESULT OF Proj1;
		)
	`, "file2.sql:4:17: undefined field field2")

	f(`SCHEMA test; WORKSPACE Workspace (
			VIEW test(
				field1 varchar,
				PRIMARY KEY((field1))
			) AS RESULT OF Proj1;
		)
	`, "file2.sql:4:17: varchar field field1 not supported in partition key")

	f(`SCHEMA test; WORKSPACE Workspace (
		VIEW test(
			field1 bytes,
			PRIMARY KEY((field1))
		) AS RESULT OF Proj1;
	)
	`, "file2.sql:4:16: bytes field field1 not supported in partition key")

	f(`SCHEMA test; WORKSPACE Workspace (
		VIEW test(
			field1 varchar,
			field2 int,
			PRIMARY KEY(field1, field2)
		) AS RESULT OF Proj1;
	)
	`, "file2.sql:5:16: varchar field field1 can only be the last one in clustering key")

	f(`SCHEMA test; WORKSPACE Workspace (
		VIEW test(
			field1 bytes,
			field2 int,
			PRIMARY KEY(field1, field2)
		) AS RESULT OF Proj1;
	)
	`, "file2.sql:5:16: bytes field field1 can only be the last one in clustering key")

	f(`SCHEMA test; WORKSPACE Workspace (
		ABSTRACT TABLE abc INHERITS CDoc();
		VIEW test(
			field1 ref(abc),
			field2 ref(unexisting),
			PRIMARY KEY(field1, field2)
		) AS RESULT OF Proj1;
	)
	`, "file2.sql:4:4: reference to abstract table abc", "file2.sql:5:4: unexisting undefined")
}

func Test_Views2(t *testing.T) {
	require := require.New(t)

	{
		ast, err := ParseFile("file2.sql", `SCHEMA test; WORKSPACE Workspace (
			VIEW test(
				-- comment1
				field1 int,
				-- comment2
				field2 varchar(20),
				-- comment3
				field3 bytes(20),
				-- comment4
				field4 ref,
				PRIMARY KEY((field1,field4),field2)
			) AS RESULT OF Proj1;
		)
		`)
		require.NoError(err)
		pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{ast})
		require.NoError(err)

		packages, err := MergePackageSchemas([]*PackageSchemaAST{
			getSysPackageAST(),
			pkg,
		})
		require.NoError(err)

		def := appdef.New()
		err = BuildAppDefs(packages, def)
		require.NoError(err)

		v := def.View(appdef.NewQName("test", "test"))
		require.NotNil(v)
	}
	{
		ast, err := ParseFile("file2.sql", `SCHEMA test; WORKSPACE Workspace (
			VIEW test(
				-- comment1
				field1 int,
				-- comment2
				field3 bytes(20),
				-- comment4
				field4 ref,
				PRIMARY KEY((field1),field4,field3)
			) AS RESULT OF Proj1;
		)
		`)
		require.NoError(err)
		pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{ast})
		require.NoError(err)

		packages, err := MergePackageSchemas([]*PackageSchemaAST{
			getSysPackageAST(),
			pkg,
		})
		require.NoError(err)

		def := appdef.New()
		err = BuildAppDefs(packages, def)
		require.NoError(err)

		v := def.View(appdef.NewQName("test", "test"))
		require.NotNil(v)

	}

}
func Test_Comments(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("example.sql", `SCHEMA test;
	EXTENSION ENGINE BUILTIN (

	-- My function
	-- line 2
	FUNCTION MyFunc() RETURNS void;

	/* 	Multiline 
		comment  */
	FUNCTION MyFunc1() RETURNS void;
	);

	`)
	require.NoError(err)

	ps, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.Nil(err)

	require.NotNil(ps.Ast.Statements[0].ExtEngine.Statements[0].Function.Comments)

	comments := ps.Ast.Statements[0].ExtEngine.Statements[0].Function.GetComments()
	require.Equal(2, len(comments))
	require.Equal("My function", comments[0])
	require.Equal("line 2", comments[1])

	fn := ps.Ast.Statements[0].ExtEngine.Statements[1].Function
	comments = fn.GetComments()
	require.Equal(2, len(comments))
	require.Equal("Multiline", comments[0])
	require.Equal("comment", comments[1])
}

func Test_UnexpectedSchema(t *testing.T) {
	require := require.New(t)

	ast1, err := ParseFile("file1.sql", `SCHEMA schema1; ROLE abc;`)
	require.NoError(err)

	ast2, err := ParseFile("file2.sql", `SCHEMA schema2; ROLE xyz;`)
	require.NoError(err)

	_, err = MergeFileSchemaASTs("", []*FileSchemaAST{ast1, ast2})
	require.EqualError(err, "file2.sql: package schema2; expected schema1")
}

func Test_Undefined(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("example.sql", `SCHEMA test;
	WORKSPACE test (
		EXTENSION ENGINE WASM (
			COMMAND Orders() WITH Tags=(UndefinedTag);
			QUERY Query1 RETURNS void WITH Rate=UndefinedRate;
			PROJECTOR ImProjector ON xyz.CreateUPProfile;
			COMMAND CmdFakeReturn() RETURNS text;
			COMMAND CmdNoReturn() RETURNS void;
			COMMAND CmdFakeArg(text);
			COMMAND CmdVoidArg(void);
			COMMAND CmdFakeUnloggedArg(UNLOGGED text);
		)
	)
	`)
	require.Nil(err)

	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.Nil(err)

	_, err = MergePackageSchemas([]*PackageSchemaAST{pkg, getSysPackageAST()})

	require.EqualError(err, strings.Join([]string{
		"example.sql:4:4: UndefinedTag undefined",
		"example.sql:5:4: UndefinedRate undefined",
		"example.sql:6:4: xyz undefined",
		"example.sql:7:4: text undefined",
		"example.sql:9:4: text undefined",
		"example.sql:11:4: text undefined",
	}, "\n"))
}

func Test_Projectors(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("example.sql", `SCHEMA test;
	WORKSPACE test (
		TABLE Order INHERITS ODoc();
		EXTENSION ENGINE WASM (
			COMMAND Orders();
			PROJECTOR ImProjector1 ON test.CreateUPProfile; 		-- Undefined
			PROJECTOR ImProjector2 ON Order; 						-- Good
			PROJECTOR ImProjector3 AFTER UPDATE ON Order; 			-- Bad
			PROJECTOR ImProjector4 AFTER ACTIVATE ON Order; 		-- Bad
			PROJECTOR ImProjector5 AFTER DEACTIVATE ON Order; 		-- Bad
		)
	)
	`)
	require.Nil(err)

	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.NoError(err)

	_, err = MergePackageSchemas([]*PackageSchemaAST{pkg, getSysPackageAST()})

	require.EqualError(err, strings.Join([]string{
		"example.sql:6:4: test.CreateUPProfile undefined, expected command, type or table",
		"example.sql:8:4: only INSERT allowed for ODoc or ORecord",
		"example.sql:9:4: only INSERT allowed for ODoc or ORecord",
		"example.sql:10:4: only INSERT allowed for ODoc or ORecord",
	}, "\n"))
}

func Test_Imports(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("example.sql", `SCHEMA pkg1;
	IMPORT SCHEMA 'github.com/untillpro/airsbp3/pkg2';
	IMPORT SCHEMA 'github.com/untillpro/airsbp3/pkg3' AS air;
	WORKSPACE test (
		EXTENSION ENGINE WASM (
    		COMMAND Orders WITH Tags=(pkg2.SomeTag);
    		QUERY Query2 RETURNS void WITH Tags=(air.SomePkg3Tag);
    		QUERY Query3 RETURNS void WITH Tags=(air.UnknownTag); -- air.UnknownTag undefined
    		PROJECTOR ImProjector ON Air.CreateUPProfil; -- Air undefined
		)
	)
	`)
	require.NoError(err)
	pkg1, err := MergeFileSchemaASTs("github.com/untillpro/airsbp3/pkg1", []*FileSchemaAST{fs})
	require.NoError(err)

	fs, err = ParseFile("example.sql", `SCHEMA pkg2;
	TAG SomeTag;
	`)
	require.NoError(err)
	pkg2, err := MergeFileSchemaASTs("github.com/untillpro/airsbp3/pkg2", []*FileSchemaAST{fs})
	require.NoError(err)

	fs, err = ParseFile("example.sql", `SCHEMA pkg3;
	TAG SomePkg3Tag;
	`)
	require.NoError(err)
	pkg3, err := MergeFileSchemaASTs("github.com/untillpro/airsbp3/pkg3", []*FileSchemaAST{fs})
	require.NoError(err)

	_, err = MergePackageSchemas([]*PackageSchemaAST{getSysPackageAST(), pkg1, pkg2, pkg3})
	require.EqualError(err, strings.Join([]string{
		"example.sql:8:7: air.UnknownTag undefined",
		"example.sql:9:7: Air undefined",
	}, "\n"))

}

func Test_AbstractWorkspace(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("example.sql", `SCHEMA test;
	WORKSPACE ws1 ();
	ABSTRACT WORKSPACE ws2(
		DESCRIPTOR(					-- Incorrect
			a int
		);
	);
	WORKSPACE ws4 INHERITS ws2 ();
	WORKSPACE ws5 INHERITS ws1 ();  -- Incorrect
	`)
	require.Nil(err)

	ps, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.Nil(err)

	require.False(ps.Ast.Statements[0].Workspace.Abstract)
	require.True(ps.Ast.Statements[1].Workspace.Abstract)
	require.False(ps.Ast.Statements[2].Workspace.Abstract)
	require.Equal("ws2", ps.Ast.Statements[2].Workspace.Inherits[0].String())

	_, err = MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		ps,
	})
	require.EqualError(err, strings.Join([]string{
		"example.sql:3:2: abstract workspace cannot have a descriptor",
		"example.sql:9:2: base workspace must be abstract",
	}, "\n"))

}

func Test_UniqueFields(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("example.sql", `SCHEMA test;
	TABLE MyTable INHERITS CDoc (
		Int1 int32,
		Int2 int32 NOT NULL,
		UNIQUEFIELD UnknownField,
		UNIQUEFIELD Int1,
		UNIQUEFIELD Int2
	)
	`)
	require.Nil(err)

	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.Nil(err)

	packages, err := MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg,
	})
	require.NoError(err)

	def := appdef.New()
	err = BuildAppDefs(packages, def)
	require.EqualError(err, strings.Join([]string{
		"example.sql:5:3: undefined field UnknownField",
		"example.sql:6:3: field has to be NOT NULL",
	}, "\n"))

	cdoc := def.CDoc(appdef.NewQName("test", "MyTable"))
	require.NotNil(cdoc)

	fld := cdoc.UniqueField()
	require.NotNil(fld)
	require.Equal("Int2", fld.Name())
}

func Test_NestedTables(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("example.sql", `SCHEMA test;
	TABLE NestedTable INHERITS CRecord (
		ItemName varchar,
		DeepNested TABLE DeepNestedTable (
			ItemName varchar
		)
	);
	`)
	require.Nil(err)

	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.Nil(err)

	packages, err := MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg,
	})
	require.NoError(err)

	def := appdef.New()
	err = BuildAppDefs(packages, def)
	require.NoError(err)

	require.NotNil(def.CRecord(appdef.NewQName("test", "NestedTable")))
	require.NotNil(def.CRecord(appdef.NewQName("test", "DeepNestedTable")))
}

func Test_SemanticAnalysisForReferences(t *testing.T) {
	t.Run("Should return error because CDoc references to ODoc", func(t *testing.T) {
		require := require.New(t)

		fs, err := ParseFile("example.sql", `SCHEMA test;
		TABLE OTable INHERITS ODoc ();
		TABLE CTable INHERITS CDoc (
			OTableRef ref(OTable)
		);
		`)
		require.Nil(err)

		pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
		require.Nil(err)

		packages, err := MergePackageSchemas([]*PackageSchemaAST{
			getSysPackageAST(),
			pkg,
		})
		require.NoError(err)

		def := appdef.New()
		err = BuildAppDefs(packages, def)

		require.Contains(err.Error(), "table test.CTable can not reference to table test.OTable")
	})
}

func Test_1KStringField(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("example.sql", `SCHEMA test;
	TABLE MyTable INHERITS CDoc (
		KB varchar(1024)
	)
	`)
	require.Nil(err)

	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.Nil(err)

	packages, err := MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg,
	})
	require.NoError(err)

	def := appdef.New()
	err = BuildAppDefs(packages, def)
	require.NoError(err)

	cdoc := def.CDoc(appdef.NewQName("test", "MyTable"))
	require.NotNil(cdoc)

	fld := cdoc.Field("KB").(appdef.IStringField)
	require.NotNil(fld)
	require.EqualValues(1024, fld.Restricts().MaxLen())
}

func Test_ReferenceToNoTable(t *testing.T) {
	require := require.New(t)

	fs, err := ParseFile("example.sql", `SCHEMA test;
	ROLE Admin;
	TABLE CTable INHERITS CDoc (
		RefField ref(Admin)
	);
	`)
	require.Nil(err)

	pkg, err := MergeFileSchemaASTs("", []*FileSchemaAST{fs})
	require.Nil(err)

	_, err = MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		pkg,
	})
	require.Contains(err.Error(), "Admin undefined")

}

func Test_VRestaurantBasic(t *testing.T) {

	require := require.New(t)

	vRestaurantPkgAST, err := ParsePackageDir("github.com/untillpro/vrestaurant", fsvRestaurant, "sql_example_app/vrestaurant")
	require.NoError(err)

	packages, err := MergePackageSchemas([]*PackageSchemaAST{
		getSysPackageAST(),
		vRestaurantPkgAST,
	})
	require.NoError(err)

	builder := appdef.New()
	err = BuildAppDefs(packages, builder)
	require.NoError(err)

	// table
	cdoc := builder.Type(appdef.NewQName("vrestaurant", "TablePlan"))
	require.NotNil(cdoc)
	require.Equal(appdef.TypeKind_CDoc, cdoc.Kind())
	require.Equal(appdef.DataKind_RecordID, cdoc.(appdef.IFields).Field("Picture").DataKind())

	cdoc = builder.Type(appdef.NewQName("vrestaurant", "Client"))
	require.NotNil(cdoc)

	cdoc = builder.Type(appdef.NewQName("vrestaurant", "POSUser"))
	require.NotNil(cdoc)

	cdoc = builder.Type(appdef.NewQName("vrestaurant", "Department"))
	require.NotNil(cdoc)

	cdoc = builder.Type(appdef.NewQName("vrestaurant", "Article"))
	require.NotNil(cdoc)

	// child table
	crec := builder.Type(appdef.NewQName("vrestaurant", "TableItem"))
	require.NotNil(crec)
	require.Equal(appdef.TypeKind_CRecord, crec.Kind())
	require.Equal(appdef.DataKind_int32, crec.(appdef.IFields).Field("Tableno").DataKind())

	// view
	view := builder.View(appdef.NewQName("vrestaurant", "SalesPerDay"))
	require.NotNil(view)
	require.Equal(appdef.TypeKind_ViewRecord, view.Kind())

}

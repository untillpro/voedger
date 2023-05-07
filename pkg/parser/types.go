/*
* Copyright (c) 2023-present unTill Pro, Ltd.
* @author Michael Saigachenko
 */

package parser

import (
	"fmt"
	fs "io/fs"

	"github.com/alecthomas/participle/v2/lexer"
)

type FileSchemaAST struct {
	FileName string
	Ast      *SchemaAST
}

type PackageSchemaAST struct {
	QualifiedPackageName string
	Ast                  *SchemaAST
}

type IReadFS interface {
	fs.ReadDirFS
	fs.ReadFileFS
}

type IStatement interface {
	GetPos() *lexer.Position
	GetComments() *[]string
}

type INamedStatement interface {
	IStatement
	GetName() string
}
type IStatementCollection interface {
	Iterate(callback func(stmt interface{}))
}

type SchemaAST struct {
	Package    string          `parser:"'SCHEMA' @Ident ';'"`
	Imports    []ImportStmt    `parser:"@@? (';' @@)* ';'?"`
	Statements []RootStatement `parser:"@@? (';' @@)* ';'?"`
}

func (s *SchemaAST) Iterate(callback func(stmt interface{})) {
	for i := 0; i < len(s.Statements); i++ {
		raw := &s.Statements[i]
		if raw.stmt == nil {
			raw.stmt = extractStatement(*raw)
		}
		callback(raw.stmt)
	}
}

type ImportStmt struct {
	Pos   lexer.Position
	Name  string  `parser:"'IMPORT' 'SCHEMA' @String"`
	Alias *string `parser:"('AS' @Ident)?"`
}

type RootStatement struct {
	// Only allowed in root
	Template *TemplateStmt `parser:"@@"`

	// Also allowed in root
	Role      *RoleStmt      `parser:"| @@"`
	Comment   *CommentStmt   `parser:"| @@"`
	Tag       *TagStmt       `parser:"| @@"`
	Function  *FunctionStmt  `parser:"| @@"`
	Workspace *WorkspaceStmt `parser:"| @@"`
	Table     *TableStmt     `parser:"| @@"`
	Type      *TypeStmt      `parser:"| @@"`
	// Sequence  *sequenceStmt  `parser:"| @@"`

	stmt interface{}
}

type WorkspaceStatement struct {
	// Only allowed in workspace
	Projector *ProjectorStmt `parser:"@@"`
	Command   *CommandStmt   `parser:"| @@"`
	Query     *QueryStmt     `parser:"| @@"`
	Rate      *RateStmt      `parser:"| @@"`
	View      *ViewStmt      `parser:"| @@"`
	UseTable  *UseTableStmt  `parser:"| @@"`

	// Also allowed in workspace
	Role      *RoleStmt      `parser:"| @@"`
	Comment   *CommentStmt   `parser:"| @@"`
	Tag       *TagStmt       `parser:"| @@"`
	Function  *FunctionStmt  `parser:"| @@"`
	Workspace *WorkspaceStmt `parser:"| @@"`
	Table     *TableStmt     `parser:"| @@"`
	Type      *TypeStmt      `parser:"| @@"`
	//Sequence  *sequenceStmt  `parser:"| @@"`
	Grant *GrantStmt `parser:"| @@"`

	stmt interface{}
}

type WorkspaceStmt struct {
	Statement
	Abstract   bool                 `parser:"@'ABSTRACT'?"`
	Name       string               `parser:"'WORKSPACE' @Ident "`
	Of         []DefQName           `parser:"('OF' @@ (',' @@)*)?"`
	A          int                  `parser:"'('"`
	Descriptor *WsDescriptorStmt    `parser:"('DESCRIPTOR' @@)?"`
	Statements []WorkspaceStatement `parser:"@@? (';' @@)* ';'? ')'"`
}

func (s WorkspaceStmt) GetName() string { return s.Name }
func (s *WorkspaceStmt) Iterate(callback func(stmt interface{})) {
	for i := 0; i < len(s.Statements); i++ {
		raw := &s.Statements[i]
		if raw.stmt == nil {
			raw.stmt = extractStatement(*raw)
		}
		callback(raw.stmt)
	}
}

type TypeStmt struct {
	Statement
	Name  string          `parser:"'TYPE' @Ident "`
	Of    []DefQName      `parser:"('OF' @@ (',' @@)*)?"`
	Items []TableItemExpr `parser:"'(' @@ (',' @@)* ')'"`
}

func (s TypeStmt) GetName() string { return s.Name }

type WsDescriptorStmt struct {
	Statement
	Of    []DefQName      `parser:"('OF' @@ (',' @@)*)?"`
	Items []TableItemExpr `parser:"'(' @@ (',' @@)* ')'"`
	_     int             `parser:"';'"`
}

type DefQName struct {
	Package string `parser:"(@Ident '.')?"`
	Name    string `parser:"@Ident"`
}

type TypeQName struct {
	Package string `parser:"(@Ident '.')?"`
	Name    string `parser:"@Ident"`
	IsArray bool   `parser:"@Array?"`
}

func (q DefQName) String() string {
	if q.Package == "" {
		return q.Name
	}
	return fmt.Sprintf("%s.%s", q.Package, q.Name)

}

type Statement struct {
	Pos      lexer.Position
	Comments []string `parser:"@Comment*"`
}

func (s *Statement) GetPos() *lexer.Position {
	return &s.Pos
}

func (s *Statement) GetComments() *[]string {
	return &s.Comments
}

type ProjectorStmt struct {
	Statement
	Name    string      `parser:"'PROJECTOR' @Ident?"`
	On      ProjectorOn `parser:"'ON' @@"`
	Targets []DefQName  `parser:"(('IN' '(' @@ (',' @@)* ')') | @@)!"`
	Func    DefQName    `parser:"'AS' @@"`
}

func (s ProjectorStmt) GetName() string { return s.Name }

type ProjectorOn struct {
	CommandArgument bool `parser:"@('COMMAND' 'ARGUMENT')"`
	Command         bool `parser:"| @('COMMAND')"`
	Insert          bool `parser:"| @(('INSERT' ('OR' 'UPDATE')?) | ('UPDATE' 'OR' 'INSERT'))"`
	Update          bool `parser:"| @(('UPDATE' ('OR' 'INSERT')?) | ('INSERT' 'OR' 'UPDATE'))"`
	Activate        bool `parser:"| @(('ACTIVATE' ('OR' 'DEACTIVATE')?) | ('DEACTIVATE' 'OR' 'ACTIVATE'))"`
	Deactivate      bool `parser:"| @(('DEACTIVATE' ('OR' 'ACTIVATE')?) | ('ACTIVATE' 'OR' 'DEACTIVATE'))"`
}

type TemplateStmt struct {
	Statement
	Name      string   `parser:"'TEMPLATE' @Ident 'OF' 'WORKSPACE'" `
	Workspace DefQName `parser:"@@"`
	Source    string   `parser:"'SOURCE' @Ident"`
}

func (s TemplateStmt) GetName() string { return s.Name }

type RoleStmt struct {
	Statement
	Name string `parser:"'ROLE' @Ident"`
}

func (s RoleStmt) GetName() string { return s.Name }

type TagStmt struct {
	Statement
	Name string `parser:"'TAG' @Ident"`
}

func (s TagStmt) GetName() string { return s.Name }

type CommentStmt struct {
	Statement
	Name  string `parser:"'COMMENT' @Ident"`
	Value string `parser:"@String"`
}

func (s CommentStmt) GetName() string { return s.Name }

type UseTableStmt struct {
	Statement
	Package   string `parser:"'USE' 'TABLE' (@Ident '.')?"`
	Name      string `parser:"(@Ident "`
	AllTables bool   `parser:"| @'*')"`
}

type UseTableItem struct {
	Package   string `parser:"(@Ident '.')?"`
	Name      string `parser:"(@Ident "`
	AllTables bool   `parser:"| @'*')"`
}

/*type sequenceStmt struct {
	Name        string `parser:"'SEQUENCE' @Ident"`
	Type        string `parser:"@Ident"`
	StartWith   *int   `parser:"(('START' 'WITH' @Number)"`
	MinValue    *int   `parser:"| ('MINVALUE' @Number)"`
	MaxValue    *int   `parser:"| ('MAXVALUE' @Number)"`
	IncrementBy *int   `parser:"| ('INCREMENT' 'BY' @Number) )*"`
}*/

type RateStmt struct {
	Statement
	Name   string `parser:"'RATE' @Ident"`
	Amount int    `parser:"@Int"`
	Per    string `parser:"'PER' @('SECOND' | 'MINUTE' | 'HOUR' | 'DAY' | 'YEAR')"`
	PerIP  bool   `parser:"(@('PER' 'IP'))?"`
}

func (s RateStmt) GetName() string { return s.Name }

type GrantStmt struct {
	Statement
	Grants []string `parser:"'GRANT' @('ALL' | 'EXECUTE' | 'SELECT' | 'INSERT' | 'UPDATE') (','  @('ALL' | 'EXECUTE' | 'SELECT' | 'INSERT' | 'UPDATE'))*"`
	On     string   `parser:"'ON' @('TABLE' | ('ALL' 'TABLES' 'WITH' 'TAG') | 'COMMAND' | ('ALL' 'COMMANDS' 'WITH' 'TAG') | 'QUERY' | ('ALL' 'QUERIES' 'WITH' 'TAG'))"`
	Target DefQName `parser:"@@"`
	To     string   `parser:"'TO' @Ident"`
}

type FunctionStmt struct {
	Statement
	Name    string          `parser:"'FUNCTION' @Ident"`
	Params  []FunctionParam `parser:"'(' @@? (',' @@)* ')'"`
	Returns TypeQName       `parser:"'RETURNS' @@"`
	Engine  EngineType      `parser:"'ENGINE' @@"`
}

func (s FunctionStmt) GetName() string { return s.Name }

type CommandStmt struct {
	Statement
	Name   string          `parser:"'COMMAND' @Ident"`
	Params []FunctionParam `parser:"('(' @@? (',' @@)* ')')?"`
	Func   DefQName        `parser:"'AS' @@"`
	With   []WithItem      `parser:"('WITH' @@ (',' @@)* )?"`
}

func (s CommandStmt) GetName() string { return s.Name }

type WithItem struct {
	Comment *DefQName  `parser:"('Comment' '=' @@)"`
	Tags    []DefQName `parser:"| ('Tags' '=' '[' @@ (',' @@)* ']')"`
	Rate    *DefQName  `parser:"| ('Rate' '=' @@)"`
}

type QueryStmt struct {
	Statement
	Name    string          `parser:"'QUERY' @Ident"`
	Params  []FunctionParam `parser:"('(' @@? (',' @@)* ')')?"`
	Returns TypeQName       `parser:"'RETURNS' @@"`
	Func    DefQName        `parser:"'AS' @@"`
	With    []WithItem      `parser:"('WITH' @@ (',' @@)* )?"`
}

func (s QueryStmt) GetName() string { return s.Name }

type EngineType struct {
	WASM    bool `parser:"@'WASM'"`
	Builtin bool `parser:"| @'BUILTIN'"`
}

type FunctionParam struct {
	NamedParam       *NamedParam `parser:"@@"`
	UnnamedParamType *TypeQName  `parser:"| @@"`
}

type NamedParam struct {
	Name string    `parser:"@Ident"`
	Type TypeQName `parser:"@@"`
}

// TODO: validate that table has no duplicated fields
type TableStmt struct {
	Statement
	Name     string          `parser:"'TABLE' @Ident"`
	Inherits DefQName        `parser:"('INHERITS' @@)?"`
	Of       []DefQName      `parser:"('OF' @@ (',' @@)*)?"`
	Items    []TableItemExpr `parser:"'(' @@ (',' @@)* ')'"`
	With     []WithItem      `parser:"('WITH' @@ (',' @@)* )?"`
}

func (s TableStmt) GetName() string { return s.Name }

type TableItemExpr struct {
	Table  *TableStmt      `parser:"@@"`
	Unique *UniqueExpr     `parser:"| @@"`
	Check  *TableCheckExpr `parser:"| @@"`
	Field  *FieldExpr      `parser:"| @@"`
}

type TableCheckExpr struct {
	ConstraintName string     `parser:"('CONSTRAINT' @Ident)?"`
	Expression     Expression `parser:"'CHECK' '(' @@ ')'"`
}

type UniqueExpr struct {
	Fields []string `parser:"'UNIQUE' '(' @Ident (',' @Ident)* ')'"`
}

type FieldExpr struct {
	Name               string      `parser:"@Ident"`
	Type               TypeQName   `parser:"@@"`
	NotNull            bool        `parser:"@(NOTNULL)?"`
	Verifiable         bool        `parser:"@('VERIFIABLE')?"`
	DefaultIntValue    *int        `parser:"('DEFAULT' @Int)?"`
	DefaultStringValue *string     `parser:"('DEFAULT' @String)?"`
	DefaultNextVal     *string     `parser:"(DEFAULTNEXTVAL  '(' @String ')')?"`
	References         *DefQName   `parser:"('REFERENCES' @@)?"`
	CheckRegexp        *string     `parser:"('CHECK' @String)?"`
	CheckExpression    *Expression `parser:"('CHECK' '(' @@ ')')?"`
}

type ViewStmt struct {
	Statement
	Name     string         `parser:"'VIEW' @Ident"`
	Fields   []ViewItemExpr `parser:"'(' @@? (',' @@)* ')'"`
	ResultOf DefQName       `parser:"'AS' 'RESULT' 'OF' @@"`
}

// TODO: validate that view has not more than 1 PrimaryKeyExpr
// TODO: validate that view has no duplicated fields
type ViewItemExpr struct {
	PrimaryKey *PrimaryKeyExpr `parser:"(PRIMARYKEY '(' @@ ')')"`
	Field      *ViewField      `parser:"| @@"`
}

type PrimaryKeyExpr struct {
	PartitionKeyFields      []string `parser:"('(' @Ident (',' @Ident)* ')')?"`
	ClusteringColumnsFields []string `parser:"','? @Ident (',' @Ident)*"`
}

func (s ViewStmt) GetName() string { return s.Name }

type ViewField struct {
	Name string `parser:"@Ident"`
	Type string `parser:"@Ident"` // TODO: viewField: predefined types?
}

// TODO TYPE + "TABLE|WORKSPACE OF" validation

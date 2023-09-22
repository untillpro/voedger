/*
* Copyright (c) 2023-present unTill Pro, Ltd.
* @author Michael Saigachenko
 */

package parser

import (
	"fmt"
	fs "io/fs"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/voedger/voedger/pkg/appdef"
)

type FileSchemaAST struct {
	FileName string
	Ast      *SchemaAST
}

type PackageSchemaAST struct {
	Name                 string // Fill on the analysis stage, when the APPLICATION statement is found
	QualifiedPackageName string
	Ast                  *SchemaAST
}

type AppSchemaAST struct {
	// Application name
	Name string

	// key = Fully Qualified Name
	Packages map[string]*PackageSchemaAST
}

type IReadFS interface {
	fs.ReadDirFS
	fs.ReadFileFS
}

type Ident string

func (b *Ident) Capture(values []string) error {
	*b = Ident(strings.Trim(values[0], "\""))
	return nil
}

type IStatement interface {
	GetPos() *lexer.Position
	GetRawCommentBlocks() []string
	SetComments(comments []string)
	GetComments() []string
}

type INamedStatement interface {
	IStatement
	GetName() string
}
type IStatementCollection interface {
	Iterate(callback func(stmt interface{}))
}
type IExtensionStatement interface {
	SetEngineType(EngineType)
}

type SchemaAST struct {
	Imports    []ImportStmt    `parser:"@@? (';' @@)* ';'?"`
	Statements []RootStatement `parser:"@@? (';' @@)* ';'?"`
}

func (p *PackageSchemaAST) NewQName(name Ident) appdef.QName {
	return appdef.NewQName(string(p.Name), string(name))
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
	Name  string `parser:"'IMPORT' 'SCHEMA' @String"`
	Alias *Ident `parser:"('AS' @Ident)?"`
}

type RootStatement struct {
	// Only allowed in root
	Template *TemplateStmt `parser:"@@"`

	// Also allowed in root
	Role           *RoleStmt           `parser:"| @@"`
	Tag            *TagStmt            `parser:"| @@"`
	ExtEngine      *RootExtEngineStmt  `parser:"| @@"`
	Workspace      *WorkspaceStmt      `parser:"| @@"`
	AlterWorkspace *AlterWorkspaceStmt `parser:"| @@"`
	Table          *TableStmt          `parser:"| @@"`
	Type           *TypeStmt           `parser:"| @@"`
	Application    *ApplicationStmt    `parser:"| @@"`
	// Sequence  *sequenceStmt  `parser:"| @@"`

	stmt interface{}
}

type WorkspaceStatement struct {
	// Only allowed in workspace
	Rate         *RateStmt         `parser:"@@"`
	View         *ViewStmt         `parser:"| @@"`
	UseTable     *UseTableStmt     `parser:"| @@"`
	UseWorkspace *UseWorkspaceStmt `parser:"| @@"`

	// Also allowed in workspace
	Role      *RoleStmt               `parser:"| @@"`
	Tag       *TagStmt                `parser:"| @@"`
	ExtEngine *WorkspaceExtEngineStmt `parser:"| @@"`
	Workspace *WorkspaceStmt          `parser:"| @@"`
	Table     *TableStmt              `parser:"| @@"`
	Type      *TypeStmt               `parser:"| @@"`
	//Sequence  *sequenceStmt  `parser:"| @@"`
	Grant *GrantStmt `parser:"| @@"`

	stmt interface{}
}

type RootExtEngineStatement struct {
	Function *FunctionStmt `parser:"@@"`
	Storage  *StorageStmt  `parser:"| @@"`
	stmt     interface{}
}

type WorkspaceExtEngineStatement struct {
	Function  *FunctionStmt  `parser:"@@"`
	Projector *ProjectorStmt `parser:"| @@"`
	Command   *CommandStmt   `parser:"| @@"`
	Query     *QueryStmt     `parser:"| @@"`
	stmt      interface{}
}

type WorkspaceExtEngineStmt struct {
	Engine     EngineType                    `parser:"EXTENSIONENGINE @@"`
	Statements []WorkspaceExtEngineStatement `parser:"'(' @@? (';' @@)* ';'? ')'"`
}

func (s *WorkspaceExtEngineStmt) Iterate(callback func(stmt interface{})) {
	for i := 0; i < len(s.Statements); i++ {
		raw := &s.Statements[i]
		if raw.stmt == nil {
			raw.stmt = extractStatement(*raw)
			if es, ok := raw.stmt.(IExtensionStatement); ok {
				es.SetEngineType(s.Engine)
			}
		}
		callback(raw.stmt)
	}
}

type RootExtEngineStmt struct {
	Engine     EngineType               `parser:"EXTENSIONENGINE @@"`
	Statements []RootExtEngineStatement `parser:"'(' @@? (';' @@)* ';'? ')'"`
}

func (s *RootExtEngineStmt) Iterate(callback func(stmt interface{})) {
	for i := 0; i < len(s.Statements); i++ {
		raw := &s.Statements[i]
		if raw.stmt == nil {
			raw.stmt = extractStatement(*raw)
			if es, ok := raw.stmt.(IExtensionStatement); ok {
				es.SetEngineType(s.Engine)
			}
		}
		callback(raw.stmt)
	}
}

type UseStmt struct {
	Statement
	Name Ident `parser:"'USE' @Ident"`
}

type ApplicationStmt struct {
	Statement
	Name Ident     `parser:"'APPLICATION' @Ident '('"`
	Uses []UseStmt `parser:"@@? (';' @@)* ';'? ')'"`
}

type WorkspaceStmt struct {
	Statement
	Abstract   bool                 `parser:"(@'ABSTRACT' "`
	Alterable  bool                 `parser:"| @'ALTERABLE')?"`
	Pool       bool                 `parser:"@('POOL' 'OF')?"`
	Name       Ident                `parser:"'WORKSPACE' @Ident "`
	Inherits   []DefQName           `parser:"('INHERITS' @@ (',' @@)* )?"`
	A          int                  `parser:"'('"`
	Descriptor *WsDescriptorStmt    `parser:"('DESCRIPTOR' @@)?"`
	Statements []WorkspaceStatement `parser:"@@? (';' @@)* ';'? ')'"`
}

func (s WorkspaceStmt) GetName() string { return string(s.Name) }
func (s *WorkspaceStmt) Iterate(callback func(stmt interface{})) {
	if s.Descriptor != nil {
		callback(s.Descriptor)
	}
	for i := 0; i < len(s.Statements); i++ {
		raw := &s.Statements[i]
		if raw.stmt == nil {
			raw.stmt = extractStatement(*raw)
		}
		callback(raw.stmt)
	}
}

type AlterWorkspaceStmt struct {
	Statement
	Name       DefQName             `parser:"'ALTER' 'WORKSPACE' @@ "`
	A          int                  `parser:"'('"`
	Statements []WorkspaceStatement `parser:"@@? (';' @@)* ';'? ')'"`
}

func (s *AlterWorkspaceStmt) Iterate(callback func(stmt interface{})) {
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
	Name  Ident           `parser:"'TYPE' @Ident "`
	Items []TableItemExpr `parser:"'(' @@ (',' @@)* ')'"`
}

func (s TypeStmt) GetName() string { return string(s.Name) }

type WsDescriptorStmt struct {
	Statement
	Name  Ident           `parser:"@Ident?"`
	Items []TableItemExpr `parser:"'(' @@? (',' @@)* ')'"`
	_     int             `parser:"';'"`
}

func (s WsDescriptorStmt) GetName() string { return string(s.Name) }

type DefQName struct {
	Package Ident `parser:"(@Ident '.')?"`
	Name    Ident `parser:"@Ident"`
}

func (q DefQName) String() string {
	if q.Package == "" {
		return string(q.Name)
	}
	return fmt.Sprintf("%s.%s", q.Package, q.Name)

}

type TypeVarchar struct {
	MaxLen *uint16 `parser:"('varchar' | 'text') ( '(' @Int ')' )?"`
}

type TypeBytes struct {
	MaxLen *uint16 `parser:"'bytes' ( '(' @Int ')' )?"`
}

type VoidOrDataType struct {
	Void     bool           `parser:"( @'void'"`
	DataType *DataTypeOrDef `parser:"| @@)"`
}

type VoidOrDef struct {
	Void bool      `parser:"( @'void'"`
	Def  *DefQName `parser:"| @@)"`
}

type DataType struct {
	Varchar   *TypeVarchar `parser:"( @@"`
	Bytes     *TypeBytes   `parser:"| @@"`
	Int32     bool         `parser:"| @('int' | 'int32')"`
	Int64     bool         `parser:"| @'int64'"`
	Float32   bool         `parser:"| @('float' | 'float32')"`
	Float64   bool         `parser:"| @'float64'"`
	QName     bool         `parser:"| @'qname'"`
	Bool      bool         `parser:"| @'bool'"`
	Blob      bool         `parser:"| @'blob'"`
	Timestamp bool         `parser:"| @'timestamp'"`
	Currency  bool         `parser:"| @'currency' )"`
}

func (q DataType) String() (s string) {
	if q.Varchar != nil {
		if q.Varchar.MaxLen != nil {
			return fmt.Sprintf("varchar[%d]", *q.Varchar.MaxLen)
		}
		return fmt.Sprintf("varchar[%d]", appdef.DefaultFieldMaxLength)
	} else if q.Int32 {
		return "int32"
	} else if q.Int64 {
		return "int64"
	} else if q.Float32 {
		return "int32"
	} else if q.Float64 {
		return "int64"
	} else if q.QName {
		return "qname"
	} else if q.Bool {
		return "bool"
	} else if q.Bytes != nil {
		if q.Bytes.MaxLen != nil {
			return fmt.Sprintf("bytes[%d]", *q.Bytes.MaxLen)
		}
		return fmt.Sprintf("bytes[%d]", appdef.DefaultFieldMaxLength)
	} else if q.Blob {
		return "blob"
	} else if q.Timestamp {
		return "timestamp"
	} else if q.Currency {
		return "currency"
	}

	return "?"
}

type DataTypeOrDef struct {
	DataType *DataType `parser:"( @@"`
	Def      *DefQName `parser:"| @@ )"`
}

func (q DataTypeOrDef) String() (s string) {
	if q.DataType != nil {
		return q.DataType.String()
	}
	return q.Def.String()
}

type Statement struct {
	Pos              lexer.Position
	RawCommentBlocks []string `parser:"@PreStmtComment*"`
	Comments         []string // will be set after 1st pass
}

func (s *Statement) GetPos() *lexer.Position {
	return &s.Pos
}

func (s *Statement) GetRawCommentBlocks() []string {
	return s.RawCommentBlocks
}

func (s *Statement) GetComments() []string {
	return s.Comments
}

func (s *Statement) SetComments(comments []string) {
	s.Comments = comments
}

type StorageKey struct {
	Storage DefQName  `parser:"@@"`
	Entity  *DefQName `parser:"( @@ )?"`
}

/*
	ProjectorStmt: asdas

zczxczxc
*/
type ProjectorStmt struct {
	Statement
	Sync      bool                `parser:"@'SYNC'?"`
	Name      Ident               `parser:"'PROJECTOR' @Ident"`
	CUDEvents *ProjectorCUDEvents `parser:"('AFTER' @@)?"`
	On        []DefQName          `parser:"'ON' (('(' @@ (',' @@)* ')') | @@)!"`
	State     []StorageKey        `parser:"('STATE'   '(' @@ (',' @@)* ')' )?"`
	Intents   []StorageKey        `parser:"('INTENTS' '(' @@ (',' @@)* ')' )?"`
	Engine    EngineType          // Initialized with 1st pass
}

func (s *ProjectorStmt) GetName() string            { return string(s.Name) }
func (s *ProjectorStmt) SetEngineType(e EngineType) { s.Engine = e }

type ProjectorCUDEvents struct {
	Insert     bool `parser:"  @(('INSERT' ('OR' 'UPDATE')?) | ('UPDATE' 'OR' 'INSERT'))"`
	Update     bool `parser:"| @(('UPDATE' ('OR' 'INSERT')?) | ('INSERT' 'OR' 'UPDATE'))"`
	Activate   bool `parser:"| @(('ACTIVATE' ('OR' 'DEACTIVATE')?) | ('DEACTIVATE' 'OR' 'ACTIVATE'))"`
	Deactivate bool `parser:"| @(('DEACTIVATE' ('OR' 'ACTIVATE')?) | ('ACTIVATE' 'OR' 'DEACTIVATE'))"`
}

/*type ProjectorOn struct {
	CommandArgument bool `parser:"@('COMMAND' 'ARGUMENT')"`
	Command         bool `parser:"| @('COMMAND')"`
	Insert          bool `parser:"| @(('INSERT' ('OR' 'UPDATE')?) | ('UPDATE' 'OR' 'INSERT'))"`
	Update          bool `parser:"| @(('UPDATE' ('OR' 'INSERT')?) | ('INSERT' 'OR' 'UPDATE'))"`
	Activate        bool `parser:"| @(('ACTIVATE' ('OR' 'DEACTIVATE')?) | ('DEACTIVATE' 'OR' 'ACTIVATE'))"`
	Deactivate      bool `parser:"| @(('DEACTIVATE' ('OR' 'ACTIVATE')?) | ('ACTIVATE' 'OR' 'DEACTIVATE'))"`
}*/

type TemplateStmt struct {
	Statement
	Name      Ident    `parser:"'TEMPLATE' @Ident 'OF' 'WORKSPACE'" `
	Workspace DefQName `parser:"@@"`
	Source    Ident    `parser:"'SOURCE' @Ident"`
}

func (s TemplateStmt) GetName() string { return string(s.Name) }

type RoleStmt struct {
	Statement
	Name Ident `parser:"'ROLE' @Ident"`
}

func (s RoleStmt) GetName() string { return string(s.Name) }

type TagStmt struct {
	Statement
	Name Ident `parser:"'TAG' @Ident"`
}

func (s TagStmt) GetName() string { return string(s.Name) }

type UseTableStmt struct {
	Statement
	Table DefQName `parser:"'USE' 'TABLE' @@"`
	// TODO: Use all tables from package
}

type UseWorkspaceStmt struct {
	Statement
	Workspace Ident `parser:"'USE' 'WORKSPACE' @Ident"`
}

/*type sequenceStmt struct {
	Name        Ident `parser:"'SEQUENCE' @Ident"`
	Type        Ident `parser:"@Ident"`
	StartWith   *int   `parser:"(('START' 'WITH' @Number)"`
	MinValue    *int   `parser:"| ('MINVALUE' @Number)"`
	MaxValue    *int   `parser:"| ('MAXVALUE' @Number)"`
	IncrementBy *int   `parser:"| ('INCREMENT' 'BY' @Number) )*"`
}*/

type RateStmt struct {
	Statement
	Name   Ident  `parser:"'RATE' @Ident"`
	Amount int    `parser:"@Int"`
	Per    string `parser:"'PER' @('SECOND' | 'MINUTE' | 'HOUR' | 'DAY' | 'YEAR')"`
	PerIP  bool   `parser:"(@('PER' 'IP'))?"`
}

func (s RateStmt) GetName() string { return string(s.Name) }

// TODO: better Grant syntax
type GrantStmt struct {
	Statement
	Grants []string `parser:"'GRANT' @('ALL' | 'EXECUTE' | 'SELECT' | 'INSERT' | 'UPDATE') (','  @('ALL' | 'EXECUTE' | 'SELECT' | 'INSERT' | 'UPDATE'))*"`
	On     string   `parser:"'ON' @('TABLE' | ('ALL' 'TABLES' 'WITH' 'TAG') | 'COMMAND' | ('ALL' 'COMMANDS' 'WITH' 'TAG') | 'QUERY' | ('ALL' 'QUERIES' 'WITH' 'TAG'))"`
	Target DefQName `parser:"@@"`
	To     Ident    `parser:"'TO' @Ident"`
}

type StorageStmt struct {
	Statement
	Name         Ident       `parser:"'STORAGE' @Ident"`
	Ops          []StorageOp `parser:"'(' @@ (',' @@)* ')'"`
	EntityRecord bool        `parser:"@('ENTITY' 'RECORD')?"`
	EntityView   bool        `parser:"@('ENTITY' 'VIEW')?"`
}

func (s StorageStmt) GetName() string { return string(s.Name) }

type StorageOp struct {
	Get      bool           `parser:"( @'GET'"`
	GetBatch bool           `parser:"| @'GETBATCH'"`
	Read     bool           `parser:"| @'READ'"`
	Insert   bool           `parser:"| @'INSERT'"`
	Update   bool           `parser:"| @'UPDATE')"`
	Scope    []StorageScope `parser:"'SCOPE' '(' @@ (',' @@)* ')'"`
}

type StorageScope struct {
	Commands   bool `parser:" ( @'COMMANDS'"`
	Queries    bool `parser:" | @'QUERIES'"`
	Projectors bool `parser:" | @'PROJECTORS')"`
}

type FunctionStmt struct {
	Statement
	Name    Ident           `parser:"'FUNCTION' @Ident"`
	Params  []FunctionParam `parser:"'(' @@? (',' @@)* ')'"`
	Returns DataTypeOrDef   `parser:"'RETURNS' @@"`
	Engine  EngineType      // Initialized with 1st pass
}

func (s *FunctionStmt) GetName() string            { return string(s.Name) }
func (s *FunctionStmt) SetEngineType(e EngineType) { s.Engine = e }

type CommandStmt struct {
	Statement
	Name        Ident      `parser:"'COMMAND' @Ident"`
	Arg         *VoidOrDef `parser:"('(' @@? "`
	UnloggedArg *VoidOrDef `parser:"(','? UNLOGGED @@)? ')')?"`
	Returns     *VoidOrDef `parser:"('RETURNS' @@)?"`
	With        []WithItem `parser:"('WITH' @@ (',' @@)* )?"`
	Engine      EngineType // Initialized with 1st pass
}

func (s *CommandStmt) GetName() string            { return string(s.Name) }
func (s *CommandStmt) SetEngineType(e EngineType) { s.Engine = e }

type WithItem struct {
	Comment *string    `parser:"('Comment' '=' @String)"`
	Tags    []DefQName `parser:"| ('Tags' '=' '(' @@ (',' @@)* ')')"`
	Rate    *DefQName  `parser:"| ('Rate' '=' @@)"`
}

type AnyOrVoidOrDef struct {
	Any  bool      `parser:"@'ANY'"`
	Void bool      `parser:"| @'void'"`
	Def  *DefQName `parser:"| @@"`
}

type QueryStmt struct {
	Statement
	Name    Ident          `parser:"'QUERY' @Ident"`
	Arg     *VoidOrDef     `parser:"('(' @@? ')')?"`
	Returns AnyOrVoidOrDef `parser:"'RETURNS' @@"`
	With    []WithItem     `parser:"('WITH' @@ (',' @@)* )?"`
	Engine  EngineType     // Initialized with 1st pass
}

func (s *QueryStmt) GetName() string            { return string(s.Name) }
func (s *QueryStmt) SetEngineType(e EngineType) { s.Engine = e }

type EngineType struct {
	WASM    bool `parser:"@'WASM'"`
	Builtin bool `parser:"| @'BUILTIN'"`
}

type FunctionParam struct {
	NamedParam       *NamedParam    `parser:"@@"`
	UnnamedParamType *DataTypeOrDef `parser:"| @@"`
}

type NamedParam struct {
	Name Ident         `parser:"@Ident"`
	Type DataTypeOrDef `parser:"@@"`
}

type TableStmt struct {
	Statement
	Abstract     bool            `parser:"@'ABSTRACT'? 'TABLE'"`
	Name         Ident           `parser:"@Ident"`
	Inherits     *DefQName       `parser:"('INHERITS' @@)?"`
	Items        []TableItemExpr `parser:"'(' @@? (',' @@)* ')'"`
	With         []WithItem      `parser:"('WITH' @@ (',' @@)* )?"`
	tableDefKind appdef.DefKind  // filled on the analysis stage
	singletone   bool
}

func (s *TableStmt) GetName() string { return string(s.Name) }
func (s *TableStmt) Iterate(callback func(stmt interface{})) {
	for i := 0; i < len(s.Items); i++ {
		item := &s.Items[i]
		if item.Field != nil {
			callback(item.Field)
		}
	}
}

type NestedTableStmt struct {
	Pos   lexer.Position
	Name  Ident     `parser:"@Ident"`
	Table TableStmt `parser:"@@"`
}

type FieldSetItem struct {
	Pos  lexer.Position
	Type DefQName `parser:"@@"`
}

type TableItemExpr struct {
	NestedTable *NestedTableStmt `parser:"@@"`
	Constraint  *TableConstraint `parser:"| @@"`
	RefField    *RefFieldExpr    `parser:"| @@"`
	Field       *FieldExpr       `parser:"| @@"`
	FieldSet    *FieldSetItem    `parser:"| @@"`
}

type TableConstraint struct {
	Pos            lexer.Position
	ConstraintName Ident            `parser:"('CONSTRAINT' @Ident)?"`
	UniqueField    *UniqueFieldExpr `parser:"(@@"`
	//	Unique         *UniqueExpr      `parser:"(@@"` // TODO: not supported by kernel yet
	Check *TableCheckExpr `parser:"| @@)"`
}

type TableCheckExpr struct {
	Expression Expression `parser:"'CHECK' '(' @@ ')'"`
}

type UniqueFieldExpr struct {
	Field Ident `parser:"'UNIQUEFIELD' @Ident"`
}

type UniqueExpr struct {
	Fields []Ident `parser:"'UNIQUE' '(' @Ident (',' @Ident)* ')'"`
}

type RefFieldExpr struct {
	Pos     lexer.Position
	Name    Ident      `parser:"@Ident"`
	RefDocs []DefQName `parser:"'ref' ('(' @@ (',' @@)* ')')?"`
	NotNull bool       `parser:"@(NOTNULL)?"`
}

type FieldExpr struct {
	Statement
	Name               Ident         `parser:"@Ident"`
	Type               DataTypeOrDef `parser:"@@"`
	NotNull            bool          `parser:"@(NOTNULL)?"`
	Verifiable         bool          `parser:"@('VERIFIABLE')?"`
	DefaultIntValue    *int          `parser:"('DEFAULT' @Int)?"`
	DefaultStringValue *string       `parser:"('DEFAULT' @String)?"`
	//	DefaultNextVal     *string       `parser:"(DEFAULTNEXTVAL  '(' @String ')')?"`
	CheckRegexp     *string     `parser:"('CHECK' @String)?"`
	CheckExpression *Expression `parser:"('CHECK' '(' @@ ')')? "`
}

type ViewStmt struct {
	Statement
	Name     Ident          `parser:"'VIEW' @Ident"`
	Items    []ViewItemExpr `parser:"'(' @@? (',' @@)* ')'"`
	ResultOf DefQName       `parser:"'AS' 'RESULT' 'OF' @@"`
	pkRef    *PrimaryKeyExpr
}

func (s *ViewStmt) Iterate(callback func(stmt interface{})) {
	for i := 0; i < len(s.Items); i++ {
		item := &s.Items[i]
		if item.Field != nil {
			callback(item.Field)
		} else if item.RefField != nil {
			callback(item.RefField)
		}
	}
}

type ViewItemExpr struct {
	Pos        lexer.Position
	PrimaryKey *PrimaryKeyExpr `parser:"(PRIMARYKEY '(' @@ ')')"`
	RefField   *ViewRefField   `parser:"| @@"`
	Field      *ViewField      `parser:"| @@"`
}

type PrimaryKeyExpr struct {
	Pos                     lexer.Position
	PartitionKeyFields      []Ident `parser:"('(' @Ident (',' @Ident)* ')')?"`
	ClusteringColumnsFields []Ident `parser:"(','? @Ident (',' @Ident)*)?"`
}

func (s ViewStmt) GetName() string { return string(s.Name) }

type ViewRefField struct {
	Statement
	Name    Ident      `parser:"@Ident"`
	RefDocs []DefQName `parser:"'ref' ('(' @@ (',' @@)* ')')?"`
	NotNull bool       `parser:"@(NOTNULL)?"`
}

type ViewField struct {
	Statement
	Name    Ident    `parser:"@Ident"`
	Type    DataType `parser:"@@"`
	NotNull bool     `parser:"@(NOTNULL)?"`
}

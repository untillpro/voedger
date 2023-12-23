/*
* Copyright (c) 2023-present unTill Pro, Ltd.
* @author Michael Saigachenko
 */

package parser

import (
	"fmt"
	"io/fs"
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

type PackageFS struct {
	QualifiedPackageName string
	FS                   IReadFS
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
	Rate           *RateStmt           `parser:"| @@"`
	Limit          *LimitStmt          `parser:"| @@"`
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
	Limit     *LimitStmt              `parser:"| @@"`
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
	Items []TableItemExpr `parser:"'(' @@? (',' @@)* ')'"`
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
	MaxLen *uint64 `parser:"('varchar' | 'text') ( '(' @Int ')' )?"`
}

type TypeBytes struct {
	MaxLen *uint64 `parser:"'bytes' ( '(' @Int ')' )?"`
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
	Record    bool         `parser:"| @'record'"`
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

// not suppored by kernel yet:
// type DataTypeOrDefArray struct {
// 	Unbounded bool `parser:"@'[]' |"`
// 	MaxOccurs int  `parser:"'[' @Int ']'"`
// }

type DataTypeOrDef struct {
	DataType *DataType `parser:"( @@"`
	Def      *DefQName `parser:"| @@ )"`
	// Array    *DataTypeOrDefArray `parser:"@@?"` not suppored by kernel yet
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

type ProjectorStorage struct {
	Storage  DefQName   `parser:"@@"`
	Entities []DefQName `parser:"( '(' @@ (',' @@)* ')')?"`
}

type ProjectionTableAction struct {
	Insert     bool `parser:"@'INSERT'"`
	Update     bool `parser:"| @'UPDATE'"`
	Activate   bool `parser:"| @'ACTIVATE'"`
	Deactivate bool `parser:"| @'DEACTIVATE'"`
}

type ProjectorCommandAction struct {
	Execute   bool `parser:"@'EXECUTE'"`
	WithParam bool `parser:"@('WITH' 'PARAM')?"`
}

type ProjectorTrigger struct {
	ExecuteAction *ProjectorCommandAction `parser:"'AFTER' (@@"`
	TableActions  []ProjectionTableAction `parser:"| (@@ ('OR' @@)* ))"`
	QNames        []DefQName              `parser:"'ON' (('(' @@ (',' @@)* ')') | @@)!"`
}

type ProjectorStmt struct {
	Statement
	Sync            bool               `parser:"@'SYNC'?"`
	Name            Ident              `parser:"'PROJECTOR' @Ident"`
	Triggers        []ProjectorTrigger `parser:"@@ ('OR' @@)*"`
	State           []ProjectorStorage `parser:"('STATE'   '(' @@ (',' @@)* ')' )?"`
	Intents         []ProjectorStorage `parser:"('INTENTS' '(' @@ (',' @@)* ')' )?"`
	IncludingErrors bool               `parser:"('INCLUDING' 'ERRORS')?"`
	Engine          EngineType         // Initialized with 1st pass
}

func (s *ProjectorStmt) GetName() string            { return string(s.Name) }
func (s *ProjectorStmt) SetEngineType(e EngineType) { s.Engine = e }

// func (t *ProjectorCUDEvents) insert() bool {
// 	for i := 0; i < len(t.Actions); i++ {
// 		if t.Actions[i] == "INSERT" {
// 			return true
// 		}
// 	}
// 	return false
// }

func (t *ProjectorTrigger) update() bool {
	for i := 0; i < len(t.TableActions); i++ {
		if t.TableActions[i].Update {
			return true
		}
	}
	return false
}

func (t *ProjectorTrigger) insert() bool {
	for i := 0; i < len(t.TableActions); i++ {
		if t.TableActions[i].Insert {
			return true
		}
	}
	return false
}

func (t *ProjectorTrigger) activate() bool {
	for i := 0; i < len(t.TableActions); i++ {
		if t.TableActions[i].Activate {
			return true
		}
	}
	return false
}

func (t *ProjectorTrigger) deactivate() bool {
	for i := 0; i < len(t.TableActions); i++ {
		if t.TableActions[i].Deactivate {
			return true
		}
	}
	return false
}

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

// type UseAllTables struct {
// 	FromPackage Ident `parser:"(@Ident '.')? '*'"`
// }

type UseTableStmt struct {
	Statement
	Package   Ident  `parser:"'USE' 'TABLE' (@Ident '.')?"`
	AllTables bool   `parser:"(@'*'"`
	TableName *Ident `parser:"| @Ident)"`
	// AllTables *UseAllTables `parser:"'USE' 'TABLE' (@@"`
	// Table     *DefQName     `parser:"| @@)"`
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

type RateValueTimeUnit struct {
	Second bool `parser:"@('SECOND' | 'SECONDS')"`
	Minute bool `parser:"| @('MINUTE' | 'MINUTES')"`
	Hour   bool `parser:"| @('HOUR' | 'HOURS')"`
	Day    bool `parser:"| @('DAY' | 'DAYS')"`
	Year   bool `parser:"| @('YEAR' | 'YEARS')"`
}

type RateValue struct {
	Count           int               `parser:"@Int 'PER'"`
	TimeUnitAmounts *int              `parser:"@Int?"`
	TimeUnit        RateValueTimeUnit `parser:"@@"`
}

type RateObjectScope struct {
	PerAppPartition bool `parser:"  @('PER' 'APP' 'PARTITION')"`
	PerWorkspace    bool `parser:" | @('PER' 'WORKSPACE')"`
}

type RateSubjectScope struct {
	PerUser bool `parser:"@('PER' 'USER')"`
	PerIp   bool `parser:" | @('PER' 'IP')"`
}

type RateStmt struct {
	Statement
	Name         Ident             `parser:"'RATE' @Ident"`
	Value        RateValue         `parser:"@@"`
	ObjectScope  *RateObjectScope  `parser:"@@?"`
	SubjectScope *RateSubjectScope `parser:"@@?"`
}

func (s RateStmt) GetName() string { return string(s.Name) }

type LimitAction struct {
	Command              *DefQName `parser:"(EXECUTEONCOMMAND @@)"`
	AllCommandsWithTag   *DefQName `parser:"| (EXECUTEONALLCOMMANDSWITHTAG @@)"`
	AllCommands          bool      `parser:"| @EXECUTEONALLCOMMANDS"`
	Query                *DefQName `parser:"| (EXECUTEONQUERY @@)"`
	AllQueriesWithTag    *DefQName `parser:"| (EXECUTEONALLQUERIESWITHTAG @@)"`
	AllQueries           bool      `parser:"| @EXECUTEONALLQUERIES"`
	Workspace            *DefQName `parser:"| (INSERTONWORKSPACE @@)"`
	AllWorkspacesWithTag *DefQName `parser:"| (INSERTONALLWORKSPACESWITHTAG @@)"`
}

type LimitStmt struct {
	Statement
	Name     Ident       `parser:"'LIMIT' @Ident"`
	Action   LimitAction `parser:"@@"`
	RateName DefQName    `parser:"'WITH' 'RATE' @@"`
}

func (s LimitStmt) GetName() string { return string(s.Name) }

type GrantTableAction struct {
	Select  bool     `parser:"(@'SELECT'"`
	Insert  bool     `parser:"| @'INSERT'"`
	Update  bool     `parser:"| @'UPDATE')"`
	Columns []string `parser:"( '(' @Ident (',' @Ident)* ')' )?"`
}

type GrantTableAll struct {
	AllTables bool     `parser:"@'ALL'"`
	Columns   []string `parser:"( '(' @Ident (',' @Ident)* ')' )?"`
}

type GrantTableActions struct {
	All   *GrantTableAll     `parser:"@@ | "`
	Items []GrantTableAction `parser:"(@@ (',' @@)*)"`
}

type GrantTable struct {
	Actions          GrantTableActions `parser:"@@"`
	OneTable         bool              `parser:"'ON' (@'TABLE'"`
	AllTablesWithTag bool              `parser:"| @('ALL' 'TABLES' 'WITH' 'TAG'))"`
}

type GrantStmt struct {
	Statement
	Command              bool        `parser:"'GRANT' ( @EXECUTEONCOMMAND"`
	AllCommandsWithTag   bool        `parser:"| @EXECUTEONALLCOMMANDSWITHTAG"`
	Query                bool        `parser:"| @EXECUTEONQUERY"`
	AllQueriesWithTag    bool        `parser:"| @EXECUTEONALLQUERIESWITHTAG"`
	Workspace            bool        `parser:"| @INSERTONWORKSPACE"`
	AllWorkspacesWithTag bool        `parser:"| @INSERTONALLWORKSPACESWITHTAG"`
	Table                *GrantTable `parser:"| @@)"`

	On DefQName `parser:"@@"`
	To DefQName `parser:"'TO' @@"`
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
	Name          Ident           `parser:"'COMMAND' @Ident"`
	Param         *AnyOrVoidOrDef `parser:"('(' @@? "`
	UnloggedParam *AnyOrVoidOrDef `parser:"(','? UNLOGGED @@)? ')')?"`
	Returns       *AnyOrVoidOrDef `parser:"('RETURNS' @@)?"`
	With          []WithItem      `parser:"('WITH' @@ (',' @@)* )?"`
	Engine        EngineType      // Initialized with 1st pass
}

func (s *CommandStmt) GetName() string            { return string(s.Name) }
func (s *CommandStmt) SetEngineType(e EngineType) { s.Engine = e }

type WithItem struct {
	Comment *string    `parser:"('Comment' '=' @String)"`
	Tags    []DefQName `parser:"| ('Tags' '=' '(' @@ (',' @@)* ')')"`
	Rate    *DefQName  `parser:"| ('Rate' '=' @@)"`
}

type AnyOrVoidOrDef struct {
	Any  bool      `parser:"@'any'"`
	Void bool      `parser:"| @'void'"`
	Def  *DefQName `parser:"| @@"`
}

type QueryStmt struct {
	Statement
	Name    Ident           `parser:"'QUERY' @Ident"`
	Param   *AnyOrVoidOrDef `parser:"('(' @@? ')')?"`
	Returns AnyOrVoidOrDef  `parser:"'RETURNS' @@"`
	With    []WithItem      `parser:"('WITH' @@ (',' @@)* )?"`
	Engine  EngineType      // Initialized with 1st pass
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
	Abstract      bool            `parser:"@'ABSTRACT'? 'TABLE'"`
	Name          Ident           `parser:"@Ident"`
	Inherits      *DefQName       `parser:"('INHERITS' @@)?"`
	Items         []TableItemExpr `parser:"'(' @@? (',' @@)* ')'"`
	With          []WithItem      `parser:"('WITH' @@ (',' @@)* )?"`
	tableTypeKind appdef.TypeKind // filled on the analysis stage
	singletone    bool
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
	ConstraintName Ident `parser:"('CONSTRAINT' @Ident)?"`
	//	Unique         *UniqueExpr      `parser:"(@@"` // TODO: not supported by kernel yet
	Check *TableCheckExpr `parser:"| @@)"`
}

type TableCheckExpr struct {
	Expression Expression `parser:"'CHECK' '(' @@ ')'"`
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

// Returns view item with field by field name
func (s ViewStmt) Field(fieldName Ident) *ViewItemExpr {
	for i := 0; i < len(s.Items); i++ {
		item := &s.Items[i]
		if item.FieldName() == fieldName {
			return item
		}
	}
	return nil
}

// Iterate view partition fields
func (s ViewStmt) PartitionFields(callback func(f *ViewItemExpr)) {
	for i := 0; i < len(s.pkRef.PartitionKeyFields); i++ {
		if f := s.Field(s.pkRef.PartitionKeyFields[i]); f != nil {
			callback(f)
		}
	}
}

// Iterate view clustering columns
func (s ViewStmt) ClusteringColumns(callback func(f *ViewItemExpr)) {
	for i := 0; i < len(s.pkRef.ClusteringColumnsFields); i++ {
		if f := s.Field(s.pkRef.ClusteringColumnsFields[i]); f != nil {
			callback(f)
		}
	}
}

// Iterate view value fields
func (s ViewStmt) ValueFields(callback func(f *ViewItemExpr)) {
	for i := 0; i < len(s.Items); i++ {
		f := &s.Items[i]
		if n := f.FieldName(); len(n) > 0 {
			if !contains(s.pkRef.PartitionKeyFields, n) && !contains(s.pkRef.ClusteringColumnsFields, n) {
				callback(f)
			}
		}
	}
}

type ViewItemExpr struct {
	Pos        lexer.Position
	PrimaryKey *PrimaryKeyExpr `parser:"(PRIMARYKEY '(' @@ ')')"`
	RefField   *ViewRefField   `parser:"| @@"`
	Field      *ViewField      `parser:"| @@"`
}

// Returns field name
func (i ViewItemExpr) FieldName() Ident {
	if i.Field != nil {
		return i.Field.Name
	}
	if i.RefField != nil {
		return i.RefField.Name
	}
	return ""
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

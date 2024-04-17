/*
* Copyright (c) 2023-present unTill Pro, Ltd.
* @author Michael Saigachenko
 */

package parser

import (
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"

	"github.com/voedger/voedger/pkg/appdef"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

type FileSchemaAST struct {
	FileName string
	Ast      *SchemaAST
}

type PackageSchemaAST struct {
	Name string // Fill on the analysis stage, when the APPLICATION statement is found
	Path string
	Ast  *SchemaAST
}

type AppSchemaAST struct {
	// Application name
	Name string

	// key = Fully Qualified Name
	Packages map[string]*PackageSchemaAST

	LocalNameToFullPath map[string]string
}

type PackageFS struct {
	Path string
	FS   coreutils.IReadFS
}

type Ident string

func (b *Ident) Capture(values []string) error {
	*b = Ident(strings.Trim(values[0], "\""))
	return nil
}

type Identifier struct {
	Pos   lexer.Position
	Value Ident `parser:"@Ident"`
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
	return appdef.NewQName(p.Name, string(name))
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
	Declare        *DeclareStmt        `parser:"| @@"`
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
	// Sequence  *sequenceStmt  `parser:"| @@"`
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

type DeclareStmt struct {
	Statement
	Name         Ident  `parser:"'DECLARE' @Ident"`
	DataType     string `parser:"@('int' | 'int32')"`
	DefaultValue *int   `parser:"'DEFAULT' @Int"`
}

func (s DeclareStmt) GetName() string { return string(s.Name) }

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
	Inherits   []DefQName           `parser:"('INHERITS' @@ (',' @@)* )? '('"`
	Descriptor *WsDescriptorStmt    `parser:"('DESCRIPTOR' @@)?"`
	Statements []WorkspaceStatement `parser:"@@? (';' @@)* ';'? ')'"`

	// filled on the analysis stage
	qNames []appdef.QName
}

func (s *WorkspaceStmt) containsQName(qName appdef.QName) bool {
	for i := 0; i < len(s.qNames); i++ {
		if s.qNames[i] == qName {
			return true
		}
	}
	return false
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

	alteredWorkspace    *WorkspaceStmt    // filled on the analysis stage
	alteredWorkspacePkg *PackageSchemaAST // filled on the analysis stage
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
	Pos     lexer.Position
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
	Pos    lexer.Position
	MaxLen *uint64 `parser:"('varchar' | 'text') ( '(' @Int ')' )?"`
}

type TypeBytes struct {
	Pos    lexer.Position
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

	// filled on the analysis stage
	qName     appdef.QName
	tableStmt *TableStmt        // when qName is table
	tablePkg  *PackageSchemaAST // when qName is table
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

type StateStorage struct {
	Storage  DefQName   `parser:"@@"`
	Entities []DefQName `parser:"( '(' @@ (',' @@)* ')')?"`

	// filled on the analysis stage
	storageQName appdef.QName
	entityQNames []appdef.QName
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
	CronSchedule  *string                 `parser:"('CRON' @String) | ("`
	ExecuteAction *ProjectorCommandAction `parser:"'AFTER' (@@"`
	TableActions  []ProjectionTableAction `parser:"| (@@ ('OR' @@)* ))"`
	QNames        []DefQName              `parser:"'ON' (('(' @@ (',' @@)* ')') | @@)!)"`

	qNames []appdef.QName // filled on the analysis stage
}

type ProjectorStmt struct {
	Statement
	Sync            bool               `parser:"@'SYNC'?"`
	Name            Ident              `parser:"'PROJECTOR' @Ident"`
	Triggers        []ProjectorTrigger `parser:"@@ ('OR' @@)*"`
	State           []StateStorage     `parser:"('STATE'   '(' @@ (',' @@)* ')' )?"`
	Intents         []StateStorage     `parser:"('INTENTS' '(' @@ (',' @@)* ')' )?"`
	IncludingErrors bool               `parser:"@('INCLUDING' 'ERRORS')?"`
	Engine          EngineType         // Initialized with 1st pass
}

func (s *ProjectorStmt) GetName() string            { return string(s.Name) }
func (s *ProjectorStmt) SetEngineType(e EngineType) { s.Engine = e }

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

type UseTableStmt struct {
	Statement

	Package   *Identifier `parser:"'USE' 'TABLE' (@@ '.')?"`
	AllTables bool        `parser:"(@'*'"`
	TableName *Identifier `parser:"| @@)"`

	qNames []appdef.QName // filled on the analysis stage
}

type UseWorkspaceStmt struct {
	Statement
	Workspace Identifier `parser:"'USE' 'WORKSPACE' @@"`

	qName appdef.QName // filled on the analysis stage
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
	Count           *int              `parser:"(@Int"`
	Variable        *DefQName         `parser:"| @@) 'PER'"`
	TimeUnitAmounts *int              `parser:"@Int?"`
	TimeUnit        RateValueTimeUnit `parser:"@@"`
	variable        appdef.QName      // filled on the analysis stage
	declare         *DeclareStmt      // filled on the analysis stage
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
	Pos        lexer.Position
	Table      *DefQName `parser:"(ONTABLE @@)"`
	Command    *DefQName `parser:"| ('ON' 'COMMAND' @@)"`
	Query      *DefQName `parser:"| ('ON' 'QUERY' @@)"`
	Tag        *DefQName `parser:"| ('ON' 'TAG' @@)"`
	Everything bool      `parser:"| @('ON' 'EVERYTHING')"`
}

type LimitStmt struct {
	Statement
	Name     Ident       `parser:"'LIMIT' @Ident"`
	Action   LimitAction `parser:"@@"`
	RateName DefQName    `parser:"'WITH' 'RATE' @@"`
}

func (s LimitStmt) GetName() string { return string(s.Name) }

type GrantTableAction struct {
	Pos     lexer.Position
	Select  bool         `parser:"(@'SELECT'"`
	Insert  bool         `parser:"| @'INSERT'"`
	Update  bool         `parser:"| @'UPDATE')"`
	Columns []Identifier `parser:"( '(' @@ (',' @@)* ')' )?"`
}

type GrantAllTablesWithTagAction struct {
	Pos    lexer.Position
	Select bool `parser:"@'SELECT'"`
	Insert bool `parser:"| @'INSERT'"`
	Update bool `parser:"| @'UPDATE'"`
}

type GrantTableAll struct {
	Pos      lexer.Position
	GrantAll bool         `parser:"@'ALL'"`
	Columns  []Identifier `parser:"( '(' @@ (',' @@)* ')' )?"`
}

type GrantTableActions struct {
	Pos      lexer.Position
	GrantAll *GrantTableAll     `parser:"(@@ | "`
	Items    []GrantTableAction `parser:"(@@ (',' @@)*))"`
}

type GrantAllTablesWithTagActions struct {
	Pos      lexer.Position
	GrantAll bool                          `parser:"@'ALL' | "`
	Items    []GrantAllTablesWithTagAction `parser:"(@@ (',' @@)*)"`
}

type GrantStmt struct {
	Statement
	Command              bool                          `parser:"'GRANT' ( @INSERTONCOMMAND"`
	AllCommandsWithTag   bool                          `parser:"| @INSERTONALLCOMMANDSWITHTAG"`
	Query                bool                          `parser:"| @SELECTONQUERY"`
	AllQueriesWithTag    bool                          `parser:"| @SELECTONALLQUERIESWITHTAG"`
	Workspace            bool                          `parser:"| @INSERTONWORKSPACE"`
	AllWorkspacesWithTag bool                          `parser:"| @INSERTONALLWORKSPACESWITHTAG"`
	AllTablesWithTag     *GrantAllTablesWithTagActions `parser:"| (@@ ONALLTABLESWITHTAG)"`
	Table                *GrantTableActions            `parser:"| (@@ ONTABLE) )"`

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
	State         []StateStorage  `parser:"('STATE'   '(' @@ (',' @@)* ')' )?"`
	Intents       []StateStorage  `parser:"('INTENTS' '(' @@ (',' @@)* ')' )?"`
	Returns       *AnyOrVoidOrDef `parser:"('RETURNS' @@)?"`
	With          []WithItem      `parser:"('WITH' @@ (',' @@)* )?"`
	Engine        EngineType      // Initialized with 1st pass
}

func (s *CommandStmt) GetName() string            { return string(s.Name) }
func (s *CommandStmt) SetEngineType(e EngineType) { s.Engine = e }

type WithItem struct {
	Comment *string    `parser:"('Comment' '=' @String)"`
	Tags    []DefQName `parser:"| ('Tags' '=' '(' @@ (',' @@)* ')')"`
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
	State   []StateStorage  `parser:"('STATE'   '(' @@ (',' @@)* ')' )?"`
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
	Abstract      bool            `parser:"@'ABSTRACT'?'TABLE'"`
	Name          Ident           `parser:"@Ident"`
	Inherits      *DefQName       `parser:"('INHERITS' @@)?"`
	Items         []TableItemExpr `parser:"'(' @@? (',' @@)* ')'"`
	With          []WithItem      `parser:"('WITH' @@ (',' @@)* )?"`
	tableTypeKind appdef.TypeKind // filled on the analysis stage
	singleton     bool
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
	Statement
	ConstraintName Ident            `parser:"('CONSTRAINT' @Ident)?"`
	UniqueField    *UniqueFieldExpr `parser:"(@@"`
	Unique         *UniqueExpr      `parser:"| @@"`
	Check          *TableCheckExpr  `parser:"| @@)"`
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

type CheckRegExp struct {
	Pos    lexer.Position
	Regexp string `parser:"@String"`
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
	CheckRegexp     *CheckRegExp `parser:"('CHECK' @@ )?"`
	CheckExpression *Expression  `parser:"('CHECK' '(' @@ ')')? "`
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
		if f := s.Field(s.pkRef.PartitionKeyFields[i].Value); f != nil {
			callback(f)
		}
	}
}

// Iterate view clustering columns
func (s ViewStmt) ClusteringColumns(callback func(f *ViewItemExpr)) {
	for i := 0; i < len(s.pkRef.ClusteringColumnsFields); i++ {
		if f := s.Field(s.pkRef.ClusteringColumnsFields[i].Value); f != nil {
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
		return i.Field.Name.Value
	}
	if i.RefField != nil {
		return i.RefField.Name.Value
	}
	return ""
}

type PrimaryKeyExpr struct {
	Pos                     lexer.Position
	PartitionKeyFields      []Identifier `parser:"('(' @@ (',' @@)* ')')?"`
	ClusteringColumnsFields []Identifier `parser:"(','? @@ (',' @@)*)?"`
}

func (s ViewStmt) GetName() string { return string(s.Name) }

type ViewRefField struct {
	Statement
	Name    Identifier `parser:"@@"`
	RefDocs []DefQName `parser:"'ref' ('(' @@ (',' @@)* ')')?"`
	NotNull bool       `parser:"@(NOTNULL)?"`

	// filled on the analysis stage
	refQNames []appdef.QName
}

type ViewField struct {
	Statement
	Name    Identifier `parser:"@@"`
	Type    DataType   `parser:"@@"`
	NotNull bool       `parser:"@(NOTNULL)?"`
}

type IVariableResolver interface {
	AsInt32(name appdef.QName) (int32, bool)
}

// BuildAppDefsOption is a function that can be passed to BuildAppDefs to configure it.
type BuildAppDefsOption = func(*buildContext)

func WithVariableResolver(resolver IVariableResolver) BuildAppDefsOption {
	return func(c *buildContext) {
		c.variableResolver = resolver
	}
}

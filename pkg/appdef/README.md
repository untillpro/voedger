# Application Definition

[![codecov](https://codecov.io/gh/voedger/voedger/appdef/branch/main/graph/badge.svg?token=u6VrbqKtnn)](https://codecov.io/gh/voedger/voedger/appdef)

## Types

### Types inheritance

```mermaid
classDiagram
    class IType {
        <<interface>>
        +QName() QName
        +Kind()* TypeKind
        +Comment() []string
        +Tags(func(ITag)bool)
    }

    ITag --|> IType : inherits
    class ITag {
        <<interface>>
        +Kind()* TypeKind_Tag
    }

    IData --|> IType : inherits
    class IData {
        <<interface>>
        +Kind()* TypeKind_Data
        +Ancestor() IData
        +Constraints() []IConstraint
    }

    IArray --|> IType : inherits
    class IArray {
        <<interface>>
        +Kind()* TypeKind_Array
        +MaxLen() uint
        +Elem() IType
    }

    IType <|-- IStructure : inherits
    class IStructure {
        <<interface>>
        +Abstract() bool
        +Fields() []IField
        +Containers() []IContainer
        +Uniques() []IUnique
        +SystemField_QName() IField
    }

    IStructure <|-- IRecord  : inherits
    class IRecord {
        <<interface>>
        +SystemField_ID() IField
        +SystemField_IsActive() IField
    }

    IDoc --|> IRecord : inherits
    class IDoc {
        <<interface>>
    }

    ISingleton --|> IDoc : inherits
    class ISingleton {
        <<interface>>
        +Singleton() bool
    }

    IGDoc --|> IDoc : inherits
    class IGDoc {
        <<interface>>
        +Kind()* TypeKind_GDoc
    }

    ICDoc --|> ISingleton : inherits
    class ICDoc {
        <<interface>>
        +Kind()* TypeKind_CDoc
    }

    IWDoc --|> ISingleton : inherits
    class IWDoc {
        <<interface>>
        +Kind()* TypeKind_WDoc
    }
                    
    IODoc --|> IDoc: inherits
    class IODoc {
        <<interface>>
        +Kind()* TypeKind_ODoc
    }

    IRecord <|-- IContainedRecord  : inherits
    class IContainedRecord {
        <<interface>>
        +SystemField_ParentID() IField
        +SystemField_Container() IField
    }

    IContainedRecord <|-- IGRecord : inherits
    class IGRecord {
        <<interface>>
        +Kind()* TypeKind_GRecord
    }

    IContainedRecord <|-- ICRecord : inherits
    class ICRecord {
        <<interface>>
        +Kind()* TypeKind_CRecord
    }

    IContainedRecord <|-- IWRecord : inherits
    class IWRecord {
        <<interface>>
        +Kind()* TypeKind_WRecord
    }

    IContainedRecord <|-- IORecord : inherits
    class IORecord {
        <<interface>>
        +Kind()* TypeKind_ORecord
    }

    IObject --|> IStructure : inherits
    class IObject {
        <<interface>>
        +Kind()* TypeKind_Object
    }

    IType <|-- IView : inherits
    class IView {
        <<interface>>
        +Kind()* TypeKind_ViewRecord
        +Key() IViewKey
        +Value() IViewValue
    }
            
    IType <|-- IExtension : inherits
    class IExtension {
        <<interface>>
        +Name() string
        +Engine() ExtensionEngineKind
        +States() IStorages
        +Intents() IStorages
    }

    IExtension <|-- IFunction : inherits
    class IFunction {
        <<interface>>
        +Param() IType
        +Result() IType
    }

    IFunction <|-- ICommand : inherits
    class ICommand {
        <<interface>>
        +Kind()* TypeKind_Command
        +UnloggedParam() IType
    }

    IFunction <|-- IQuery : inherits
    class IQuery {
        <<interface>>
        +Kind()* TypeKind_Query
    }

    IExtension <|-- IProjector : inherits
    class IProjector {
        <<interface>>
        +Kind()* TypeKind_Projector
        +WantErrors() bool
        +Events() IProjectorEvents
    }

    IExtension <|-- IJob : inherits
    class IJob {
        <<interface>>
        +Kind()* TypeKind_Job
        +CronSchedule() string
    }

    IWorkspace --|> IType : inherits
    class IWorkspace {
        <<interface>>
        +Kind()* TypeKind_Workspace
        +Abstract() bool
        +Descriptor() QName
        +Types(func(IType) bool)
    }

    IRole --|> IType : inherits
    class IRole {
        <<interface>>
        +Kind()* TypeKind_Role
        +ACL() []IACLRule
    }
```

### Types iterators

```mermaid
classDiagram

  class IAppDef {
    <<interface>>
    +Types(func(IType) bool)
  }
  IAppDef "1" *--> "1" iterator : Types

  class IWorkspace {
    <<interface>>
    +LocalTypes(func(IType) bool)
    +Types(func(IType) bool)
  }
  IWorkspace "1" *--> "2" iterator : LocalTypes and Types

  class iterator {
    <<func(iter.Seq[IType])iter.Seq[T]>>
    
    +CDocs(iter.Seq[IType]) iter.Seq[ICDoc]
    +Commands(iter.Seq[IType]) iter.Seq[ICommand]
    +CRecords(iter.Seq[IType]) iter.Seq[ICRecord]
    +DataTypes(iter.Seq[IType]) iter.Seq[IData]
    +Extensions(iter.Seq[IType]) iter.Seq[IExtension]
    +Functions(iter.Seq[IType]) iter.Seq[IFunction]
    +GDocs(iter.Seq[IType]) iter.Seq[IGDoc]
    +GRecords(iter.Seq[IType]) iter.Seq[IGRecord]
    +Jobs(iter.Seq[IType]) iter.Seq[IJob]
    +Limits(iter.Seq[IType]) iter.Seq[ILimit]
    +Objects(iter.Seq[IType]) iter.Seq[IObject]
    +ODocs(iter.Seq[IType]) iter.Seq[IODoc]
    +ORecords(iter.Seq[IType]) iter.Seq[IORecord]
    +Projectors(iter.Seq[IType]) iter.Seq[IProjector]
    +Queries(iter.Seq[IType]) iter.Seq[IQuery]
    +Rates(iter.Seq[IType]) iter.Seq[IRate]
    +Records(iter.Seq[IType]) iter.Seq[IRecord]
    +Roles(iter.Seq[IType]) iter.Seq[IRole]
    +Singletons(iter.Seq[IType]) iter.Seq[ISingleton]
    +Structures(iter.Seq[IType]) iter.Seq[IStructure]
    +Tags(iter.Seq[IType]) iter.Seq[ITag]
    +Views(iter.Seq[IType]) iter.Seq[IView]
    +WDocs(iter.Seq[IType]) iter.Seq[IWDoc]
    +WRecords(iter.Seq[IType]) iter.Seq[IWRecord]

    +TypesByKind(iter.Seq[IType],TypeKind) iter.Seq[T]
    +TypesByKinds(iter.Seq[IType],TypeKinds) iter.Seq[T]
  }

```

### Data types

```mermaid
classDiagram
    direction BT

  class IAppDef {
    <<Interface>>
    +DataTypes(inclSys bool) []IData
    +SysData(DataKind) IData
  }

  IData "0..*" <--o "1" IAppDef : DataTypes
  IData "1..DataKind_count" <--o "1" IAppDef : SysData

    class IType {
        <<interface>>
        +Name() QName
        +Kind() TypeKind
    }

    IData --|> IType : inherits
    class IData {
        <<interface>>
        +Name()* QName
        +Kind()* TypeKind_Data
        +DataKind() DataKind
        +Ancestor() IData
        +Constraints() []IConstraint
    }

    Name "1" <--* "1" IData : Name
    class Name {
        <<QName>>
    }
    note for Name "- for built-in types sys.int32, sys.float64, etc.,
                   - for custom types — user-defined and
                   - NullQName for anonymous types"

    DataKind "1" <--* "1" IData : Kind
    class DataKind {
        <<DataKind>>
    }
    note for DataKind " - null
                        - int32
                        - int64
                        - float32
                        - float64
                        - bytes
                        - string
                        - QName
                        - bool
                        - RecordID
                        - Record
                        - Event"
 
    Ancestor "1" <--* "1" IData : Ancestor
    class Ancestor {
        <<IData>>
    }
    note for Ancestor "  - data type from which the user data type is inherits or 
                         - nil for built-in types"

    IConstraint "0..*" <--*  "1" IData : Constraints
    class IConstraint {
        <<interface>>
        +Kind() ConstraintKind
        +Value() any
    }
    note for IConstraint " - minLen() uint
                           - maxLen() uint
                           - Pattern() RegExp
                           - MinInclusive() float
                           - MinExclusive() float
                           - MaxInclusive() float
                           - MaxExclusive() float
                           - Enum() []enumerable"
```

### Structures

Structured (documents, records, objects) are those structural types that have fields and can contain containers with other structural types.

The inheritance and composing diagrams given below are expanded general diagrams of the types above.

### Structures inheritance

```mermaid
classDiagram
    direction BT
    namespace _ {
        class IStructure {
            <<interface>>
            +Abstract() bool
            +Fields() []IField
            +Containers() []IContainer
            +Uniques() []IUnique
            +SystemField_QName() IField
        }

        class IRecord {
            <<interface>>
            +SystemField_ID() IField
            +SystemField_IsActive() IField
        }
    }

    IRecord --|> IStructure : inherits

    IDoc --|> IRecord : inherits
    class IDoc {
        <<interface>>
    }

    ISingleton --|> IDoc : inherits
    class ISingleton {
        <<interface>>
        +Singleton() bool
    }

    IGDoc --|> IDoc : inherits
    class IGDoc {
        <<interface>>
        +Kind()* TypeKind_GDoc
    }

    ICDoc --|> ISingleton : inherits
    class ICDoc {
        <<interface>>
        +Kind()* TypeKind_CDoc
    }

    IWDoc --|> ISingleton : inherits
    class IWDoc {
        <<interface>>
        +Kind()* TypeKind_WDoc
    }
                    
    IODoc --|> IDoc: inherits
    class IODoc {
        <<interface>>
        +Kind()* TypeKind_ODoc
    }

    IRecord <|-- IContainedRecord  : inherits
    class IContainedRecord {
        <<interface>>
        +SystemField_ParentID() IField
        +SystemField_Container() IField
    }

    IContainedRecord <|-- IGRecord : inherits
    class IGRecord {
        <<interface>>
        +Kind()* TypeKind_GRecord
    }

    IContainedRecord <|-- ICRecord : inherits
    class ICRecord {
        <<interface>>
        +Kind()* TypeKind_CRecord
    }

    IContainedRecord <|-- IWRecord : inherits
    class IWRecord {
        <<interface>>
        +Kind()* TypeKind_WRecord
    }

    IContainedRecord <|-- IORecord : inherits
    class IORecord {
        <<interface>>
        +Kind()* TypeKind_ORecord
    }
```

### Structures composing

```mermaid
classDiagram
  direction TB

  IGDoc "1" o--> "0..*" IGRecord : children
  IGRecord "1" o--> "0..*" IGRecord : children

  ICDoc "1" o--> "0..*" ICRecord : children
  ICRecord "1" o--> "0..*" ICRecord : children

  IWDoc "1" o--> "0..*" IWRecord : children
  IWRecord "1" o--> "0..*" IWRecord : children

  IODoc "1" o--> "0..*" IORecord : children
  IODoc "1" o--> "0..*" IODoc : children document
  IORecord "1" o--> "0..*" IORecord : children

  IObject "1" o--> "0..*" IObject : children
```

### Fields, Containers, Uniques

```mermaid
classDiagram

  class IField {
    <<Interface>>
    +Name() FieldName
    +DataKind() DataKind
    +Required() bool
    +Verified() bool
    +VerificationKind() []VerificationKind
    +Constraints() []IConstraint
  }

  class IWithFields{
    <<Interface>>
    Field(FieldName) IField
    FieldCount() int
    Fields() []IField
  }
  IWithFields "1" --* "0..*" IField : compose

  class IFieldsBuilder {
    <<Interface>>
    AddField(…)
    AddVerifiedField(…)
    AddRefField(…)
    AddStringField(…)
    AddConstraints(IConstraint...)
  }

  IRefField --|> IField : inherits
  class IRefField {
    <<Interface>>
    Refs() []QName
  }

  class IContainer {
    <<Interface>>
    +Name() string
    +Def() IDef
    +MinOccurs() int
    +MaxOccurs() int
  }

  class IWithContainers{
    <<Interface>>
    Container(string) IContainer
    ContainerCount() int
    ContainerDef() [string]IType
    Containers() []IContainer
  }
  IWithContainers "1" --* "0..*" IContainer : compose

  class IContainersBuilder {
    <<Interface>>
    AddContainer(…) IContainer
  }

  class IUnique {
    <<Interface>>
    +Name() QName
    +Fields() []IFeld
  }

  class IWithUniques{
    <<Interface>>
    UniqueByName(QName) IUnique
    UniqueCount() int
    Uniques() []IUnique
  }
  IWithUniques "1" --* "0..*" IUnique : compose

  class IUniquesBuilder {
    <<Interface>>
    AddUnique(…) IUnique
  }
```

### Views

```mermaid
classDiagram
  class IType{
    <<Interface>>
    +Kind()* TypeKind
    +QName() QName
  }

  IType <|-- IView : inherits
  class IView {
    <<Interface>>
    +Kind()* TypeKind_View
    IWithFields
    +Key() IViewKey
    +Value() IViewValue
  }
  IView "1" *--> "1" IViewKey : Key
  IView "1" *--> "1" IViewValue : Value

  class IViewKey {
    <<Interface>>
    IWithFields
    +PartKey() IViewPartKey
    +ClustCols() IViewClustCols
  }
  IViewKey "1" *--> "1..*" IField : fields
  IViewKey "1" *--> "1" IViewPartKey : PartKey
  IViewKey "1" *--> "1" IViewClustCols : ClustCols

  class IViewPartKey {
    <<Interface>>
    IWithFields
  }
  IViewPartKey "1" *--> "1..*" IField : fields

  class IViewClustCols {
    <<Interface>>
    IWithFields
  }
  IViewClustCols "1" *--> "1..*" IField : fields

  class IViewValue {
    <<Interface>>
    IWithFields
  }
  IViewValue "1" *--> "1..*" IField : fields

  class IField {
    <<interface>>
    …
  }
```

### Extensions

```mermaid
    classDiagram
    IType <|-- IExtension : inherits
    class IExtension {
        <<interface>>
        +Name() string
        +Engine() ExtensionEngineKind
        +States() IStorages
        +Intents() IStorages
    }

    IExtension "1" ..> "1" ExtensionEngineKind : Engine
    class ExtensionEngineKind {
        <<enumeration>>
        BuiltIn
        WASM
    }

    IExtension "1" *--> "1" IStorages : States
    IExtension "1" *--> "1" IStorages : Intents
    class IStorages {
        <<interface>>
        +Enum(func(IStorage))
        +Len() int
        +Map() map[QName] []QName
        +Storage(QName) IStorage
    }
    IStorages "1" *--> "0..*" IStorage : Storages
    class IStorage {
        <<interface>>
        +Comment() : []string
        +Name(): QName
        +QNames() []QName
    }

    IExtension <|-- IFunction : inherits
    class IFunction {
        <<interface>>
        +Param() IType
        +Result() IType
    }

    IFunction <|-- ICommand : inherits
    class ICommand {
        <<interface>>
        +Kind()* TypeKind_Command
        +UnloggedParam() IType
    }

    IFunction <|-- IQuery : inherits
    class IQuery {
        <<interface>>
        +Kind()* TypeKind_Query
    }

    IExtension <|-- IProjector : inherits
    class IProjector {
        <<interface>>
        +Kind()* TypeKind_Projector
        +WantErrors() bool
        +Events() IProjectorEvents
    }

    IProjector "1" *--> "1" IProjectorEvents : Events
    IProjectorEvents "1" *--> "1..*" IProjectorEvent : Event
    class IProjectorEvents {
        <<interface>>
        +Enum(func(IProjectorEvent))
        +Event(QName) IProjectorEvent
        +Len() int
        +Map() map[QName] []ProjectorEventKind
    }
    class IProjectorEvent {
        <<interface>>
        +Comment() []string
        +On() IType
        +Kind() []ProjectorEventKind
    }

    IProjectorEvent "1" ..> "1..*" ProjectorEventKind : Kind
    class ProjectorEventKind {
        <<enumeration>>
        Insert
        Update
        Activate
        Deactivate
        Execute
        ExecuteWithParam
    }

    IExtension <|-- IJob : inherits
    class IJob {
        <<interface>>
        +Kind()* TypeKind_Job
        +CronSchedule() string
    }
```

*Rem*: In the above diagram the Param and Result of the function are `IType`, in future versions it will be changed to an array of `[]IParam` and renamed to plural (`Params`, `Results`).

### Roles and ACL

```mermaid
    classDiagram
    IType <|-- IRole : inherits
    class IRole {
        <<interface>>
        +Kind()* TypeKind_Role
        +ACL() []IACLRule
    }

    IRole "1" *--> "1..*" IACLRule : ACL

    class IACLRule {
        <<interface>>
        +Comment() []string
        +Ops() []OperationKind
        +Policy() PolicyKind
        +Resources() IResourcePattern
    }

    IACLRule "1" *--> "1..*" OperationKind : operations
    
    class OperationKind {
        <<enumeration>>
        Insert
        Update
        Select
        Execute
        Inherits
    }

    IACLRule "1" *--> "1" PolicyKind : policy
    
    class PolicyKind {
        <<enumeration>>
        Allow
        Deny
    }

    IACLRule "1" *--> "1" IResourcePattern : resources

    class IResourcePattern {
        <<interface>>
        +On()[]QName
        +Fields()[]FieldName
    }

    IResourcePattern "1" *--> "1..*" QName : On
    IResourcePattern "1" *--> "0..*" FieldName : Fields

    note for QName "names or patterns of resources"
    note for FieldName "fields of records or view records for select and update operations"
```

### Tags

### Rates and Limits

### Workspaces

## Restrictions

### Names

- Only letters (from `A` to `Z` and from `a` to `z`), digits (from `0` to `9`) and underscore symbol (`_`) are used.
- First symbol must be letter or underscore.
- Maximum length of name is 255.
- Names are case sensitive.
- System level names can contains buck char (`$`).

Valid names examples:

```text
  Foo
  bar
  FooBar
  foo_bar
  f007
  _f00_bar
```

Invalid names examples:

```text
  Fo-o
  7bar
```

### Fields

- Maximum fields per structure is 65536.
- Maximum string and bytes field length is 65535.

### Containers

- Maximum containers per structure is 65536.

### Uniques

- Maximum fields per unique is 256
- Maximum uniques per structure is 100.

### Singletons

- Maximum singletons per application is 512.

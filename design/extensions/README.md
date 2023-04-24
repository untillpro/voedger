# Extensions

## Extensions in Packages
```mermaid
erDiagram
Package ||--|{ SchemaFile: "has *.heeus"
Package ||..|| PackageSchema: defines
PackageSchema ||..|{ SchemaFile: "defined by"
PackageSchema ||--o{ ExtensionDef: has
Package ||--o{ ExtSrcFile: "has *.go, *.ts, *.lua"
Package ||--o| ExtBuildFile: "has build.sh"
ExtBuildFile ||..|{ ExtSrcFile: "compiles"
ExtBuildFile ||..|| ExtWasmFile: "produces pkg.wasm"
ExtensionDef ||..|{ ExtSrcFile: "implemented in"
ExtWasmFile ||..|| ExtensionModule: "is"
```
See also: 
- [Design: Extensions](https://github.com/heeus/heeus-design#extensions)

## Extension Engine Architecture
```mermaid
    erDiagram
    AppPartition ||--|| ExtensionsSite: is

    ExtensionsSite ||--|{ ExtensionModule : "has"
    ExtensionsSite ||--|{ ExtensionPoint : "has"
    ExtensionsSite ||--|{ ExtensionEngine : "has"

    ExtensionPoint ||--|| ExtensionIO: "has"

    ExtensionEngine ||--|| ExtensionLimits : "has"
    ExtensionEngine ||..|| ExtensionEngineFactory : "created by"
    ExtensionEngine ||..|{ Extension: "invokes"
    ExtensionEngine ||..|| ExtensionModule: "instantiated from"
    
    
    ExtensionLimits ||..|| ExecutionInterval: "can be"
    ExtensionLimits ||..|| IntentsLimit: "can be"

    ExtensionEngineFactory ||..|| ExtEngineKind : "one per"

    ExtensionModule ||..|| ExtEngineKind: "has"
    ExtensionPoint ||--|{ Extension : "has"


    ExtensionIO ||..|{ Extension: "used to read from State by"
    ExtensionIO ||..|{ Extension: "used to make Intents by"

    ExtEngineKind ||..|| ExtEngineKind_WASM: "can be"
    ExtEngineKind ||..|| ExtEngineKind_BuiltIn: "can be"

    ExtEngineConfiguration ||--|| MemoryLimit: "includes"
    ExtensionEngineFactory ||..|| ExtEngineConfiguration: "uses"
```
See Also:
- Current Diagram [Extension Engines](https://github.com/heeus/heeus-design/#extension-engines)


### Principles: Extension Programming 
- Extensions are [pure functions](https://en.wikipedia.org/wiki/Pure_function). No global variables allowed.
- Any kind of input/output is done using State and Intents only.
- package name must be the same that is declared in PackageSchema


## Declaration in PackageSchema
### Projector
```sql
package wasmprojectors

PROJECTOR myprojector ON ARG untill.PBill, ARG untill.Order ENGINE wasm
```
> Note: LANGUAGE is actually an Extension Kind. We use LANGUAGE because it's the part of SQL syntax (Cassandra, PostgreSQL, etc)

Syntax:

`PROJECTOR projector_name ON ARG qname | EVENT qname | ERRORS (',' ARG qname | EVENT qname | ERRORS)* ENGINE language` 

Notes:
- `ON` is a conjunction of:
    - `ARG <QName>` - handle events with specified arguments
    - `EVENT <QName>` - handle specified events 
    - `ERRORS` - handle errors

### Command
```sql
package wasmcommands

COMMAND mycommand(untill.pbill) RETURNS sys.Json ENGINE wasm
```
Syntax:

`COMMAND command_name '(' params_schema [',' unlogged_params_schema] ')' [ RETURNS return_schema ] ENGINE language` 

### Query Function
```sql
package wasmqueryfuncs

QUERYFUNC myqueryfunc(sys.Json) RETURNS sys.Json ENGINE wasm
```
Syntax:

`QUERYFUNC function_name '(' params_schema ')' [ RETURNS return_schema ] ENGINE language` 

## Use in the App Schema
```sql
import "github.com/mycompany/mymodule/wasmprojectors"
import "github.com/mycompany/mymodule/wasmcommands"
import "github.com/mycompany/mymodule/wasmqueryfuncs"

```

## Create Extension
```bash
heeus ext init assemblyscript|tinygo|lua
```
Creates in the current package: 
|      File              |       Description      | AssempblyScript | TinyGo | LUA
| ---------------------- | ---------------------- | --------------- | ------ | ---- 
| extension.go\|ts\|lua  |  extension source code |        +        |   +    |  +
| package.heeus          |  extension declaraion  |        +        |   +    |  +
| test.sh                |  run extension tests   |        +        |   +    |  +
| build.sh               |  build WASM file       |        +        |   +    |  -
 
## Constraints
- Not possible to use both assemblyscript and tinygo langs within the same package. But it is possible to combine WASM and LUA extension kinds within the same package.


# Literature
- https://cassandra.apache.org/doc/latest/cassandra/cql/functions.html
- https://www.ibm.com/docs/en/db2/11.1?topic=statements-create-procedure-sql
- https://www.postgresql.org/docs/current/sql-createprocedure.html

# See Also
- [Heeus: Repository & Application Schema](https://github.com/heeus/heeus-design#repository--application-schema)
- https://github.com/heeus/heeus
- https://github.com/heeus/inv-go/blob/master/20220221-parsing/participle/schema.sql
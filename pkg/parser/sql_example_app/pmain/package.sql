/*
* Copyright (c) 2023-present unTill Pro, Ltd.
*/

/*
* Package consists of schema and resources
* Schema consists of few schema files
*/

IMPORT SCHEMA 'github.com/untillpro/untill';
IMPORT SCHEMA 'github.com/untillpro/airsbp' AS air;

/*
* APPLICATION statement defines the application.
* Package cannot contain more than one APPLICATION statement
* When building Application schema from packages, exactly one package must have APPLICATION statement
*/
APPLICATION example_app (
    USE air; -- name or alias. This identifies package name in QNames of the app
    USE untill;
);


-- Declare variable to use it later in the schema. Can be overriden on app deployment stage
DECLARE nAntiDdosPerSecRate int32 DEFAULT 1000;


/*
    Abstract tables can only be used for INHERITance by other tables.
    INHERITS includes all the fields, nested tables and constraints from an ancestor table.
    It is not allowed to use abstract tables for:
        - including into workspaces with USE statement;
        - declaring as nested table;
        - specifying in reference fields;
        - using in projectors;
        - making CUD in any workspace;
*/
ABSTRACT TABLE NestedWithName INHERITS CRecord (
    /*  Field is added to any table inherited from NestedWithName
        The current comment is also added to scheme for this field  */

    ItemName varchar(50) -- Max length is 1024
);

/*
    Declare a table to use it later as nested.
    Note: Quotes can be optionally used with identifiers
*/
TABLE "NestedTable" INHERITS NestedWithName (
    ItemDescr varchar -- Default length is 255
);

-- Declare tag to assign it later to definition(s)
TAG BackofficeTag;

/*
    Any declared table must have one of the following tables as a root anchestor:
        - CDoc (Configuration)
        - ODoc (Operational)
        - WDoc (Workflow)
        - Singleton (Configration singleton)

    Nested tables must have one of the following tables as a root anchestor:
        - CRecord (Configuration)
        - ODoc (Operational)
        - WDoc (Workflow)
*/
TABLE TablePlan INHERITS CDoc (
    FState int,
    TableNumber int,
    Name varchar NOT NULL,
    Rate currency NOT NULL,
    Expiration timestamp,
    VerifiableField varchar NOT NULL VERIFIABLE, -- Verifiable field
    Int1 int DEFAULT 1 CHECK(Int1 >= 1 AND Int2 < 10000),  -- Expressions evaluating to TRUE or UNKNOWN succeed.
    Text1 varchar DEFAULT 'a',
    "bytes" bytes, -- optional quotes
    ScreenGroupRef ref(ScreenGroup),
    AnyTableRef ref,
    FewTablesRef ref(ScreenGroup, TablePlan) NOT NULL,
    CheckedField varchar(8) CHECK '^[0-9]{8}$', -- Field validated by regexp
    CHECK (ValidateRow(this)), -- Unnamed CHECK table constraint. Expressions evaluating to TRUE or UNKNOWN succeed.
    CONSTRAINT StateChecker CHECK (ValidateFState(FState)), -- Named CHECK table constraint
    UNIQUE (FState, Name), -- unnamed UNIQUE table constraint, core generates `main.TablePlan$uniques$01` automatically
    CONSTRAINT UniqueTable UNIQUE (TableNumber), -- named UNIQUE table constraint
    UNIQUEFIELD Name, -- deprecated. For Air backward compatibility only
    TableItems TABLE TablePlanItem (
        TableNo int,
        Chairs int
    ),
    items NestedTable, -- Include table declared in different place. Must be one of Record types
    ExcludedTableItems TablePlanItem
) WITH Comment='Backoffice Table', Tags=(BackofficeTag); -- Optional comment and tags

TABLE ScreenGroup INHERITS CDoc();

/*
    Singletones are always CDOC. Error is thrown on attempt to declare it as WDOC or ODOC
    These comments are included in the statement definition, but may be overridden with `WITH Comment=...`
*/
TABLE SubscriptionProfile INHERITS Singleton (
    CustomerID varchar,
    CustomerKind int,
    CompanyName varchar
);

-- Package-level extensions
EXTENSION ENGINE WASM (

    -- Function which takes sys.TableRow (unnamed param), returns boolean and implemented in WASM module in this package
    FUNCTION ValidateRow(TableRow) RETURNS boolean;

    -- Function which takes named parameter, returns boolean, and implemented in WASM module in this package
    FUNCTION ValidateFState(State int) RETURNS boolean;

);

--  Default object scope is PER APP PARTITION and no subject scope
RATE AppDefaultRate 1000 PER HOUR;

-- WORKSPACE statement declares the Workspace, descriptor and definitions, allowed in this workspace
WORKSPACE MyWorkspace (
    DESCRIPTOR(                     -- Workspace descriptor is always SINGLETON
                                    -- If name omitted, then QName is: <WorkspaceName>+"Descriptor"

        air.TypeWithName,           -- Fieldset
        Country varchar(2) CHECK '^[A-Za-z]{2}$',
        Description varchar(100)
    );

    -- Definitions declared in the workspace are only available in this workspace
    TAG PosTag;
    ROLE LocationManager;
    ROLE LocationUser;
    TYPE TypeWithKind (
        Kind int
    );
    TYPE SubscriptionEvent (
        Origin varchar(20),
        Data varchar(20)
    );
    TYPE RestorePasswordParam (
        Email varchar(50)
    );

    -- To include table or workspace declared in different place of the schema, they must be USEd:
	USE TABLE SubscriptionProfile;
    USE WORKSPACE MyWorkspace;  -- It's now possible to create MyWorkspace in MyWorkspace hierarchy

    -- Declare table within workspace
    TABLE WsTable INHERITS CDoc (
        air.TypeWithName,   -- Fieldset

        PsName varchar(15),
        items TABLE Child (
            TypeWithKind, -- Fieldset
            Number int
        )
    );

    -- Workspace-level extensions
    EXTENSION ENGINE BUILTIN (

        /*
        Projector can only be declared in workspace.

        A builtin function CountOrders must exist in package resources.
            ON Orders - points to a command
            INTENTS - lists all storage keys, projector generates intents for
            STATE - lists all storage keys, projector reads state from
                (key consist of Storage Qname, and Entity name, when required by storage)
                (no need to specify in STATE when already listed in INTENTS)
        */
        PROJECTOR CountOrders
            AFTER EXECUTE ON NewOrder
            INTENTS(View(OrdersCountView));

        -- Projectors triggered by CUD operation
        -- SYNC means that projector is synchronous
        SYNC PROJECTOR TablePlanThumbnailGen
            AFTER INSERT ON TablePlan
            INTENTS(View(TablePlanThumbnails));

        -- Projector triggered by command argument SubscriptionEvent
        -- Projector uses sys.HTTPStorage
        PROJECTOR UpdateSubscriptionProfile
            AFTER EXECUTE WITH PARAM ON SubscriptionEvent
            STATE(sys.Http, AppSecret);

        -- Projector triggered by few COMMANDs
        PROJECTOR UpdateDashboard
            AFTER EXECUTE ON (NewOrder, NewOrder2)
            STATE (Http, AppSecret)
            INTENTS(View(DashboardView, XZReports, NotificationsHistory, ActiveTablePlansView));

        -- Projector triggered by few types of CUD operations
        PROJECTOR UpdateActivePlans
            AFTER ACTIVATE OR DEACTIVATE ON TablePlan
            INTENTS(View(ActiveTablePlansView));

        /*
            Some projector which sends E-mails and performs HTTP queries.
            This one also triggered on events with errors
        */
        PROJECTOR NotifyOnChanges
            AFTER INSERT OR UPDATE ON (TablePlan, WsTable)
            STATE(Http, AppSecret)
            INTENTS(SendMail, View(NotificationsHistory))
            INCLUDING ERRORS;

        /*
        Projector on any CUD operation.
        CDoc, WDoc, ODoc are the only abstract tables which are allowed to use in this case
        */
        PROJECTOR RecordsRegistryProjector
            AFTER INSERT OR ACTIVATE OR DEACTIVATE ON (CRecord, WRecord);

        /*
        Commands can only be declared in workspaces
        Command can have optional argument and/or unlogged argument
        Command can return TYPE
        */
        COMMAND NewOrder(air.Order, UNLOGGED air.TypeWithName) RETURNS air.Order;

        -- Command can return void (in this case `RETURNS void` may be omitted)
        COMMAND NewOrder2(air.Order) RETURNS void;

        COMMAND RestorePassword(RestorePasswordParam) RETURNS void;

        -- Command with declared Comment, Tags
        COMMAND NewOrder4(UNLOGGED air.Order) WITH
            Tags=(BackofficeTag, PosTag);

        -- Qieries can only be declared in workspaces
        QUERY Query1 RETURNS void;

        -- WITH Comment... overrides this comment
        QUERY _Query1() RETURNS air.Order WITH Comment='A comment';

        -- Query which can return any value
        QUERY Query2(air.Order) RETURNS any;
    );

    -- Object scope is PER APP PARTITION PER IP
    -- Use variable declared in the package
    RATE AntiDDosRate nAntiDdosPerSecRate PER SECOND;

    --  Custom scopes
    RATE BackofficeRate 1000 PER HOUR PER APP PARTITION;
    RATE QueryRate 1000 PER HOUR PER APP PARTITION PER IP;
    RATE CudRate 100 PER HOUR PER USER;
    RATE RestorePasswordRate1 3 PER 5 MINUTES PER APP PARTITION PER IP;
    RATE RestorePasswordRate2 10 PER DAY PER APP PARTITION PER IP;

	LIMIT AntiDDOS ON EVERYTHING WITH RATE AntiDDosRate; -- all commands, queries and CUD
	LIMIT RestorePasswordLimit1 ON COMMAND RestorePassword WITH RATE RestorePasswordRate1;   -- Single command applied with rate
	LIMIT RestorePasswordLimit2 ON COMMAND RestorePassword WITH RATE RestorePasswordRate2;   -- Combination of two rates
	LIMIT Query1Limit ON QUERY Query1 WITH RATE QueryRate; -- Single query applied with rate
	LIMIT tl1 ON TABLE WsTable WITH RATE CudRate; -- CUD operations on a single table
	LIMIT BackofficeLimit ON TAG BackofficeTag WITH RATE BackofficeRate; -- Limit on anything with tag

    -- ACLs
    GRANT ALL ON ALL TABLES WITH TAG BackofficeTag TO LocationManager;
    GRANT INSERT,UPDATE ON ALL TABLES WITH TAG PosTag TO LocationUser;
    GRANT SELECT ON TABLE untill.Prices TO LocationUser;
    GRANT SELECT(Price) ON TABLE untill.Prices TO LocationUser;
    GRANT INSERT ON COMMAND NewOrder TO LocationUser;
    GRANT SELECT ON QUERY Query1 TO LocationUser;
    GRANT SELECT ON ALL QUERIES WITH TAG PosTag TO main.LocationUser;
    GRANT INSERT ON WORKSPACE MyWorkspace TO LocationUser;

    -- VIEWs generated by the PROJECTOR.
    -- Primary Key must be declared in View.
    VIEW XZReports(

        -- Report Year
        Year int32,

        -- Report Month
        Month int32,

        -- Report Day
        Day int32,

        /*
            Field comment:
            0=X, 1=Z
        */
        Kind int32,
        Number int32,
        Description varchar(50),

        -- Reference to WDoc
        XZReportWDocID ref NOT NULL,
        PRIMARY KEY ((Year), Month, Day, Kind, Number)
    ) AS RESULT OF UpdateDashboard;

    VIEW OrdersCountView(
        Year int, -- same as int32
        Month int32,
        Day int32,
        Qnantity int32,
        SomeField int32,
        PRIMARY KEY ((Year), Month, Day)
    ) AS RESULT OF CountOrders;

    VIEW TablePlanThumbnails(
        Dummy int,
        Dummy2 int,
        PRIMARY KEY ((Dummy), Dummy2)
    ) AS RESULT OF TablePlanThumbnailGen;

    VIEW DashboardView(
        Dummy int,
        SomeRec record,
        Dummy2 int,
        PRIMARY KEY ((Dummy), Dummy2)
    ) AS RESULT OF UpdateDashboard;
    VIEW NotificationsHistory(
        Dummy int,
        Dummy2 int,
        PRIMARY KEY ((Dummy), Dummy2)
    ) AS RESULT OF UpdateDashboard;
    VIEW ActiveTablePlansView(
        Dummy int,
        Dummy2 int,
        PRIMARY KEY ((Dummy), Dummy2)
    ) AS RESULT OF UpdateDashboard;

);

/*
    Abstract workspaces:
        - Cannot be created
        - Cannot declare DESCRIPTOR
        - Cannot be USEd in other workspaces
        - Can only be used by other workspaces for INHERITance
*/
ABSTRACT WORKSPACE AWorkspace ();

/*
    INHERITS includes everything which is declared and/or USEd by other workspace.
    Possible to inherit from multiple workspaces
*/
WORKSPACE MyWorkspace1 INHERITS AWorkspace, untill.UntillAWorkspace (
    POOL OF WORKSPACE MyPool ();
);

/*
    Allow my statements to be used in sys.Profile.
    sys.Profile workspace is declared as ALTERABLE, this allows other packages to extend it with ALTER WORKSPACE.
    We can also ALTER non-alterable workspaces when they are in the same package
*/
ALTER WORKSPACE sys.Profile(
    USE TABLE TablePlan;
    USE WORKSPACE MyWorkspace1;
);

-- Declares ROLE
ROLE UntillPaymentsUser;

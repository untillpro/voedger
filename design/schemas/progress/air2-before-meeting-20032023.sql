SCHEMA air;

IMPORT SCHEMA github.com/untillpro/airs-bp3/packages/untill

-- Principles:
---- 1. The following DDLs can only be declared in WORKSPACE:
----    QUERY, COMMAND, PROJECTOR, RATE, GRANT, USE TABLE
---- 2. The following DDLs can only be declared out of WORKSPACE:
----    TEMPLATE

WORKSPACE Restaurant (
    
    -------------------------------------------------------------------------------------
    -- Roles
    --
    ROLE UntillPaymentsUser;
    ROLE LocationManager;
    ROLE LocationUser;

    -------------------------------------------------------------------------------------
    -- Checks
    --
    FUNCTION MyTableValidator(sys.TableRow) RETURNS void ENGINE BUILTIN; 
    FUNCTION MyTableValidator RETURNS void ENGINE BUILTIN; -- parameters may be omitted
    FUNCTION MyFieldsValidator(fieldA text, fieldB text) RETURNS void ENGINE BUILTIN; 
    PROCEDURE MyFieldsValidator ENGINE BUILTIN; -- same as previous
    FUNCTION ApproxEqual(param1 float, param2 float) RETURNS boolean ENGINE BUILTIN;

    CHECK ON TABLE untill.bill IS MyTableValidator;
    CHECK ON TABLE untill.bill AS PROCEDURE MyTableValidator(sys.TableRow) ENGINE BUILTIN; 
    CHECK MyBillCheck ON TABLE untill.bill(name text, pcname text) IS MyFieldsValidator; -- name is optional
    CHECK ON TABLE untill.bill(name text, pcname text) AS FUNCTION MyFieldsValidator(text, text) RETURNS void ENGINE BUILTIN;
    CHECK ON TABLE untill.bill(name text, pcname text) AS PROCEDURE MyFieldsValidator ENGINE BUILTIN; -- same as previous
    CHECK ON TABLE untill.bill(name text, pcname text) AS (text != pcname); 
    CHECK ON FIELD name OF TABLE untill.bill AS (name != '')
    CHECK ON FIELD working_day OF TABLE untill.bill AS '^[0-9]{8}$'
    CHECK NettoBruttoCheck ON TABLE sometable(netto float, brutto float) AS (!ApproxEqual(netto, brutto)); 
    -------------------------------------------------------------------------------------
    -- Projectors
    --
    FUNCTION FillUPProfile(sys.Event) RETURNS void ENGINE WASM;
    PROCEDURE FillUPProfile(sys.Event) ENGINE WASM; -- same as previous

    PROJECTOR ApplyUPProfile ON COMMAND IN (air.CreateUPProfile, air.UpdateUPProfile) IS FillUPProfile; -- name is optional
    PROJECTOR ON COMMAND air.CreateUPProfile AS FUNCTION FillUPProfile(sys.Event) RETURNS void ENGINE WASM;
    PROJECTOR ON COMMAND air.CreateUPProfile AS PROCEDURE FillUPProfile(sys.Event) ENGINE WASM; -- same as previous
    PROJECTOR ON COMMAND ARGUMENT untill.QNameOrders AS PROCEDURE OrdersDatesProjector(sys.Event) ENGINE BUILTIN; 


    -------------------------------------------------------------------------------------
    -- Commands
    --
    PROCEDURE OrdersFunc(untill.orders) ENGINE BUILTIN; 
    FUNCTION PbillFunc(untill.pbill) RETURNS PbillResult ENGINE BUILTIN; 

    COMMAND Orders(untill.orders) IS PbillFunc;
    COMMAND Pbill(untill.pbill) IS PbillFunc;
    COMMAND LinkDeviceToRestaurant(LinkDeviceToRestaurantParams) RETURNS void AS PROCEDURE MyFunc(LinkDeviceToRestaurantParams) ENGINE WASM;

    -------------------------------------------------------------------------------------
    -- Comments
    --
    STRING BackofficeComment AS "This is a backoffice table";

    COMMENT ON QUERY TransactionHistory AS 'Transaction History';      
    COMMENT ON QUERY IN (TransactionHistory, ...) AS 'Transaction History';  
    COMMENT ON ALL QUERIES WITH TAG Backoffice IS BackofficeComment;  
    
    -- ??? optional name
    COMMENT BackofficeQueriesComment ON ALL QUERIES WITH TAG Backoffice IS BackofficeComment;  

    TYPE QueryResellerInfoResult (
        reseller_phone text,
        reseller_company text,
        reseller_email text,
        reseller_website text
    ) WITH Comment AS 'Contains information about Reseller';

    -------------------------------------------------------------------------------------
    -- Rates and Limits
    --
    
    -- "Limit defines the maximum frequency of some events. 
    -- Limit is represented as number of events per second." 
    -- https://pkg.go.dev/golang.org/x/time/rate

    LIMIT BackofficeFuncLimit AS 100 PER MINUTE PER IP; 
    
    RATE ON QUERY TransactionHistory IS BackofficeFuncLimit;
    RATE ON QUERY TransactionHistory AS 101 PER MINUTE PER IP; 

    -- ??? optional name
    RATE TransactionHistoryRate ON QUERY TransactionHistory AS 101 PER MINUTE PER IP; 

    -------------------------------------------------------------------------------------
    -- Tags
    --

    STRING BackofficeTag AS "Backoffice";
    STRING PosTag AS "Pos";
    STRING CollectionTag AS "Collection";

    TAG ON TABLE bill IS PosTag;
    TAG ON COMMAND Orders IS PosTag; 
    TAG ON QUERY QueryResellerInfo AS "Resellers";

    -- Collection is applied to all tables with tag "sys.Collection"
    TAG ON ALL TABLES WITH TAG "Backoffice" AS "sys.Collection";
    TAG ON ALL TABLES WITH TAG BackofficeTag AS "sys.Collection"; --same as previous
    TAG ON ALL TABLES WITH TAG BackofficeTag IS CollectionTag; --same as previous

    -- ??? optional name
    TAG AllBackofficeTablesHaveCollection ON ALL TABLES WITH TAG BackofficeTag IS CollectionTag; 

    -------------------------------------------------------------------------------------
    -- Sequences
    --

    SEQUENCE bill_numbers AS int START WITH 1;
    SEQUENCE bill_numbers AS int MINVALUE 1; -- same as previous
    SEQUENCE SomeDecrementSeqneuce AS int MAXVALUE 1000000 INCREMENT BY -1;

    -------------------------------------------------------------------------------------
    -- Types
    --

    TYPE TransactionHistoryParams (
        BillIDs text NOT NULL,
        EventTypes text NOT NULL,
    );

    TYPE TransactionHistoryResult (
        Offset offset NOT NULL,
        EventType int64 NOT NULL,
        Event text NOT NULL,
    );

    -------------------------------------------------------------------------------------
    -- Queries
    --

    FUNCTION MyFunc(reseller_id text) RETURNS QueryResellerInfoResult ENGINE WASM;

    QUERY QueryResellerInfo(reseller_id text) RETURNS QueryResellerInfoResult IS MyFunc 
        WITH Rate IS BackofficeFuncRate    
        AND Comment AS 'Transaction History';

    -- same as:
    QUERY TransactionHistory(TransactionHistoryParams) AS
        FUNCTION MyFunc(TransactionHistoryParams) RETURNS TransactionHistoryResult[] ENGINE WASM  
        WITH Rate AS PosRate
        AND Comment IS PosComment
        AND Tag IS PosTag;


    -------------------------------------------------------------------------------------
    -- Tables
    --
    
    -- Every workspace Restaurant has all tables from schema `untill`
    USE TABLE untill.*; 

    -- ??? Do we need to USE something else besides TABLEs?

    TABLE air_table_plan OF CDOC (
        fstate int,
        name text,
        ml_name text,
        num int,
        width int,
        height int
    )

    -- see also: untill-tables.sql

    -------------------------------------------------------------------------------------
    -- ACLs
    --
    GRANT ALL ON ALL TABLES WITH TAG untill.Backoffice TO LocationManager;
    GRANT INSERT,UPDATE ON ALL TABLES WITH TAG sys.ODoc TO LocationUser;
    GRANT SELECT ON TABLE untill.orders TO LocationUser;
    GRANT EXECUTE ON COMMAND PBill TO LocationUser;
    GRANT EXECUTE ON COMMAND Orders TO LocationUser;
    GRANT EXECUTE ON QUERY TransactionHistory TO LocationUser;
    GRANT EXECUTE ON ALL QUERIES WITH TAG PosTag TO LocationUser;

    -------------------------------------------------------------------------------------
    -- Singletones
    --
    TABLE Restaurant OF SINGLETONE (
        WorkStartTime text,
        DefaultCurrency int64,
        NextCourseTicketLayout int64,
        TransferTicketLayout int64,
        DisplayName text,
        Country text,
        City text,
        ZipCode text,
        Address text,
        PhoneNumber text,
        VATNumber text,
        ChamberOfCommerce text,
    )

    -------------------------------------------------------------------------------------
    -- Views
    --
    VIEW HourlySalesView(
        yyyymmdd text, 
        hour int, 
        total int, 
        count int,
        primary key((yyyymmdd, hour))
    ) AS SELECT 
        working_day as yyyymmdd,
        EXTRACT(hour from ord_datetime) as hour,
        SUM(price * quantity) as total,
        SUM(quantity) as count
        from untill.orders 
            join order_item on order_item.id_orders=orders.id        
        group by working_day, hour
    WITH Comment IS PosComment;

    VIEW XZReports(
        Year int32, 
        Month int32, 
        Day int32, 
        Kind int32, 
        Number int32, 
        XZReportWDocID id,
        PRIMARY KEY((Year), Month, Day, Kind, Number)
    ) AS RESULT OF UpdateXZReportsView

    -- see also air-views.sql
    
) 

-------------------------------------------------------------------------------------
-- Child Workspaces
--
WORKSPACE Resellers {
    
    ROLE ResellersAdmin;

    -- Child workspace
    WORKSPACE Reseller {
        ROLE UntillPaymentsReseller;
        ROLE AirReseller;
        USE Table PaymentsProfile
    }
}

-------------------------------------------------------------------------------------
-- WORKSPACE Templates
--
TEMPLATE demo OF WORKSPACE air.Restaurant WITH SOURCE wsTemplate_demo;
TEMPLATE resdemo OF WORKSPACE untill.Resellers WITH SOURCE wsTemplate_demo_resellers;


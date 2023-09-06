SCHEMA air;

IMPORT SCHEMA "github.com/untillpro/airs-bp3/packages/untill"

-- Principles:
---- 1. The following DDLs can only be declared in WORKSPACE:
----    QUERY, COMMAND, PROJECTOR, RATE, GRANT, USE TABLE
----        ??? Incompatible to APIv2, e.g.: POST /api/rest/owner/app/<wsid>/<query_qname>/
----              maybe:  /api/rest/owner/qname_app/qname_ws/<wsid>/<query_qname>/
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
    FUNCTION MyFieldsValidator(fieldA text, fieldB text) RETURNS void ENGINE BUILTIN; 
    FUNCTION ApproxEqual(param1 float, param2 float) RETURNS boolean ENGINE BUILTIN;

    ALTER TABLE untill.bill ADD CHECK (MyTableValidator(row));  -- ??? row/this/current ?
    ALTER TABLE untill.bill ADD CONSTRAINT MyBillCheck CHECK (MyFieldsValidator(name, pcname)); -- with name
    ALTER TABLE untill.bill ADD CHECK (text != pcname); 
    ALTER TABLE untill.bill ALTER name SET CHECK (name != ''); 
    ALTER TABLE untill.bill ALTER working_day SET CHECK '^[0-9]{8}$';
    ALTER TABLE sometable ADD CHECK (!ApproxEqual(netto, brutto));

    -- see also: https://www.postgresql.org/docs/15/sql-altertable.html

    -- Was:
    -- CHECK ON TABLE untill.bill IS MyTableValidator;
    -- CHECK MyBillCheck ON TABLE untill.bill(name text, pcname text) IS MyFieldsValidator; -- name is optional
    -- CHECK ON TABLE untill.bill(name text, pcname text) IS (text != pcname); 
    -- CHECK ON FIELD name OF TABLE untill.bill IS (name != '')
    -- CHECK ON FIELD working_day OF TABLE untill.bill IS '^[0-9]{8}$'
    -- CHECK NettoBruttoCheck ON TABLE sometable(netto float, brutto float) IS (!ApproxEqual(netto, brutto)); 

    -------------------------------------------------------------------------------------
    -- Projectors
    --
    FUNCTION FillUPProfile(sys.Event) RETURNS void ENGINE WASM;

    PROJECTOR ApplyUPProfile ON COMMAND IN (air.CreateUPProfile, air.UpdateUPProfile) AS FillUPProfile; -- name is optional
    PROJECTOR ON COMMAND air.CreateUPProfile AS SomeFunc;
    PROJECTOR ON COMMAND ARGUMENT untill.QNameOrders AS SomeFunc; 

    -------------------------------------------------------------------------------------
    -- Commands
    --
    FUNCTION OrdersFunc(untill.orders) RETURNS void ENGINE BUILTIN; 
    FUNCTION PbillFunc(untill.pbill) RETURNS PbillResult ENGINE BUILTIN; 

    COMMAND Orders(untill.orders) AS PbillFunc;
    COMMAND Pbill(untill.pbill) AS PbillFunc;
    
    -------------------------------------------------------------------------------------
    -- Comments
    --

    -- Declare comments
    COMMENT BackofficeComment "This is a backoffice table";

    -- Apply comments
    -- See ALTER TABLE ... SET ( storage_parameter [= value], https://www.postgresql.org/docs/15/sql-altertable.html
    ALTER QUERY TransactionHistory SET Comment=BackofficeComment
    ALTER QUERY TransactionHistory SET Comment='Transaction History', Tags=[Pos, Table];
    ALTER TABLE ALL IN (Currency, Users) SET Comment=BackofficeComment
    ALTER TABLE ALL WITH TAG Backoffice SET Comment=BackofficeComment

    -- was:
    -- COMMENT BackofficeComment "This is a backoffice table";
    --
    -- COMMENT ON QUERY TransactionHistory IS 'Transaction History'; -- Do we allow inline values?     
    -- COMMENT ON QUERY IN (TransactionHistory, ...) IS 'Transaction History';  
    -- COMMENT ON ALL QUERIES WITH TAG Backoffice IS BackofficeComment;  
    
    TYPE QueryResellerInfoResult (
        reseller_phone text,
        reseller_company text,
        reseller_email text,
        reseller_website text
    ) WITH Comment='Contains information about Reseller';

    -------------------------------------------------------------------------------------
    -- Rates 
    --
    
    -- Declare rate
    RATE BackofficeFuncRate 100 PER MINUTE PER IP; 
    
    -- Apply rate
    ALTER QUERY TransactionHistory SET Rate=BackofficeFuncRate;
    ALTER QUERY TransactionHistory SET Rate=101 PER MINUTE PER IP;

    --was:
    --RATE ON QUERY TransactionHistory IS BackofficeFuncLimit;
    --RATE ON QUERY TransactionHistory IS 101 PER MINUTE PER IP;  -- Do we allow inline values?

    -------------------------------------------------------------------------------------
    -- Tags
    --

    -- Declare tags
    TAG Backoffice;
    TAG Pos;
    TAG Collection;

    -- Apply tags
    ALTER TABLE bill SET Tags=[Pos]; -- not possible to declare new tag here
    ALTER COMMAND Orders ADD TAG Pos ADD TAG Backoffice; -- ??? adds tags
    ALTER QUERY QueryResellerInfo SET Tags=[Resseler, Backoffice];
    ALTER TABLE ALL WITH TAG Backoffice SET Tags=[sys.Collection]

    --was:
    --TAG ON TABLE bill IS Pos;
    --TAG ON COMMAND Orders IS Pos; -- inline values are not possible because no way to figure out if Pos is a new tag or reference
    --TAG ON QUERY QueryResellerInfo IS [Resseler, Backoffice];

    -- Collection is applied to all tables with tag "sys.Collection"
    -- TAG ON ALL TABLES WITH TAG Backoffice IS sys.Collection;


    -------------------------------------------------------------------------------------
    -- Sequences
    --

    SEQUENCE bill_numbers int START WITH 1;
    SEQUENCE bill_numbers int MINVALUE 1; -- same as previous
    SEQUENCE SomeDecrementSeqneuce int MAXVALUE 1000000 INCREMENT BY -1;

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

    QUERY QueryResellerInfo(reseller_id text) RETURNS QueryResellerInfoResult AS MyFunc 
        WITH Rate=BackofficeFuncRate,
        Comment='Transaction History',
        Tags=[PosTag1, PosTag2];

    QUERY QueryResellerInfo AS MyFunc --same as previous, parameters maybe omitted
        WITH Rate=BackofficeFuncRate,
        Comment='Transaction History',
        Tags=[PosTag1, PosTag2];

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


    -- ??? AS or IS
    VIEW HourlySalesView(
        yyyymmdd, 
        hour, 
        total, 
        count
    ) AS SELECT
        working_day,
        EXTRACT(hour from ord_datetime),
        (select sum(price * quantity) from order_item),
        (select sum(quantity) from order_item),
        from untill.orders 
    WITH Comment=PosComment, PrimaryKey='(yyyymmdd, hour), asdas';

    VIEW HourlySalesView AS SELECT
        working_day,
        EXTRACT(hour from ord_datetime) as hour,
        (select open_datetime from bill where id=orders.id_bill),
        (select close_datetime from bill where id=orders.id_bill),        
        (select sum(price * quantity) from order_item) as total, -- available for child tables
        (select sum(quantity) from order_item) as count,
        from untill.orders 
    WITH Comment=PosComment, PrimaryKey='(yyyymmdd, hour), asdas';

    VIEW BillsCount AS SELECT count(*) from bill
    VIEW OrdersCount AS SELECT count(*) from orders

    -- same as previous, but with BILL joined
    VIEW HourlySalesView AS SELECT
        working_day,
        EXTRACT(hour from ord_datetime) as hour,
        open_datetime,
        close_datetime,
        (select sum(price * quantity) from order_item) as total, -- available for child tables
        (select sum(quantity) from order_item) as count,
        from untill.orders 
        join bill on id_bill=bill.id
    WITH Comment=PosComment, PrimaryKey='(yyyymmdd, hour), asdas';

--    VIEW HourlySalesView AS SELECT
        ---working_day, 
        --EXTRACT(hour from ord_datetime) as hour, -- alias
        --SUM(price * quantity) as total,
        --SUM(quantity) as count
        --from untill.orders 
            --join order_item on order_item.id_orders=orders.id        
        --group by working_day, hour
    --WITH Comment=PosComment, PrimaryKey='(yyyymmdd, hour), asdas';


    VIEW XZReports(
        Year int32,
        Month int32, 
        Day int32, 
        Kind int32, 
        Number int32, 
        XZReportWDocID id
    ) AS RESULT OF UpdateXZReportsView
    WITH PrimaryKey='(Year), Month, Day, Kind, Number'

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
TEMPLATE demo OF WORKSPACE air.RestaurantWS SOURCE wsTemplate_demo;
TEMPLATE resdemo OF WORKSPACE untill.ResellersWS SOURCE wsTemplate_demo_resellers;



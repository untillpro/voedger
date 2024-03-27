ABSTRACT WORKSPACE MyWorkspace1(
    TABLE MyTable1 INHERITS CDoc(
        Field1 varchar,
        Field2 varchar
    );
    TABLE MyTable3 INHERITS ODoc(
        Field1 varchar,
        Field2 varchar
    );
    TABLE MyTable11 INHERITS WDoc(
        Field11 varchar,
        Field22 bool,
        Field33 int32
    );
    TYPE T1 (
        Field1 varchar,
        Field2 varchar
    );
	TABLE NextNumbers INHERITS WSingleton (
		NextPBillNumber int32
	);
    VIEW XZReportsView(
        Field11 int64,
        Field22 bool,
        Field33 varchar,
        Field44 varchar,
        PRIMARY KEY((Field11), Field22, Field33)
    ) AS RESULT OF UpdateXZReportsView;
	TYPE CreateXZReportParams (
		Kind int32 NOT NULL,
		Number int32 NOT NULL,
		WaiterID int64 NOT NULL,
		From int64,
		Till int64
	);
	TYPE SaveXZReportParams (
		XZReportWDocID ref NOT NULL,
		ReportDataID ref NOT NULL,
		TicketDataID ref NOT NULL
	);
	TYPE XZReportResult (
		Kind int32 NOT NULL,
		Total float64 NOT NULL
	);
    TYPE GetUPTransferInstrumentResult (
		TransferInstrument varchar NOT NULL
	);

	EXTENSION ENGINE BUILTIN (
        COMMAND CreateXZReport(CreateXZReportParams) RETURNS void;
        COMMAND SaveXZReport(SaveXZReportParams) RETURNS XZReportResult;
		QUERY GetUPTransferInstrument() RETURNS GetUPTransferInstrumentResult;
        PROJECTOR GenerateXZReport AFTER EXECUTE ON (CreateXZReport);
        COMMAND RawCmd(sys.Raw) RETURNS sys.Raw;
        SYNC PROJECTOR UpdateXZReportsView AFTER EXECUTE ON (CreateXZReport, SaveXZReport) INTENTS (View(XZReportsView));
	);
);

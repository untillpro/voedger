IMPORT SCHEMA 'github.com/untillpro/airs-scheme/bp3' AS untill;

WORKSPACE RestaurantWS (

	TABLE ProformaPrinted INHERITS ODoc (
		Number int32 NOT NULL,
		UserID ref(untill.untill_users) NOT NULL,
		Timestamp int64 NOT NULL,
		BillID ref(untill.bill) NOT NULL
	);

	VIEW PbillDates (
		Year int32 NOT NULL,
		DayOfYear int32 NOT NULL,
		FirstOffset int64 NOT NULL,
		LastOffset int64 NOT NULL,
		PRIMARY KEY ((Year), DayOfYear)
	) AS RESULT OF FillPbillDates;

	COMMAND Orders(untill.orders);
	COMMAND Pbill(untill.pbill) RETURNS CmdPBillResult;
)
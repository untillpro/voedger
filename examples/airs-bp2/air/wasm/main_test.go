/*
 * Copyright (c) 2024-present unTill Software Development Group B. V.
 * @author Alisher Nurmanov
 */

package main

import (
	"air/wasm/orm"
	"testing"
	"time"

	test "github.com/voedger/voedger/pkg/exttinygo/exttinygotests"
)

func TestPbill(t *testing.T) {
	t.Parallel()

	date := time.Now()

	t.Run("Singleton NextPBillNumber: insert", func(t *testing.T) {

		test.NewCommandTest(
			t,
			orm.Package_air.Command_Pbill,
			Pbill,
		).
			StateRecord(
				orm.Package_untill.WDoc_bill,
				100002,
				`tableno`, 1,
			).
			ArgumentObject(
				2,
				`id_bill`, 100002,
				`id_untill_users`, 100001,
				`pdatetime`, date.UnixMicro(),
			).
			ArgumentObjectRow(`pbill_item`,
				3,
				`sys.ParentID`, 2,
				`id_pbill`, 100000,
				`id_untill_users`, 100001,
				`tableno`, 123,
				`quantity`, 2,
				`price`, 50_00,
			).
			IntentSingletonInsert(
				orm.Package_air.WSingleton_NextNumbers,
				`NextPBillNumber`, 1,
			).
			IntentRecordUpdate(
				orm.Package_untill.WDoc_bill,
				100002,
				`close_year`, date.Year(),
			).
			Run()
	})

	t.Run("Singleton NextPBillNumber: update", func(t *testing.T) {
		nextNumber := 5

		test.NewCommandTest(
			t,
			orm.Package_air.Command_Pbill,
			Pbill,
		).
			StateSingletonRecord(
				orm.Package_air.WSingleton_NextNumbers,
				`NextPBillNumber`, nextNumber,
			).
			StateRecord(
				orm.Package_untill.WDoc_bill,
				100002,
				`tableno`, 1,
			).
			ArgumentObject(
				2,
				`id_bill`, 100002,
				`id_untill_users`, 100001,
				`pdatetime`, date.UnixMicro(),
			).
			ArgumentObjectRow(`pbill_item`,
				3,
				`sys.ParentID`, 2,
				`id_pbill`, 100000,
				`id_untill_users`, 100001,
				`tableno`, 123,
				`quantity`, 2,
				`price`, 50_00,
			).
			IntentSingletonUpdate(
				orm.Package_air.WSingleton_NextNumbers,
				`NextPBillNumber`, nextNumber+1,
			).
			IntentRecordUpdate(
				orm.Package_untill.WDoc_bill,
				100002,
				`close_year`, date.Year(),
			).
			Run()
	})
}

//func TestFillPbillDates(t *testing.T) {
//	t.Parallel()
//
//	date := time.Date(2023, 1, 9, 0, 0, 0, 0, time.UTC)
//
//	t.Run("View View_PbillDates: insert", func(t *testing.T) {
//		test.NewProjectorTest(
//			t,
//			orm.Package_air.Projector_FillPbillDates(),
//			FillPbillDates,
//		).
//			//EventOffset(123).
//			EventArgumentObject(
//
//				2,
//				`id_bill`, 100002,
//				`id_untill_users`, 100001,
//				`pdatetime`, date.UnixMicro(),
//			).
//			EventArgumentObjectRow(`pbill_item`,
//				3,
//				`sys.ParentID`, 2,
//				`id_pbill`, 100000,
//				`id_untill_users`, 100001,
//				`tableno`, 123,
//				`quantity`, 2,
//				`price`, 50_00,
//			).
//			IntentViewInsert(
//				orm.Package_air.View_PbillDates,
//				`Year`, 2023,
//				`DayOfYear`, 9,
//				`FirstOffset`, 124,
//				`LastOffset`, 124,
//			).
//			Run()
//	})
//
//	t.Run("View View_PbillDates: update", func(t *testing.T) {
//		test.NewProjectorTest(
//			t,
//			orm.Package_air.Projector_FillPbillDates(),
//			FillPbillDates,
//		).
//			EventOffset(123).
//			StateView(
//				orm.Package_air.View_PbillDates,
//				110012,
//				`Year`, 2023,
//				`DayOfYear`, 9,
//				`FirstOffset`, 10,
//				`LastOffset`, 10,
//			).
//			EventArgumentObject(
//				2,
//				`id_bill`, 100002,
//				`id_untill_users`, 100001,
//				`pdatetime`, date.UnixMicro(),
//			).
//			EventArgumentObjectRow(`pbill_item`,
//				3,
//				`sys.ParentID`, 2,
//				`id_pbill`, 100000,
//				`id_untill_users`, 100001,
//				`tableno`, 123,
//				`quantity`, 2,
//				`price`, 50_00,
//			).
//			IntentViewUpdate(
//				orm.Package_air.View_PbillDates,
//				110012,
//				`Year`, 2023,
//				`DayOfYear`, 9,
//				`FirstOffset`, 10,
//				`LastOffset`, 124,
//			).
//			Run()
//	})
//}

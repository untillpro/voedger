/*
 * Copyright (c) 2024-present unTill Software Development Group B. V.
 * @author Maxim Geraskin
 */

package main

import (
	"time"

	ext "github.com/voedger/voedger/pkg/exttinygo"

	"air/wasm/orm"
)

// Command
//
//export Pbill
func Pbill() {

	// Query untill.pbill from the ArgumentObject
	{
		pbill := orm.Package_air.Command_Pbill.ArgumentObject()

		//Basic types fields
		pbill.Get_id_untill_users()

		// Container
		pbill_items := pbill.Get_pbill_item()
		for i := 0; i < pbill_items.Len(); i++ {
			item := pbill_items.Get(i)
			item.Get_price()
		}
	}

	// Prepare intent for Package_untill.WDoc_bill
	{
		pbill := orm.Package_air.Command_Pbill.ArgumentObject()

		// Basic types fields
		billID := pbill.Get_id_bill()
		intent := orm.Package_untill.WDoc_bill.Update(billID)
		intent.Set_close_year(int32(time.Now().UTC().Year()))
	}

	// Prepare intent for Package_air.WSingleton_NextNumbers
	{
		var nextNumber int32
		nextNumberValue, nextNumberOk := orm.Package_air.WSingleton_NextNumbers.Get()
		var intent orm.Intent_WSingleton_air_NextNumbers
		if !nextNumberOk {
			nextNumber = 0
			intent = nextNumberValue.Insert()
		} else {
			intent = nextNumberValue.Update() //orm.Package_air.WSingleton_NextNumbers.Update(nextNumberValue)
			nextNumber = nextNumberValue.Get_NextPBillNumber()
		}
		intent.Set_NextPBillNumber(nextNumber + 1)
	}
}

func FillPbillDates() {
	event := ext.MustGetValue(ext.KeyBuilder(ext.StorageEvent, ext.NullEntity))
	arg := event.AsValue("ArgumentObject")
	// extract offset and count from the argument
	offs := arg.AsInt64("Offset")
	count := arg.AsInt64("Count")

	// get pbill datetime
	pbillDatetime := time.UnixMicro(arg.AsInt64("pdatetime"))
	// extract year and day of year from pbill datetime
	year := pbillDatetime.Year()
	dayOfYear := pbillDatetime.Day()

	val, ok := orm.Package_air.View_PbillDates.Get(int32(year), int32(dayOfYear))
	if !ok {
		intent := val.Insert()
		intent.Set_FirstOffset(offs)
		intent.Set_LastOffset(offs + count)
	} else {
		intent := val.Update()
		intent.Set_FirstOffset(offs)
		intent.Set_LastOffset(offs + count)
	}
}

func main() {
	Pbill()
}

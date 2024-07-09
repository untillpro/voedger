// Code generated by vpm. DO NOT EDIT.

package orm

import "github.com/voedger/voedger/pkg/exttinygo"

// package type
type TPackage_untill struct {
	Path               string
	ODoc_orders        ODoc_untill_orders
	ODoc_pbill         ODoc_untill_pbill
	CDoc_articles      CDoc_untill_articles
	WDoc_bill          WDoc_untill_bill
	ORecord_pbill_item ORecord_untill_pbill_item
	CDoc_untill_users  CDoc_untill_untill_users
}

// package variables
var Package_untill = TPackage_untill{
	Path: "untill",
	ODoc_orders: ODoc_untill_orders{
		Type: Type{fQName: "untill.orders"},
	}, ODoc_pbill: ODoc_untill_pbill{
		Type: Type{fQName: "untill.pbill"},
	}, CDoc_articles: CDoc_untill_articles{
		Type: Type{fQName: "untill.articles"},
	}, WDoc_bill: WDoc_untill_bill{
		Type: Type{fQName: "untill.bill"},
	}, ORecord_pbill_item: ORecord_untill_pbill_item{
		Type: Type{fQName: "untill.pbill_item"},
	}, CDoc_untill_users: CDoc_untill_untill_users{
		Type: Type{fQName: "untill.untill_users"},
	},
}

type ODoc_untill_orders struct {
	Type
}

type Value_ODoc_untill_orders struct {
	tv exttinygo.TValue
}

type Intent_ODoc_untill_orders struct {
	intent exttinygo.TIntent
}

func (v Value_ODoc_untill_orders) Get_id_bill() ID {
	return ID(v.tv.AsInt64("id_bill"))
}

func (v Value_ODoc_untill_orders) Get_ord_tableno() int32 {
	return v.tv.AsInt32("ord_tableno")
}

func (i Intent_ODoc_untill_orders) Set_id_bill(value ID) Intent_ODoc_untill_orders {
	i.intent.PutInt64("id_bill", int64(value))
	return i
}

func (i Intent_ODoc_untill_orders) Set_ord_tableno(value int32) Intent_ODoc_untill_orders {
	i.intent.PutInt32("ord_tableno", value)
	return i
}

func (r ODoc_untill_orders) PkgPath() string {
	return Package_untill.Path
}

func (r ODoc_untill_orders) Entity() string {
	return "orders"
}

func (v ODoc_untill_orders) Insert(id ID) Intent_ODoc_untill_orders {
	kb := exttinygo.KeyBuilder(exttinygo.StorageRecord, v.fQName)
	kb.PutInt64(FieldName_ID, int64(id))
	return Intent_ODoc_untill_orders{intent: exttinygo.NewValue(kb)}
}

type ODoc_untill_pbill struct {
	Type
}

type Value_ODoc_untill_pbill struct {
	tv exttinygo.TValue
}

type Intent_ODoc_untill_pbill struct {
	intent exttinygo.TIntent
}

func (v Value_ODoc_untill_pbill) Get_pbill_item() Container_ORecord_untill_pbill_item {
	return Container_ORecord_untill_pbill_item{tv: v.tv.AsValue("pbill_item")}
}

func (v Value_ODoc_untill_pbill) Get_id_bill() ID {
	return ID(v.tv.AsInt64("id_bill"))
}

func (v Value_ODoc_untill_pbill) Get_id_untill_users() ID {
	return ID(v.tv.AsInt64("id_untill_users"))
}

func (v Value_ODoc_untill_pbill) Get_number() int32 {
	return v.tv.AsInt32("number")
}

func (i Intent_ODoc_untill_pbill) Set_id_bill(value ID) Intent_ODoc_untill_pbill {
	i.intent.PutInt64("id_bill", int64(value))
	return i
}

func (i Intent_ODoc_untill_pbill) Set_id_untill_users(value ID) Intent_ODoc_untill_pbill {
	i.intent.PutInt64("id_untill_users", int64(value))
	return i
}

func (i Intent_ODoc_untill_pbill) Set_number(value int32) Intent_ODoc_untill_pbill {
	i.intent.PutInt32("number", value)
	return i
}

func (r ODoc_untill_pbill) PkgPath() string {
	return Package_untill.Path
}

func (r ODoc_untill_pbill) Entity() string {
	return "pbill"
}

func (v ODoc_untill_pbill) Insert(id ID) Intent_ODoc_untill_pbill {
	kb := exttinygo.KeyBuilder(exttinygo.StorageRecord, v.fQName)
	kb.PutInt64(FieldName_ID, int64(id))
	return Intent_ODoc_untill_pbill{intent: exttinygo.NewValue(kb)}
}

type CDoc_untill_articles struct {
	Type
}

type Value_CDoc_untill_articles struct {
	tv exttinygo.TValue
}

type Intent_CDoc_untill_articles struct {
	intent exttinygo.TIntent
}

func (v Value_CDoc_untill_articles) Get_article_number() int32 {
	return v.tv.AsInt32("article_number")
}

func (v Value_CDoc_untill_articles) Get_name() string {
	return v.tv.AsString("name")
}

func (r CDoc_untill_articles) PkgPath() string {
	return Package_untill.Path
}

func (r CDoc_untill_articles) Entity() string {
	return "articles"
}

func (v CDoc_untill_articles) MustGet(id ID) Value_CDoc_untill_articles {
	kb := exttinygo.KeyBuilder(exttinygo.StorageRecord, v.fQName)
	kb.PutInt64(FieldName_ID, int64(id))
	return Value_CDoc_untill_articles{tv: exttinygo.MustGetValue(kb)}
}

func (v CDoc_untill_articles) Get(id ID) (Value_CDoc_untill_articles, bool) {
	kb := exttinygo.KeyBuilder(exttinygo.StorageRecord, v.fQName)
	kb.PutInt64(FieldName_ID, int64(id))
	tv, exists := exttinygo.QueryValue(kb)
	return Value_CDoc_untill_articles{tv: tv}, exists
}

type WDoc_untill_bill struct {
	Type
}

type Value_WDoc_untill_bill struct {
	tv exttinygo.TValue
	kb exttinygo.TKeyBuilder
}

type Intent_WDoc_untill_bill struct {
	intent exttinygo.TIntent
}

func (v Value_WDoc_untill_bill) Get_tableno() int32 {
	return v.tv.AsInt32("tableno")
}

func (v Value_WDoc_untill_bill) Get_id_untill_users() ID {
	return ID(v.tv.AsInt64("id_untill_users"))
}

func (v Value_WDoc_untill_bill) Get_close_datetime() int64 {
	return v.tv.AsInt64("close_datetime")
}

func (v Value_WDoc_untill_bill) Get_total() int64 {
	return v.tv.AsInt64("total")
}

func (i Intent_WDoc_untill_bill) Set_tableno(value int32) Intent_WDoc_untill_bill {
	i.intent.PutInt32("tableno", value)
	return i
}

func (i Intent_WDoc_untill_bill) Set_id_untill_users(value ID) Intent_WDoc_untill_bill {
	i.intent.PutInt64("id_untill_users", int64(value))
	return i
}

func (i Intent_WDoc_untill_bill) Set_close_datetime(value int64) Intent_WDoc_untill_bill {
	i.intent.PutInt64("close_datetime", value)
	return i
}

func (i Intent_WDoc_untill_bill) Set_total(value int64) Intent_WDoc_untill_bill {
	i.intent.PutInt64("total", value)
	return i
}

func (r WDoc_untill_bill) PkgPath() string {
	return Package_untill.Path
}

func (r WDoc_untill_bill) Entity() string {
	return "bill"
}

func (v WDoc_untill_bill) Insert(id ID) Intent_WDoc_untill_bill {
	kb := exttinygo.KeyBuilder(exttinygo.StorageRecord, v.fQName)
	kb.PutInt64(FieldName_ID, int64(id))
	return Intent_WDoc_untill_bill{intent: exttinygo.NewValue(kb)}
}

func (v WDoc_untill_bill) Update(id ID) Intent_WDoc_untill_bill {
	existingValue := v.MustGet(id)
	kb := exttinygo.KeyBuilder(exttinygo.StorageRecord, v.fQName)
	kb.PutInt64(FieldName_ID, int64(id))
	return Intent_WDoc_untill_bill{intent: exttinygo.UpdateValue(kb, existingValue.tv)}
}

func (v WDoc_untill_bill) Get(id ID) (Value_WDoc_untill_bill, bool) {
	kb := exttinygo.KeyBuilder(exttinygo.StorageRecord, v.fQName)
	kb.PutInt64(FieldName_ID, int64(id))
	tv, exists := exttinygo.QueryValue(kb)
	return Value_WDoc_untill_bill{tv: tv, kb: kb}, exists
}

func (v WDoc_untill_bill) MustGet(id ID) Value_WDoc_untill_bill {
	kb := exttinygo.KeyBuilder(exttinygo.StorageRecord, v.fQName)
	kb.PutInt64(FieldName_ID, int64(id))
	tv := exttinygo.MustGetValue(kb)
	return Value_WDoc_untill_bill{tv: tv, kb: kb}
}

func (v Value_WDoc_untill_bill) Insert() Intent_WDoc_untill_bill {
	return Intent_WDoc_untill_bill{intent: exttinygo.NewValue(v.kb)}
}

func (v Value_WDoc_untill_bill) Update() Intent_WDoc_untill_bill {
	return Intent_WDoc_untill_bill{intent: exttinygo.UpdateValue(v.kb, v.tv)}
}

type ORecord_untill_pbill_item struct {
	Type
}

type Value_ORecord_untill_pbill_item struct {
	tv exttinygo.TValue
}

type Container_ORecord_untill_pbill_item struct {
	tv  exttinygo.TValue
	len int
}

type Intent_ORecord_untill_pbill_item struct {
	intent exttinygo.TIntent
}

func (v Value_ORecord_untill_pbill_item) Get_sys_ParentID() ID {
	return ID(v.tv.AsInt64("sys_ParentID"))
}

func (v Value_ORecord_untill_pbill_item) Get_sys_Container() string {
	return v.tv.AsString("sys_Container")
}

func (v Value_ORecord_untill_pbill_item) Get_id_pbill() ID {
	return ID(v.tv.AsInt64("id_pbill"))
}

func (v Value_ORecord_untill_pbill_item) Get_id_untill_users() ID {
	return ID(v.tv.AsInt64("id_untill_users"))
}

func (v Value_ORecord_untill_pbill_item) Get_tableno() int32 {
	return v.tv.AsInt32("tableno")
}

func (v Value_ORecord_untill_pbill_item) Get_quantity() int32 {
	return v.tv.AsInt32("quantity")
}

func (v Value_ORecord_untill_pbill_item) Get_price() int64 {
	return v.tv.AsInt64("price")
}

func (r ORecord_untill_pbill_item) PkgPath() string {
	return Package_untill.Path
}

func (r ORecord_untill_pbill_item) Entity() string {
	return "pbill_item"
}

func (v *Container_ORecord_untill_pbill_item) Len() int {
	if v.len == 0 {
		v.len = v.tv.Len() + 1
	}

	return v.len - 1
}

func (v *Container_ORecord_untill_pbill_item) Get(i int) Value_ORecord_untill_pbill_item {
	return Value_ORecord_untill_pbill_item{tv: v.tv.GetAsValue(i)}
}

type CDoc_untill_untill_users struct {
	Type
}

type Value_CDoc_untill_untill_users struct {
	tv exttinygo.TValue
}

type Intent_CDoc_untill_untill_users struct {
	intent exttinygo.TIntent
}

func (v Value_CDoc_untill_untill_users) Get_name() string {
	return v.tv.AsString("name")
}

func (v Value_CDoc_untill_untill_users) Get_phone() string {
	return v.tv.AsString("phone")
}

func (v Value_CDoc_untill_untill_users) Get_email() string {
	return v.tv.AsString("email")
}

func (r CDoc_untill_untill_users) PkgPath() string {
	return Package_untill.Path
}

func (r CDoc_untill_untill_users) Entity() string {
	return "untill_users"
}

func (v CDoc_untill_untill_users) MustGet(id ID) Value_CDoc_untill_untill_users {
	kb := exttinygo.KeyBuilder(exttinygo.StorageRecord, v.fQName)
	kb.PutInt64(FieldName_ID, int64(id))
	return Value_CDoc_untill_untill_users{tv: exttinygo.MustGetValue(kb)}
}

func (v CDoc_untill_untill_users) Get(id ID) (Value_CDoc_untill_untill_users, bool) {
	kb := exttinygo.KeyBuilder(exttinygo.StorageRecord, v.fQName)
	kb.PutInt64(FieldName_ID, int64(id))
	tv, exists := exttinygo.QueryValue(kb)
	return Value_CDoc_untill_untill_users{tv: tv}, exists
}

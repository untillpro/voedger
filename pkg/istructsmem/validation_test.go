/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package istructsmem

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/iratesce"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem/internal/containers"
	"github.com/voedger/voedger/pkg/itokens"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
)

func Test_ValidElement(t *testing.T) {
	require := require.New(t)

	test := test()

	appDef := appdef.New()

	t.Run("must be ok to build test application", func(t *testing.T) {

		objName := appdef.NewQName("test", "object")
		elName := appdef.NewQName("test", "element")
		gcName := appdef.NewQName("test", "grandChild")

		docName := appdef.NewQName("test", "document")
		recName := appdef.NewQName("test", "record")

		t.Run("build object type", func(t *testing.T) {
			obj := appDef.AddObject(objName)
			obj.
				AddField("RequiredField", appdef.DataKind_int32, true)
			obj.
				AddContainer("child", elName, 1, appdef.Occurs_Unbounded)

			el := appDef.AddElement(elName)
			el.
				AddField("RequiredField", appdef.DataKind_int32, true)
			el.
				AddContainer("grandChild", gcName, 0, 1)

			subEl := appDef.AddElement(gcName)
			subEl.
				AddRefField("recIDField", false)
		})

		t.Run("build ODoc type", func(t *testing.T) {
			doc := appDef.AddODoc(docName)
			doc.
				AddField("RequiredField", appdef.DataKind_int32, true)
			doc.
				AddContainer("child", recName, 1, appdef.Occurs_Unbounded)

			rec := appDef.AddORecord(recName)
			rec.
				AddField("RequiredField", appdef.DataKind_int32, true).
				AddRefField("recIDField", false, recName)
		})
	})

	cfgs := make(AppConfigsType, 1)
	cfg := cfgs.AddConfig(test.appName, appDef)

	storage, err := simpleStorageProvider().AppStorage(istructs.AppQName_test1_app1)
	require.NoError(err)
	err = cfg.prepare(iratesce.TestBucketsFactory(), storage)
	require.NoError(err)

	t.Run("test build object", func(t *testing.T) {
		t.Run("must error if null-name object", func(t *testing.T) {
			obj := func() istructs.IObjectBuilder {
				o := makeObject(cfg, appdef.NullQName)
				return &o
			}()
			_, err := obj.Build()
			require.ErrorIs(err, ErrNameMissed)
		})

		t.Run("must error if unknown-name object", func(t *testing.T) {
			obj := func() istructs.IObjectBuilder {
				o := makeObject(cfg, appdef.NewQName("test", "unknownDef"))
				return &o
			}()
			_, err := obj.Build()
			require.ErrorIs(err, ErrNameNotFound)
		})

		t.Run("must error if invalid type kind object", func(t *testing.T) {
			obj := func() istructs.IObjectBuilder {
				o := makeObject(cfg, appdef.NewQName("test", "element"))
				return &o
			}()
			_, err := obj.Build()
			require.ErrorIs(err, ErrUnexpectedTypeKind)
		})

		obj := func() istructs.IObjectBuilder {
			o := makeObject(cfg, appdef.NewQName("test", "object"))
			return &o
		}()

		t.Run("must error if empty object", func(t *testing.T) {
			_, err := obj.Build()
			require.ErrorIs(err, ErrNameNotFound)
		})

		obj.PutInt32("RequiredField", 555)
		t.Run("must error if no nested child", func(t *testing.T) {
			_, err := obj.Build()
			require.ErrorIs(err, ErrMinOccursViolation)
		})

		child := obj.ElementBuilder("child")
		t.Run("must error if nested child has no required field", func(t *testing.T) {
			_, err := obj.Build()
			require.ErrorIs(err, ErrNameNotFound)
		})

		child.PutInt32("RequiredField", 777)
		t.Run("must have no error if ok", func(t *testing.T) {
			_, err := obj.Build()
			require.NoError(err)
		})

		gChild := child.ElementBuilder("grandChild")
		require.NotNil(gChild)

		t.Run("must ok grand children", func(t *testing.T) {
			_, err := obj.Build()
			require.NoError(err)
		})

		t.Run("must error if unknown child name", func(t *testing.T) {
			gChild.PutString(appdef.SystemField_Container, "unknownName")
			_, err := obj.Build()
			require.ErrorIs(err, containers.ErrContainerNotFound)
		})
	})

	t.Run("test build operation document", func(t *testing.T) {
		doc := func() istructs.IObjectBuilder {
			o := makeObject(cfg, appdef.NewQName("test", "document"))
			return &o
		}()
		require.NotNil(doc)

		t.Run("must error if empty document", func(t *testing.T) {
			_, err := doc.Build()
			require.ErrorIs(err, ErrNameNotFound)
		})

		doc.PutRecordID(appdef.SystemField_ID, 1)
		doc.PutInt32("RequiredField", 555)
		t.Run("must error if no nested document record", func(t *testing.T) {
			_, err := doc.Build()
			require.ErrorIs(err, ErrMinOccursViolation)
		})

		rec := doc.ElementBuilder("child")
		require.NotNil(rec)

		t.Run("must error if empty child record", func(t *testing.T) {
			_, err := doc.Build()
			require.ErrorIs(err, ErrNameNotFound)
		})

		t.Run("must error if raw ID duplication", func(t *testing.T) {
			rec.PutRecordID(appdef.SystemField_ID, 1)
			_, err := doc.Build()
			require.ErrorIs(err, ErrRecordIDUniqueViolation)
			require.ErrorContains(err, "repeatedly uses record ID «1»")
		})

		rec.PutRecordID(appdef.SystemField_ID, 2)
		rec.PutInt32("RequiredField", 555)

		t.Run("must error if wrong record parent", func(t *testing.T) {
			rec.PutRecordID(appdef.SystemField_ParentID, 77)
			_, err := doc.Build()
			require.ErrorIs(err, ErrWrongRecordID)
		})

		t.Run("must automatically restore parent if empty record parent", func(t *testing.T) {
			rec.PutRecordID(appdef.SystemField_ParentID, istructs.NullRecordID)
			_, err := doc.Build()
			require.NoError(err)
		})

		t.Run("must error if unknown raw ID ref", func(t *testing.T) {
			rec.PutRecordID("recIDField", 7)
			_, err := doc.Build()
			require.ErrorIs(err, ErrRecordIDNotFound)
			require.ErrorContains(err, "unknown record ID «7»")
		})

		t.Run("must error if raw ID refs to invalid target", func(t *testing.T) {
			rec.PutRecordID("recIDField", 1)
			_, err := doc.Build()
			require.ErrorIs(err, ErrWrongRecordID)
			require.ErrorContains(err, "record ID «1»")
			require.ErrorContains(err, "unavailable target QName «test.document»")

			rec.PutRecordID("recIDField", 2) // fix last error
			_, err = doc.Build()
			require.NoError(err)
		})
	})
}

func Test_ValidEventArgs(t *testing.T) {
	require := require.New(t)

	appDef := appdef.New()

	docName := appdef.NewQName("test", "document")
	rec1Name := appdef.NewQName("test", "record1")
	rec2Name := appdef.NewQName("test", "record2")

	objName := appdef.NewQName("test", "object")
	elName := appdef.NewQName("test", "element")

	t.Run("must be ok to build test application", func(t *testing.T) {
		doc := appDef.AddODoc(docName)
		doc.
			AddField("RequiredField", appdef.DataKind_int32, true).
			AddRefField("RefField", false, rec1Name)
		doc.
			AddContainer("child", rec1Name, 0, 1).
			AddContainer("child2", rec2Name, 0, appdef.Occurs_Unbounded)

		_ = appDef.AddORecord(rec1Name)

		rec2 := appDef.AddORecord(rec2Name)
		rec2.AddRefField("RequiredRefField", true, rec2Name)

		obj := appDef.AddObject(objName)
		obj.AddContainer("childElement", elName, 0, appdef.Occurs_Unbounded)

		_ = appDef.AddElement(elName)
	})

	cfgs := make(AppConfigsType, 1)
	_ = cfgs.AddConfig(istructs.AppQName_test1_app1, appDef)

	provider := Provide(cfgs, iratesce.TestBucketsFactory, testTokensFactory(), simpleStorageProvider())

	app, err := provider.AppStructs(istructs.AppQName_test1_app1)
	require.NoError(err)

	t.Run("error if event name is not a command or odoc", func(t *testing.T) {
		b := app.Events().GetNewRawEventBuilder(
			istructs.NewRawEventBuilderParams{
				GenericRawEventBuilderParams: istructs.GenericRawEventBuilderParams{
					HandlingPartition: 25,
					PLogOffset:        100500,
					Workspace:         1,
					WLogOffset:        1050,
					QName:             objName, // <- error here
					RegisteredAt:      123456789,
				}})

		_, err := b.BuildRawEvent()
		require.ErrorIs(err, ErrNameNotFound)
		require.ErrorContains(err, "command function «test.object» not found")
	})

	oDocEvent := func(sync bool) *eventType {
		var b istructs.IRawEventBuilder
		if sync {
			b = app.Events().GetSyncRawEventBuilder(
				istructs.SyncRawEventBuilderParams{
					GenericRawEventBuilderParams: istructs.GenericRawEventBuilderParams{
						HandlingPartition: 25,
						PLogOffset:        100500,
						Workspace:         1,
						WLogOffset:        1050,
						QName:             docName,
						RegisteredAt:      123456789,
					},
					Device:   1,
					SyncedAt: 123456789,
				})
		} else {
			b = app.Events().GetNewRawEventBuilder(
				istructs.NewRawEventBuilderParams{
					GenericRawEventBuilderParams: istructs.GenericRawEventBuilderParams{
						HandlingPartition: 25,
						PLogOffset:        100500,
						Workspace:         1,
						WLogOffset:        1050,
						QName:             docName,
						RegisteredAt:      123456789,
					},
				})
		}
		return b.(*eventType)
	}

	t.Run("error if empty doc", func(t *testing.T) {
		e := oDocEvent(false)
		_, err := e.BuildRawEvent()
		require.ErrorIs(err, ErrNameNotFound)
		require.ErrorContains(err, "ODoc «test.document» misses required field «sys.ID»")
	})

	t.Run("errors in argument IDs and refs", func(t *testing.T) {

		t.Run("error if not raw ID in new event", func(t *testing.T) {
			e := oDocEvent(false)
			doc := e.ArgumentObjectBuilder()
			doc.PutRecordID(appdef.SystemField_ID, 123456789012345) // <- error here
			doc.PutInt32("RequiredField", 7)
			_, err := e.BuildRawEvent()
			require.ErrorIs(err, ErrRawRecordIDRequired)
			require.ErrorContains(err, "ODoc «test.document» should use raw record ID (not «123456789012345»)")
		})

		t.Run("error if repeatedly uses record ID", func(t *testing.T) {
			e := oDocEvent(false)
			doc := e.ArgumentObjectBuilder()
			doc.PutRecordID(appdef.SystemField_ID, 1)
			doc.PutInt32("RequiredField", 7)
			rec := doc.ElementBuilder("child")
			rec.PutRecordID(appdef.SystemField_ID, 1) // <- error here
			_, err := e.BuildRawEvent()
			require.ErrorIs(err, ErrRecordIDUniqueViolation)
			require.ErrorContains(err, "ODoc «test.document» repeatedly uses record ID «1» in ORecord «child: test.record1»")
		})

		t.Run("error if ref to unknown id", func(t *testing.T) {
			e := oDocEvent(false)
			doc := e.ArgumentObjectBuilder()
			doc.PutRecordID(appdef.SystemField_ID, 1)
			doc.PutInt32("RequiredField", 7)
			doc.PutRecordID("RefField", 7) // <- error here
			_, err := e.BuildRawEvent()
			require.ErrorIs(err, ErrRecordIDNotFound)
			require.ErrorContains(err, "ODoc «test.document» field «RefField» refers to unknown record ID «7»")
		})

		t.Run("error if ref to id from invalid target QName", func(t *testing.T) {
			e := oDocEvent(false)
			doc := e.ArgumentObjectBuilder()
			doc.PutRecordID(appdef.SystemField_ID, 1)
			doc.PutInt32("RequiredField", 7)
			doc.PutRecordID("RefField", 1) // <- error here
			_, err := e.BuildRawEvent()
			require.ErrorIs(err, ErrWrongRecordID)
			require.ErrorContains(err, "ODoc «test.document» field «RefField» refers to record ID «1» that has unavailable target QName «test.document»")
		})
	})

	t.Run("error if invalid argument QName", func(t *testing.T) {
		e := oDocEvent(false)
		doc := e.ArgumentObjectBuilder()

		doc.(interface{ clear() }).clear()

		doc.PutQName(appdef.SystemField_QName, rec1Name) // <- error here
		doc.PutRecordID(appdef.SystemField_ID, 1)
		_, err := e.BuildRawEvent()
		require.ErrorIs(err, ErrWrongType)
		require.ErrorContains(err, "event «test.document» argument uses wrong type «test.record1», expected «test.document»")
	})

	t.Run("error if invalid unlogged argument QName", func(t *testing.T) {
		e := oDocEvent(false)
		doc := e.ArgumentObjectBuilder()
		doc.PutRecordID(appdef.SystemField_ID, 1)
		doc.PutInt32("RequiredField", 7)

		unl := e.ArgumentUnloggedObjectBuilder()
		unl.PutQName(appdef.SystemField_QName, objName) // <- error here

		_, err := e.BuildRawEvent()
		require.ErrorIs(err, ErrWrongType)
		require.ErrorContains(err, "event «test.document» unlogged argument uses wrong type «test.object»")
	})

	t.Run("error if argument not valid", func(t *testing.T) {

		t.Run("error if misses required field", func(t *testing.T) {
			e := oDocEvent(false)
			doc := e.ArgumentObjectBuilder()
			doc.PutRecordID(appdef.SystemField_ID, 1)
			_, err := e.BuildRawEvent()
			require.ErrorIs(err, ErrNameNotFound)
			require.ErrorContains(err, "ODoc «test.document» misses required field «RequiredField»")
		})

		t.Run("error if required ref field filled with NullRecordID", func(t *testing.T) {
			e := oDocEvent(false)
			doc := e.ArgumentObjectBuilder()
			doc.PutRecordID(appdef.SystemField_ID, 1)
			doc.PutInt32("RequiredField", 7)
			rec := doc.ElementBuilder("child2")
			rec.PutRecordID(appdef.SystemField_ID, 2)
			rec.PutRecordID("RequiredRefField", istructs.NullRecordID) // <- error here
			_, err := e.BuildRawEvent()
			require.ErrorIs(err, ErrWrongRecordID)
			require.ErrorContains(err, "ORecord «child2: test.record2» required ref field «RequiredRefField» has NullRecordID")
		})

		t.Run("error if corrupted argument container structure", func(t *testing.T) {

			t.Run("error if max occurs exceeded", func(t *testing.T) {
				e := oDocEvent(false)
				doc := e.ArgumentObjectBuilder()
				doc.PutRecordID(appdef.SystemField_ID, 1)
				doc.PutInt32("RequiredField", 7)

				doc.ElementBuilder("child").
					PutRecordID(appdef.SystemField_ID, 2)

				doc.ElementBuilder("child").
					PutRecordID(appdef.SystemField_ID, 3) // <- error here

				_, err := e.BuildRawEvent()
				require.ErrorIs(err, ErrMaxOccursViolation)
				require.ErrorContains(err, "ODoc «test.document» container «child» has too many occurrences (2, maximum 1)")
			})

			t.Run("error if unknown container used", func(t *testing.T) {
				e := oDocEvent(false)
				doc := e.ArgumentObjectBuilder()
				doc.PutRecordID(appdef.SystemField_ID, 1)
				doc.PutInt32("RequiredField", 7)

				rec := doc.ElementBuilder("child")
				rec.PutRecordID(appdef.SystemField_ID, 2)
				rec.PutString(appdef.SystemField_Container, "childElement") // <- error here
				_, err := e.BuildRawEvent()
				require.ErrorIs(err, ErrNameNotFound)
				require.ErrorContains(err, "ODoc «test.document» child[0] has unknown container name «childElement»")
			})

			t.Run("error if invalid QName used for container", func(t *testing.T) {
				e := oDocEvent(false)
				doc := e.ArgumentObjectBuilder()
				doc.PutRecordID(appdef.SystemField_ID, 1)
				doc.PutInt32("RequiredField", 7)

				rec := doc.ElementBuilder("child")
				rec.PutRecordID(appdef.SystemField_ID, 2)
				rec.PutString(appdef.SystemField_Container, "child2") // <- error here
				_, err := e.BuildRawEvent()
				require.ErrorIs(err, ErrWrongType)
				require.ErrorContains(err, "ODoc «test.document» child[0] ORecord «child2: test.record1» has wrong type name, expected «test.record2»")
			})
		})
	})
}

func Test_ValidSysCudEvent(t *testing.T) {
	require := require.New(t)

	appDef := appdef.New()

	docName := appdef.NewQName("test", "document")
	rec1Name := appdef.NewQName("test", "record1")
	rec2Name := appdef.NewQName("test", "record2")

	objName := appdef.NewQName("test", "object")
	elName := appdef.NewQName("test", "element")

	t.Run("must be ok to build test application", func(t *testing.T) {
		doc := appDef.AddCDoc(docName)
		doc.
			AddField("RequiredField", appdef.DataKind_int32, true).
			AddRefField("RefField", false, rec1Name)
		doc.
			AddContainer("child", rec1Name, 0, appdef.Occurs_Unbounded).
			AddContainer("child2", rec2Name, 0, appdef.Occurs_Unbounded)

		_ = appDef.AddCRecord(rec1Name)
		_ = appDef.AddCRecord(rec2Name)

		obj := appDef.AddObject(objName)
		obj.AddContainer("childElement", elName, 0, appdef.Occurs_Unbounded)

		_ = appDef.AddElement(elName)
	})

	cfgs := make(AppConfigsType, 1)
	cfg := cfgs.AddConfig(istructs.AppQName_test1_app1, appDef)

	provider := Provide(cfgs, iratesce.TestBucketsFactory, testTokensFactory(), simpleStorageProvider())

	app, err := provider.AppStructs(istructs.AppQName_test1_app1)
	require.NoError(err)

	cudRawEvent := func(sync bool) *eventType {
		var b istructs.IRawEventBuilder
		if sync {
			b = app.Events().GetSyncRawEventBuilder(
				istructs.SyncRawEventBuilderParams{
					GenericRawEventBuilderParams: istructs.GenericRawEventBuilderParams{
						HandlingPartition: 25,
						PLogOffset:        100500,
						Workspace:         1,
						WLogOffset:        1050,
						QName:             istructs.QNameCommandCUD,
						RegisteredAt:      123456789,
					},
					Device:   1,
					SyncedAt: 123456789,
				})
		} else {
			b = app.Events().GetNewRawEventBuilder(
				istructs.NewRawEventBuilderParams{
					GenericRawEventBuilderParams: istructs.GenericRawEventBuilderParams{
						HandlingPartition: 25,
						PLogOffset:        100500,
						Workspace:         1,
						WLogOffset:        1050,
						QName:             istructs.QNameCommandCUD,
						RegisteredAt:      123456789,
					},
				})
		}
		return b.(*eventType)
	}

	testDocRec := func(id istructs.RecordID) istructs.IRecord {
		r := newRecord(cfg)
		r.PutQName(appdef.SystemField_QName, docName)
		r.PutRecordID(appdef.SystemField_ID, id)
		r.PutInt32("RequiredField", 7)
		require.NoError(r.build())
		return r
	}

	t.Run("error if empty CUD", func(t *testing.T) {
		e := cudRawEvent(false)
		_, err := e.BuildRawEvent()
		require.ErrorIs(err, ErrCUDsMissed)
	})

	t.Run("must error if empty CUD QName", func(t *testing.T) {
		e := cudRawEvent(false)
		_ = e.CUDBuilder().Create(appdef.NullQName) // <- error here
		_, err := e.BuildRawEvent()
		require.ErrorIs(err, ErrUnexpectedTypeKind)
		require.ErrorContains(err, "null row")
	})

	t.Run("must error if wrong CUD type kind", func(t *testing.T) {
		e := cudRawEvent(false)
		_ = e.CUDBuilder().Create(objName) // <- error here
		_, err := e.BuildRawEvent()
		require.ErrorIs(err, ErrUnexpectedTypeKind)
		require.ErrorContains(err, "Object «test.object»")
	})

	t.Run("test raw IDs in CUD.Create", func(t *testing.T) {

		t.Run("must require for new raw event", func(t *testing.T) {
			e := cudRawEvent(false)
			rec := e.CUDBuilder().Create(docName)
			rec.PutRecordID(appdef.SystemField_ID, 123456789012345) // <- error here
			_, err := e.BuildRawEvent()
			require.ErrorIs(err, ErrRawRecordIDRequired)
			require.ErrorContains(err, "should use raw record ID (not «123456789012345»)")
		})

		t.Run("no error for sync events", func(t *testing.T) {
			e := cudRawEvent(true)
			d := e.CUDBuilder().Create(docName)
			d.PutRecordID(appdef.SystemField_ID, 123456789012345)
			d.PutInt32("RequiredField", 1)
			_, err := e.BuildRawEvent()
			require.NoError(err)
		})
	})

	t.Run("must error if raw id in CUD.Update", func(t *testing.T) {
		e := cudRawEvent(false)
		_ = e.CUDBuilder().Update(testDocRec(1)) // <- error here
		_, err := e.BuildRawEvent()
		require.ErrorIs(err, ErrRawRecordIDUnexpected)
		require.ErrorContains(err, "unexpectedly uses raw record ID «1»")
	})

	t.Run("must error if ID duplication", func(t *testing.T) {

		t.Run("raw ID duplication", func(t *testing.T) {
			e := cudRawEvent(false)
			d := e.CUDBuilder().Create(docName)
			d.PutRecordID(appdef.SystemField_ID, 1)
			d.PutInt32("RequiredField", 7)

			r := e.CUDBuilder().Create(rec1Name)
			r.PutRecordID(appdef.SystemField_ParentID, 1)
			r.PutString(appdef.SystemField_Container, "child")
			r.PutRecordID(appdef.SystemField_ID, 1) // <- error here

			_, err := e.BuildRawEvent()
			require.ErrorIs(err, ErrRecordIDUniqueViolation)
			require.ErrorContains(err, "repeatedly uses record ID «1»")
		})

		t.Run("storage ID duplication in Update", func(t *testing.T) {
			e := cudRawEvent(true)

			c := e.CUDBuilder().Create(docName)
			c.PutRecordID(appdef.SystemField_ID, 123456789012345)
			c.PutInt32("RequiredField", 7)

			u := e.CUDBuilder().Update(testDocRec(123456789012345)) // <- error here
			u.PutInt32("RequiredField", 7)

			_, err := e.BuildRawEvent()
			require.ErrorIs(err, ErrRecordIDUniqueViolation)
			require.ErrorContains(err, "repeatedly uses record ID «123456789012345»")
		})

	})

	t.Run("must error if invalid ID refs", func(t *testing.T) {

		t.Run("must error if unknown ID refs", func(t *testing.T) {
			e := cudRawEvent(false)
			d := e.CUDBuilder().Create(docName)
			d.PutRecordID(appdef.SystemField_ID, 1)
			d.PutInt32("RequiredField", 1)
			d.PutRecordID("RefField", 7) // <- error here

			_, err := e.BuildRawEvent()
			require.ErrorIs(err, ErrRecordIDNotFound)
			require.ErrorContains(err, "unknown record ID «7»")
		})

		t.Run("must error if ID refs to invalid QName", func(t *testing.T) {
			e := cudRawEvent(false)
			d := e.CUDBuilder().Create(docName)
			d.PutRecordID(appdef.SystemField_ID, 1)
			d.PutInt32("RequiredField", 1)
			d.PutRecordID("RefField", 1) // <- error here

			_, err := e.BuildRawEvent()
			require.ErrorIs(err, ErrWrongRecordID)
			require.ErrorContains(err, "refers to record ID «1» that has unavailable target QName «test.document»")
		})

		t.Run("must error if sys.Parent / sys.Container causes invalid hierarchy", func(t *testing.T) {

			t.Run("must error if container unknown for specified ParentID", func(t *testing.T) {
				e := cudRawEvent(false)
				d := e.CUDBuilder().Create(docName)
				d.PutRecordID(appdef.SystemField_ID, 1)
				d.PutInt32("RequiredField", 1)

				r := e.CUDBuilder().Create(rec1Name)
				r.PutRecordID(appdef.SystemField_ID, 2)
				r.PutRecordID(appdef.SystemField_ParentID, 1)
				r.PutString(appdef.SystemField_Container, "childElement") // <- error here

				_, err := e.BuildRawEvent()
				require.ErrorIs(err, ErrWrongRecordID)
				require.ErrorContains(err, "has no container «childElement»")
			})

			t.Run("must error if specified container has another QName", func(t *testing.T) {
				e := cudRawEvent(false)
				d := e.CUDBuilder().Create(docName)
				d.PutRecordID(appdef.SystemField_ID, 1)
				d.PutInt32("RequiredField", 1)

				c := e.CUDBuilder().Create(rec1Name)
				c.PutRecordID(appdef.SystemField_ID, 2)
				c.PutRecordID(appdef.SystemField_ParentID, 1)
				c.PutString(appdef.SystemField_Container, "child2") // <- error here

				_, err := e.BuildRawEvent()
				require.ErrorIs(err, ErrWrongRecordID)
				require.ErrorContains(err, "container «child2» has another QName «test.record2»")
			})
		})
	})
}

func Test_VerifiedFields(t *testing.T) {
	require := require.New(t)
	test := test()

	objName := appdef.NewQName("test", "obj")

	appDef := appdef.New()
	t.Run("must be ok to build application", func(t *testing.T) {
		appDef.AddObject(objName).
			AddField("int32", appdef.DataKind_int32, true).
			AddStringField("email", false).
			SetFieldVerify("email", appdef.VerificationKind_EMail).
			AddField("age", appdef.DataKind_int32, false).
			SetFieldVerify("age", appdef.VerificationKind_Any...)
	})

	cfgs := make(AppConfigsType, 1)
	cfg := cfgs.AddConfig(test.appName, appDef)

	email := "test@test.io"

	tokens := testTokensFactory().New(test.appName)
	storage, err := simpleStorageProvider().AppStorage(istructs.AppQName_test1_app1)
	require.NoError(err)
	asp := Provide(cfgs, iratesce.TestBucketsFactory, testTokensFactory(), simpleStorageProvider())
	err = cfg.prepare(iratesce.TestBucketsFactory(), storage)
	require.NoError(err)
	_, err = asp.AppStructs(test.appName) // need to set cfg.app because IAppTokens are taken from cfg.app
	require.NoError(err)

	t.Run("test row verification", func(t *testing.T) {

		t.Run("ok verified value type in token", func(t *testing.T) {
			okEmailToken := func() string {
				p := payloads.VerifiedValuePayload{
					VerificationKind: appdef.VerificationKind_EMail,
					Entity:           objName,
					Field:            "email",
					Value:            email,
				}
				token, err := tokens.IssueToken(time.Minute, &p)
				require.NoError(err)
				return token
			}()

			okAgeToken := func() string {
				p := payloads.VerifiedValuePayload{
					VerificationKind: appdef.VerificationKind_Phone,
					Entity:           objName,
					Field:            "age",
					Value:            7,
				}
				token, err := tokens.IssueToken(time.Minute, &p)
				require.NoError(err)
				return token
			}()

			row := makeObject(cfg, objName)
			row.PutInt32("int32", 1)
			row.PutString("email", okEmailToken)
			row.PutString("age", okAgeToken)

			_, err := row.Build()
			require.NoError(err)
		})

		t.Run("error if not token, but not string value", func(t *testing.T) {

			row := makeObject(cfg, objName)
			row.PutInt32("int32", 1)
			row.PutInt32("age", 7)

			_, err := row.Build()
			require.ErrorIs(err, ErrWrongFieldType)
		})

		t.Run("error if not a token, but plain string value", func(t *testing.T) {

			row := makeObject(cfg, objName)
			row.PutInt32("int32", 1)
			row.PutString("email", email)

			_, err := row.Build()
			require.ErrorIs(err, itokens.ErrInvalidToken)
		})

		t.Run("error if unexpected token kind", func(t *testing.T) {
			ukToken := func() string {
				p := payloads.VerifiedValuePayload{
					VerificationKind: appdef.VerificationKind_Phone,
					Entity:           objName,
					Field:            "email",
					Value:            email,
				}
				token, err := tokens.IssueToken(time.Minute, &p)
				require.NoError(err)
				return token
			}()

			row := makeObject(cfg, objName)
			row.PutInt32("int32", 1)
			row.PutString("email", ukToken)

			_, err := row.Build()
			require.ErrorIs(err, ErrInvalidVerificationKind)
			require.ErrorContains(err, "Phone")
		})

		t.Run("error if wrong verified entity in token", func(t *testing.T) {
			weToken := func() string {
				p := payloads.VerifiedValuePayload{
					VerificationKind: appdef.VerificationKind_EMail,
					Entity:           appdef.NewQName("test", "other"),
					Field:            "email",
					Value:            email,
				}
				token, err := tokens.IssueToken(time.Minute, &p)
				require.NoError(err)
				return token
			}()

			row := makeObject(cfg, objName)
			row.PutInt32("int32", 1)
			row.PutString("email", weToken)

			_, err := row.Build()
			require.ErrorIs(err, ErrInvalidName)
		})

		t.Run("error if wrong verified field in token", func(t *testing.T) {
			wfToken := func() string {
				p := payloads.VerifiedValuePayload{
					VerificationKind: appdef.VerificationKind_EMail,
					Entity:           objName,
					Field:            "otherField",
					Value:            email,
				}
				token, err := tokens.IssueToken(time.Minute, &p)
				require.NoError(err)
				return token
			}()

			row := makeObject(cfg, objName)
			row.PutInt32("int32", 1)
			row.PutString("email", wfToken)

			_, err := row.Build()
			require.ErrorIs(err, ErrInvalidName)
		})

		t.Run("error if wrong verified value type in token", func(t *testing.T) {
			wtToken := func() string {
				p := payloads.VerifiedValuePayload{
					VerificationKind: appdef.VerificationKind_EMail,
					Entity:           objName,
					Field:            "email",
					Value:            3.141592653589793238,
				}
				token, err := tokens.IssueToken(time.Minute, &p)
				require.NoError(err)
				return token
			}()

			row := makeObject(cfg, objName)
			row.PutInt32("int32", 1)
			row.PutString("email", wtToken)

			_, err := row.Build()
			require.ErrorIs(err, ErrWrongFieldType)
		})

	})
}

func Test_CharsFieldRestricts(t *testing.T) {
	require := require.New(t)
	test := test()

	objName := appdef.NewQName("test", "obj")

	appDef := appdef.New()
	t.Run("must be ok to build application", func(t *testing.T) {
		appDef.AddObject(objName).
			AddStringField("email", true, appdef.MinLen(6), appdef.MaxLen(100), appdef.Pattern(`^[\w-\.]+@([\w-]+\.)+[\w-]{2,4}$`)).
			AddBytesField("mime", false, appdef.MinLen(4), appdef.MaxLen(4), appdef.Pattern(`^\w+$`))
	})

	cfgs := make(AppConfigsType, 1)
	cfg := cfgs.AddConfig(test.appName, appDef)

	storage, err := simpleStorageProvider().AppStorage(istructs.AppQName_test1_app1)
	require.NoError(err)
	asp := Provide(cfgs, iratesce.TestBucketsFactory, testTokensFactory(), simpleStorageProvider())
	err = cfg.prepare(iratesce.TestBucketsFactory(), storage)
	require.NoError(err)
	_, err = asp.AppStructs(test.appName)
	require.NoError(err)

	t.Run("test field restricts", func(t *testing.T) {

		t.Run("must be ok check good value", func(t *testing.T) {
			row := makeObject(cfg, objName)
			row.PutString("email", `test@test.io`)
			row.PutBytes("mime", []byte(`abcd`))

			_, err := row.Build()
			require.NoError(err)
		})

		t.Run("must be error if min length restricted", func(t *testing.T) {
			row := makeObject(cfg, objName)
			row.PutString("email", `t@t`)
			row.PutBytes("mime", []byte(`abc`))

			_, err := row.Build()
			require.ErrorIs(err, ErrFieldValueRestricted)
			require.ErrorContains(err, "field «email» is too short")
			require.ErrorContains(err, "field «mime» is too short")
		})

		t.Run("must be error if max length restricted", func(t *testing.T) {
			row := makeObject(cfg, objName)
			row.PutString("email", fmt.Sprintf("%s.com", strings.Repeat("test", 100)))
			row.PutBytes("mime", []byte(`abcde`))

			_, err := row.Build()
			require.ErrorIs(err, ErrFieldValueRestricted)
			require.ErrorContains(err, "field «email» is too long")
			require.ErrorContains(err, "field «mime» is too long")
		})

		t.Run("must be error if pattern restricted", func(t *testing.T) {
			row := makeObject(cfg, objName)
			row.PutString("email", "naked@🔫.error")
			row.PutBytes("mime", []byte(`++++`))

			_, err := row.Build()
			require.ErrorIs(err, ErrFieldValueRestricted)
			require.ErrorContains(err, "field «email» does not match pattern")
			require.ErrorContains(err, "field «mime» does not match pattern")
		})
	})
}

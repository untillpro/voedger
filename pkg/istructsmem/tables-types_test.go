/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package istructsmem

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	log "github.com/untillpro/goutils/logger"
	"github.com/voedger/voedger/pkg/istructs"
)

func Test_newRecord(t *testing.T) {

	t.Run("newNullRecord must return empty, nullQName record with specified sys.ID", func(t *testing.T) {
		require := require.New(t)

		rec := NewNullRecord(100500)
		require.Equal(rec.QName(), istructs.NullQName)
		require.Equal(rec.ID(), istructs.RecordID(100500))
		require.Equal(rec.Parent(), istructs.NullRecordID)
		require.Equal(rec.Container(), "")
	})

	t.Run("newRecord must return empty, nullQName record", func(t *testing.T) {
		require := require.New(t)

		rec := newRecord(test.AppCfg)
		require.True(rec.empty())

		t.Run("test as IRecord", func(t *testing.T) {
			var r istructs.IRecord = &rec
			require.Equal(istructs.NullQName, r.QName())
			require.Equal(istructs.NullRecordID, r.ID())
			require.Equal(istructs.NullRecordID, r.Parent())
			require.Equal("", r.Container())
		})

		t.Run("test as ICRecord", func(t *testing.T) {
			var r istructs.ICRecord = &rec
			require.True(r.IsActive())
		})

		t.Run("test as IRowReader", func(t *testing.T) {
			var r istructs.IRowReader = &rec
			require.Equal(istructs.NullQName, r.AsQName(istructs.SystemField_QName))
			require.Equal(istructs.NullRecordID, r.AsRecordID(istructs.SystemField_ID))
			require.Equal(istructs.NullRecordID, r.AsRecordID(istructs.SystemField_ParentID))
			require.Equal("", r.AsString(istructs.SystemField_Container))
			require.True(r.AsBool(istructs.SystemField_IsActive))
		})
	})

	t.Run("newEmptyTestCDoc must return empty, «test.CDoc»", func(t *testing.T) {
		require := require.New(t)

		doc := newEmptyTestCDoc()
		require.True(doc.empty())
		require.Equal(doc.QName(), test.testCDoc)
		require.Equal(doc.ID(), istructs.NullRecordID)
		require.True(doc.IsActive())

		t.Run("newEmptyTestCRec must return empty, «test.Record»", func(t *testing.T) {
			rec := newEmptyTestCRecord()
			require.True(rec.empty())
			require.Equal(rec.QName(), test.testCRec)
			require.Equal(rec.ID(), istructs.NullRecordID)
			require.Equal(rec.Parent(), istructs.NullRecordID)
			require.Equal(rec.Container(), "")
			require.True(rec.IsActive())
		})
	})

	t.Run("newTestCDoc must return non empty, full filled and valid «test.CDoc»", func(t *testing.T) {
		require := require.New(t)

		doc := newTestCDoc(100500)
		require.False(doc.empty())
		require.Equal(test.testCDoc, doc.QName())
		require.Equal(istructs.RecordID(100500), doc.ID())
		require.Equal(istructs.RecordID(100500), doc.AsRecordID(istructs.SystemField_ID))
		require.Equal(istructs.NullRecordID, doc.Parent())
		require.Equal("", doc.Container())
		require.True(doc.IsActive())

		testTestCDoc(t, doc, 100500)

		t.Run("system field counters for test CDoc", func(t *testing.T) {
			sysCnt := 0
			doc.FieldNames(func(fieldName string) {
				require.True(doc.hasValue(fieldName))
				if sysField(fieldName) {
					sysCnt++
				}
			})
			require.Equal(3, sysCnt) // sys.QName, sys.ID and sys.IsActive
		})

		t.Run("inactivating test CDoc", func(t *testing.T) {
			doc.PutBool(istructs.SystemField_IsActive, false)

			require.False(doc.IsActive())
			require.False(doc.AsBool(istructs.SystemField_IsActive))
		})

		t.Run("field counters for test CDoc", func(t *testing.T) {
			cnt := 0
			sysCnt := 0
			doc.FieldNames(func(fieldName string) {
				require.True(doc.hasValue(fieldName))
				if sysField(fieldName) {
					sysCnt++
				}
				cnt++
			})

			require.Equal(3, sysCnt) // sys.QName, sys.ID and sys.IsActive
			require.Equal(sysCnt+9, cnt)
			require.Equal(len(doc.schema.fieldsOrder), cnt) // lowlevel check
		})

		t.Run("newTestCRec must return non empty, full filled and valid «test.Record»", func(t *testing.T) {
			rec := newTestCRecord(100501)
			require.False(rec.empty())
			require.Equal(test.testCRec, rec.QName())
			require.Equal(istructs.RecordID(100501), rec.ID())
			require.Equal(istructs.RecordID(100501), rec.AsRecordID(istructs.SystemField_ID))
			require.Equal(istructs.NullRecordID, rec.Parent())
			require.Equal("", rec.Container())
			require.True(rec.IsActive())

			testTestCRec(t, rec, 100501)

			rec.PutRecordID(istructs.SystemField_ParentID, doc.ID())
			require.Equal(doc.ID(), rec.Parent())
			require.Equal(doc.ID(), rec.AsRecordID(istructs.SystemField_ParentID))

			rec.PutString(istructs.SystemField_Container, "record")
			require.Equal("record", rec.Container())
			require.Equal("record", rec.AsString(istructs.SystemField_Container))

			t.Run("system field counters for test CRecord", func(t *testing.T) {
				sysCnt := 0
				rec.FieldNames(func(fieldName string) {
					require.True(rec.hasValue(fieldName))
					if sysField(fieldName) {
						sysCnt++
					}
				})
				require.Equal(5, sysCnt) // sys.QName, sys.ID sys.ParentID, sys.Container and sys.IsActive
			})

			t.Run("inactivating test CRecord", func(t *testing.T) {
				rec.PutBool(istructs.SystemField_IsActive, false)

				require.False(rec.IsActive())
				require.False(rec.AsBool(istructs.SystemField_IsActive))
			})

			t.Run("field counters for test CRecord", func(t *testing.T) {
				cnt := 0
				sysCnt := 0
				rec.FieldNames(func(fieldName string) {
					require.True(rec.hasValue(fieldName))
					if sysField(fieldName) {
						sysCnt++
					}
					cnt++
				})

				require.Equal(5, sysCnt) // sys.QName, sys.ID sys.ParentID, sys.Container and sys.IsActive
				require.Equal(sysCnt+9, cnt)
				require.Equal(len(rec.schema.fieldsOrder), cnt) // lowlevel check
			})
		})
	})
}

func Test_LoadStoreRecord_Bytes(t *testing.T) {
	require := require.New(t)

	t.Run("test rec1 must be success storeToBytes() and test rec2 must success loadFromBytes(). rec1 and rec2 must be equals", func(t *testing.T) {
		rec1 := newTestCDoc(100500)

		b, err := rec1.storeToBytes()
		require.NoError(err, err)

		rec2 := newRecord(test.AppCfg)
		err = rec2.loadFromBytes(b)
		require.NoError(err, err)
		testTestCDoc(t, &rec2, 100500)

		testRecsIsEqual(t, rec1, &rec2)
	})

	t.Run("same as previous test, but for deactivated CDoc", func(t *testing.T) {
		rec1 := newTestCDoc(100501)
		rec1.PutBool(istructs.SystemField_IsActive, false)

		b, err := rec1.storeToBytes()
		require.NoError(err, err)

		rec2 := newRecord(test.AppCfg)
		err = rec2.loadFromBytes(b)
		require.NoError(err, err)
		testTestCDoc(t, &rec2, 100501)
		require.False(rec2.AsBool(istructs.SystemField_IsActive))

		testRecsIsEqual(t, rec1, &rec2)
	})

	t.Run("must be ok to read data stored with previous codec versions", func(t *testing.T) {
		store_codec_RawDynoBuffer := func(row *recordType) (out []byte) {
			buf := new(bytes.Buffer)
			_ = binary.Write(buf, binary.BigEndian, codec_RawDynoBuffer)
			id, err := row.qNameID()
			require.NoError(err)
			_ = binary.Write(buf, binary.BigEndian, int16(id))
			if row.QName() == istructs.NullQName {
				return buf.Bytes()
			}
			if schemaNeedSysField_ID(row.schema.kind) {
				require.NoError(binary.Write(buf, binary.BigEndian, uint64(row.ID())))
			}
			if schemaNeedSysField_ParentID(row.schema.kind) {
				require.NoError(binary.Write(buf, binary.BigEndian, uint64(row.parentID)))
			}
			if schemaNeedSysField_Container(row.schema.kind) {
				id, err := row.containerID()
				require.NoError(err)
				require.NoError(binary.Write(buf, binary.BigEndian, int16(id)))
			}
			if schemaNeedSysField_IsActive(row.schema.kind) {
				require.NoError(binary.Write(buf, binary.BigEndian, row.isActive))
			}
			b, err := row.dyB.ToBytes()
			require.NoError(err)
			len := uint32(len(b))
			require.NoError(binary.Write(buf, binary.BigEndian, &len))
			_, err = buf.Write(b)
			require.NoError(err)
			return buf.Bytes()
		}

		t.Run("test CDocs", func(t *testing.T) {
			doc1 := newTestCDoc(100502)

			bytes := store_codec_RawDynoBuffer(doc1)

			doc2 := newRecord(test.AppCfg)
			err := doc2.loadFromBytes(bytes)
			require.NoError(err, err)
			testTestCDoc(t, &doc2, 100502)

			testRecsIsEqual(t, doc1, &doc2)
		})

		t.Run("test CRecords", func(t *testing.T) {
			rec1 := newTestCRecord(100503)
			rec1.PutRecordID(istructs.SystemField_ParentID, 100502)
			rec1.PutString(istructs.SystemField_Container, test.goodIdent)

			bytes := store_codec_RawDynoBuffer(rec1)

			rec2 := newRecord(test.AppCfg)
			err := rec2.loadFromBytes(bytes)
			require.NoError(err, err)

			testRecsIsEqual(t, rec1, &rec2)
		})
	})

	t.Run("null records (with NullQName) must be success storeToBytes() and success loadFromBytes()", func(t *testing.T) {
		rec1 := newRecord(test.AppCfg)
		b, err := rec1.storeToBytes()
		require.NoError(err, err)

		rec2 := newEmptyTestCDoc()
		err = rec2.loadFromBytes(b)
		require.NoError(err, err)

		require.Equal(istructs.NullQName, rec2.QName())
		require.Equal(istructs.NullRecordID, rec2.ID())
	})

	t.Run("empty records (with «test.record» QName) must be success storeToBytes() and success loadFromBytes()", func(t *testing.T) {
		rec1 := newEmptyTestCDoc()
		b, err := rec1.storeToBytes()
		require.NoError(err, err)

		rec2 := newRecord(test.AppCfg)
		err = rec2.loadFromBytes(b)
		require.NoError(err, err)

		require.Equal(test.testCDoc, rec2.QName())
		require.Equal(istructs.NullRecordID, rec2.ID())
	})

	t.Run("test rec1 must be success storeToBytes(); rec2 loadFromBytes() from truncated bytes must fails", func(t *testing.T) {
		rec1 := newTestCDoc(100500)

		b, err := rec1.storeToBytes()
		require.NoError(err, err)

		len := len(b)
		for i := 0; i < len; i++ {
			corrupted := b[0:i]
			rec2 := newRecord(test.AppCfg)
			err = rec2.loadFromBytes(corrupted)
			require.Error(err, fmt.Sprintf("unexpected success load record from bytes truncated at length «%d»", i))
		}
	})

	t.Run("dynobuffer corrupt test: loadFromBytes() from corrupted bytes may:\n"+
		"— fail (Panic or Error) or\n"+
		"— success read wrong data (BadData) or\n"+
		"— success read correct data (Lucky)",
		func(t *testing.T) {
			rec1 := newTestCDoc(100500)

			b, err := rec1.storeToBytes()
			require.NoError(err, err)

			len := len(b)
			stat := make(map[string]int)
			for i := 0; i < len; i++ {
				b[i] ^= 255
				rec2 := newRecord(test.AppCfg)
				func() {
					defer func() {
						if err := recover(); err != nil {
							log.Verbose("%d: panic at read record: %v", i, err)
							stat["Panics"]++
						}
					}()
					if err = rec2.loadFromBytes(b); err != nil {
						log.Verbose("%d: error at load: %v\n", i, err)
						stat["Errors"]++
						return
					}
					if ok, diff := recsIsEqual(rec1, &rec2); ok {
						log.Verbose("%d: success load, data is ok\n", i)
						stat["Lucky"]++
					} else {
						log.Verbose("%d: success load, data is corrupted: %v\n", i, diff)
						stat["BadData"]++
					}
				}()
				b[i] ^= 255
			}
			log.Verbose("len: %d, stat: %v\n", len, stat)
		})

	t.Run("test field renaming availability", func(t *testing.T) {
		rec1 := newTestCDoc(100500)

		b, err := rec1.storeToBytes()
		require.NoError(err, err)

		newConfig := newAppConfig(test.AppCfg.Name)
		recSchema := newConfig.Schemas.Add(test.testCDoc, istructs.SchemaKind_CDoc)
		recSchema.
			AddField("int32_1", istructs.DataKind_int32, false).
			AddField("int64_1", istructs.DataKind_int64, false).
			AddField("float32_1", istructs.DataKind_float32, false).
			AddField("float64_1", istructs.DataKind_float64, false).
			AddField("bytes_1", istructs.DataKind_bytes, false).
			AddField("string_1", istructs.DataKind_string, false).
			AddField("QName_1", istructs.DataKind_QName, false).
			AddField("bool_1", istructs.DataKind_bool, false).
			AddField("RecordID_1", istructs.DataKind_RecordID, false)

		newConfig.qNames.collectAppQName(test.testCDoc)
		newConfig.qNames.collectAppQName(test.tablePhotos) // for reading QName_1 field value

		err = newConfig.prepare(nil, test.AppCfg.storage)
		require.NoError(err)

		rec2 := newRecord(newConfig)
		err = rec2.loadFromBytes(b)
		require.NoError(err, err)

		require.Equal(rec1.QName(), rec2.QName())
		rec1.dyB.IterateFields(nil, func(name string, val1 interface{}) bool {
			newName := name + "_1"
			require.True(rec2.hasValue(newName), newName)
			val2 := rec2.dyB.Get(newName)
			require.Equal(val1, val2)
			return true
		})
		rec2.dyB.IterateFields(nil, func(name string, val2 interface{}) bool {
			oldName := name[:len(name)-2]
			require.True(rec1.hasValue(oldName), oldName)
			return true
		})
	})

}

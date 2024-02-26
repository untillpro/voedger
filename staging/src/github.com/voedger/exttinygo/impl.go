/*
* Copyright (c) 2023-present unTill Pro, Ltd.
*  @author Michael Saigachenko
 */

package exttinygo

import (
	"runtime"
	"unsafe"
)

func Assert(condition bool, msg string) {
	if !condition {
		Panic("assertion failed: " + msg)
	}
}

func Panic(msg string) {
	hostPanic(uint32(uintptr(unsafe.Pointer(unsafe.StringData(msg)))), uint32(len(msg)))
}

const maxUint = ^uint64(0)

func queryValueImpl(key TKeyBuilder) (TValue, bool) {
	id := hostQueryValue(uint64(key))
	if id != maxUint {
		return TValue(id), true
	} else {
		return TValue(0), false
	}
}

func mustGetValueImpl(key TKeyBuilder) TValue {
	return TValue(hostGetValue(uint64(key)))
}

func updateValueImpl(key TKeyBuilder, existingValue TValue) TIntent {
	return TIntent(hostUpdateValue(uint64(key), uint64(existingValue)))
}

func newValueImpl(key TKeyBuilder) TIntent {
	return TIntent(hostNewValue(uint64(key)))
}

func readValuesImpl(key TKeyBuilder, callback func(key TKey, value TValue)) {
	currentReadCallback = callback
	hostReadValues(uint64(key))
}

var currentReadCallback func(key TKey, value TValue)

//lint:ignore U1000 this is an exported func
//export WasmOnReadValue
func onReadValue(key, value uint64) {
	currentReadCallback(TKey(key), TValue(value))
}

//export hostReadValues
func hostReadValues(keyId uint64)

//export hostGetValue
func hostGetValue(keyId uint64) (result uint64)

/*
	returns 0 when not exists
*/
//export hostQueryValue
func hostQueryValue(keyId uint64) (result uint64)

//export hostNewValue
func hostNewValue(keyId uint64) uint64

//export hostUpdateValue
func hostUpdateValue(keyId uint64, existingValueId uint64) uint64

//lint:ignore U1000 this is an exported func
//export WasmAbiVersion_0_0_1
func proxyABIVersion() {
}

var ms runtime.MemStats

//lint:ignore U1000 this is an exported func
//export WasmGetHeapInuse
func getHeapInuse() uint64 {
	runtime.ReadMemStats(&ms)
	return ms.HeapInuse
}

//lint:ignore U1000 this is an exported func
//export WasmGetMallocs
func getMallocs() uint64 {
	runtime.ReadMemStats(&ms)
	return ms.Mallocs
}

//lint:ignore U1000 this is an exported func
//export WasmGetFrees
func getFrees() uint64 {
	runtime.ReadMemStats(&ms)
	return ms.Frees
}

//lint:ignore U1000 this is an exported func
//export WasmGetHeapSys
func getHeapSys() uint64 {
	runtime.ReadMemStats(&ms)
	return ms.HeapSys
}

//lint:ignore U1000 this is an exported func
//export WasmGC
func gc() {
	runtime.GC()
}

//export hostPanic
func hostPanic(msgPtr, msgSize uint32)

//export hostRowWriterPutString
func hostRowWriterPutString(id uint64, typ uint32, namePtr, nameSize, valuePtr, valueSize uint32)

//export hostRowWriterPutBytes
func hostRowWriterPutBytes(id uint64, typ uint32, namePtr, nameSize, valuePtr, valueSize uint32)

//export hostRowWriterPutQName
func hostRowWriterPutQName(id uint64, typ uint32, namePtr, nameSize, pkgPtr, pkgSize, entityPtr, entitySize uint32)

//export hostRowWriterPutIntBool
func hostRowWriterPutBool(id uint64, typ uint32, namePtr, nameSize, value uint32)

//export hostRowWriterPutInt32
func hostRowWriterPutInt32(id uint64, typ uint32, namePtr, nameSize, value uint32)

//export hostRowWriterPutInt64
func hostRowWriterPutInt64(id uint64, typ uint32, namePtr, nameSize uint32, value uint64)

//export hostRowWriterPutFloat32
func hostRowWriterPutFloat32(id uint64, typ uint32, namePtr, nameSize uint32, value float32)

//export hostRowWriterPutFloat64
func hostRowWriterPutFloat64(id uint64, typ uint32, namePtr, nameSize uint32, value float64)

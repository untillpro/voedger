/*
  - Copyright (c) 2023-present unTill Software Development Group B.V.
    @author Michael Saigachenko
*/

package iextenginewazero

const bitsInFourBytes = 32

const (
	maxMemoryPages = 0xffff
	maxStdErrSize  = 1024

	WasmPreallocatedBufferIncrease          = 1000
	WasmDefaultPreallocatedBufferSize       = 64000
	metric_voedger_wasm_invocations_total   = "voedger_wasm_invocations_total"
	metric_voedger_wasm_invocations_seconds = "voedger_wasm_invocations_seconds"
	metric_voedger_wasm_errors_total        = "voedger_wasm_errors_total"
	metric_voedger_wasm_recovers_total      = "voedger_wasm_recovers_total"
)

var WasmPreallocatedBufferSize uint32 = WasmDefaultPreallocatedBufferSize

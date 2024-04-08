/*
  - Copyright (c) 2023-present unTill Software Development Group B.V.
    @author Michael Saigachenko
*/

package iextenginewazero

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/iextengine"
	"github.com/voedger/voedger/pkg/state/isafeapi"
	"github.com/voedger/voedger/pkg/state/safestate"
)

type wazeroExtPkg struct {
	module api.Module
	exts   map[string]api.Function

	funcMalloc api.Function
	funcFree   api.Function

	funcVer          api.Function
	funcGetHeapInuse api.Function
	funcGetHeapSys   api.Function
	funcGetMallocs   api.Function
	funcGetFrees     api.Function
	funcGc           api.Function
	funcOnReadValue  api.Function

	allocatedBufs []*allocatedBuf
	recoverMem    []byte
}

type wazeroExtEngine struct {
	compile bool
	config  *iextengine.ExtEngineConfig
	modules map[string]*wazeroExtPkg
	host    api.Module
	rtm     wazero.Runtime

	wasiCloser api.Closer

	// Invoke-related!
	safeApi isafeapi.ISafeAPI

	ctx context.Context
	pkg *wazeroExtPkg
}

type allocatedBuf struct {
	addr uint32
	offs uint32
	cap  uint32
}

type extensionEngineFactory struct {
	compile bool
}

func (f extensionEngineFactory) New(ctx context.Context, packages []iextengine.ExtensionPackage, config *iextengine.ExtEngineConfig, numEngines int) (engines []iextengine.IExtensionEngine, err error) {
	for i := 0; i < numEngines; i++ {
		engine := &wazeroExtEngine{
			modules: make(map[string]*wazeroExtPkg),
			config:  config,
			compile: f.compile,
		}
		err = engine.init(ctx)
		if err != nil {
			return engines, err
		}
		engines = append(engines, engine)
	}

	for _, pkg := range packages {
		if pkg.ModuleUrl.Scheme == "file" && (pkg.ModuleUrl.Host == "" || strings.EqualFold("localhost", pkg.ModuleUrl.Scheme)) {
			path := pkg.ModuleUrl.Path
			if runtime.GOOS == "windows" {
				path = strings.TrimPrefix(path, "/")
			}

			wasmdata, err := os.ReadFile(path)

			if err != nil {
				return nil, err
			}

			for _, eng := range engines {
				err = eng.(*wazeroExtEngine).initModule(ctx, pkg.QualifiedName, wasmdata, pkg.ExtensionNames)
				if err != nil {
					return nil, err
				}
			}
		} else {
			return nil, fmt.Errorf("unsupported URL: " + pkg.ModuleUrl.String())
		}
	}
	return engines, nil
}

func (f *wazeroExtEngine) SetLimits(limits iextengine.ExtensionLimits) {
	// f.cep.Duration = limits.ExecutionInterval
}

func (f *wazeroExtPkg) importFuncs(funcs map[string]*api.Function) error {

	for k, v := range funcs {
		*v = f.module.ExportedFunction(k)
		if *v == nil {
			return fmt.Errorf("missing exported function: %s", k)
		}
	}
	return nil
}

func (f *wazeroExtEngine) init(ctx context.Context) error {
	var err error
	var memPages = f.config.MemoryLimitPages
	if memPages == 0 {
		memPages = iextengine.DefaultMemoryLimitPages
	}
	if memPages > maxMemoryPages {
		return errors.New("maximum allowed MemoryLimitPages is 0xffff")
	}
	// Total amount of memory must be at least 170% of WasmPreallocatedBufferSize
	const memoryLimitCoef = 1.7
	memoryLimit := memPages * iextengine.MemoryPageSize
	limit := math.Trunc(float64(WasmPreallocatedBufferSize) * float64(memoryLimitCoef))
	if uint32(memoryLimit) <= uint32(limit) {
		return fmt.Errorf("the minimum limit of memory is: %.1f bytes, requested limit is: %.1f", limit, float32(memoryLimit))
	}

	var rtConf wazero.RuntimeConfig

	if f.compile {
		rtConf = wazero.NewRuntimeConfigCompiler()
	} else {
		rtConf = wazero.NewRuntimeConfigInterpreter()
	}
	rtConf = rtConf.
		WithCoreFeatures(api.CoreFeatureBulkMemoryOperations).
		WithCloseOnContextDone(true).
		WithMemoryCapacityFromMax(true).
		WithMemoryLimitPages(uint32(memPages))

	f.rtm = wazero.NewRuntimeWithConfig(ctx, rtConf)
	f.wasiCloser, err = wasi_snapshot_preview1.Instantiate(ctx, f.rtm)

	if err != nil {
		return err
	}

	f.host, err = f.rtm.NewHostModuleBuilder("env").
		NewFunctionBuilder().WithFunc(f.hostGetKey).Export("hostGetKey").
		NewFunctionBuilder().WithFunc(f.hostMustExist).Export("hostGetValue").
		NewFunctionBuilder().WithFunc(f.hostCanExist).Export("hostQueryValue").
		NewFunctionBuilder().WithFunc(f.hostReadValues).Export("hostReadValues").
		NewFunctionBuilder().WithFunc(f.hostPanic).Export("hostPanic").
		// IKey
		NewFunctionBuilder().WithFunc(f.hostKeyAsString).Export("hostKeyAsString").
		NewFunctionBuilder().WithFunc(f.hostKeyAsBytes).Export("hostKeyAsBytes").
		NewFunctionBuilder().WithFunc(f.hostKeyAsInt32).Export("hostKeyAsInt32").
		NewFunctionBuilder().WithFunc(f.hostKeyAsInt64).Export("hostKeyAsInt64").
		NewFunctionBuilder().WithFunc(f.hostKeyAsFloat32).Export("hostKeyAsFloat32").
		NewFunctionBuilder().WithFunc(f.hostKeyAsFloat64).Export("hostKeyAsFloat64").
		NewFunctionBuilder().WithFunc(f.hostKeyAsBool).Export("hostKeyAsBool").
		NewFunctionBuilder().WithFunc(f.hostKeyAsQNamePkg).Export("hostKeyAsQNamePkg").
		NewFunctionBuilder().WithFunc(f.hostKeyAsQNameEntity).Export("hostKeyAsQNameEntity").
		// IValue
		NewFunctionBuilder().WithFunc(f.hostValueLength).Export("hostValueLength").
		NewFunctionBuilder().WithFunc(f.hostValueAsValue).Export("hostValueAsValue").
		NewFunctionBuilder().WithFunc(f.hostValueAsString).Export("hostValueAsString").
		NewFunctionBuilder().WithFunc(f.hostValueAsBytes).Export("hostValueAsBytes").
		NewFunctionBuilder().WithFunc(f.hostValueAsInt32).Export("hostValueAsInt32").
		NewFunctionBuilder().WithFunc(f.hostValueAsInt64).Export("hostValueAsInt64").
		NewFunctionBuilder().WithFunc(f.hostValueAsFloat32).Export("hostValueAsFloat32").
		NewFunctionBuilder().WithFunc(f.hostValueAsFloat64).Export("hostValueAsFloat64").
		NewFunctionBuilder().WithFunc(f.hostValueAsQNamePkg).Export("hostValueAsQNamePkg").
		NewFunctionBuilder().WithFunc(f.hostValueAsQNameEntity).Export("hostValueAsQNameEntity").
		NewFunctionBuilder().WithFunc(f.hostValueAsBool).Export("hostValueAsBool").
		NewFunctionBuilder().WithFunc(f.hostValueGetAsBytes).Export("hostValueGetAsBytes").
		NewFunctionBuilder().WithFunc(f.hostValueGetAsString).Export("hostValueGetAsString").
		NewFunctionBuilder().WithFunc(f.hostValueGetAsInt32).Export("hostValueGetAsInt32").
		NewFunctionBuilder().WithFunc(f.hostValueGetAsInt64).Export("hostValueGetAsInt64").
		NewFunctionBuilder().WithFunc(f.hostValueGetAsFloat32).Export("hostValueGetAsFloat32").
		NewFunctionBuilder().WithFunc(f.hostValueGetAsFloat64).Export("hostValueGetAsFloat64").
		NewFunctionBuilder().WithFunc(f.hostValueGetAsValue).Export("hostValueGetAsValue").
		NewFunctionBuilder().WithFunc(f.hostValueGetAsQNamePkg).Export("hostValueGetAsQNamePkg").
		NewFunctionBuilder().WithFunc(f.hostValueGetAsQNameEntity).Export("hostValueGetAsQNameEntity").
		NewFunctionBuilder().WithFunc(f.hostValueGetAsBool).Export("hostValueGetAsBool").
		// Intents
		NewFunctionBuilder().WithFunc(f.hostNewValue).Export("hostNewValue").
		NewFunctionBuilder().WithFunc(f.hostUpdateValue).Export("hostUpdateValue").
		// RowWriters
		NewFunctionBuilder().WithFunc(f.hostRowWriterPutString).Export("hostRowWriterPutString").
		NewFunctionBuilder().WithFunc(f.hostRowWriterPutBytes).Export("hostRowWriterPutBytes").
		NewFunctionBuilder().WithFunc(f.hostRowWriterPutInt32).Export("hostRowWriterPutInt32").
		NewFunctionBuilder().WithFunc(f.hostRowWriterPutInt64).Export("hostRowWriterPutInt64").
		NewFunctionBuilder().WithFunc(f.hostRowWriterPutFloat32).Export("hostRowWriterPutFloat32").
		NewFunctionBuilder().WithFunc(f.hostRowWriterPutFloat64).Export("hostRowWriterPutFloat64").
		NewFunctionBuilder().WithFunc(f.hostRowWriterPutBool).Export("hostRowWriterPutBool").
		NewFunctionBuilder().WithFunc(f.hostRowWriterPutQName).Export("hostRowWriterPutQName").
		//ExportFunction("printstr", f.printStr).

		Instantiate(ctx)
	if err != nil {
		return err
	}

	return nil

}

func (f *wazeroExtEngine) initModule(ctx context.Context, pkgName string, wasmdata []byte, extNames []string) (err error) {
	ePkg := &wazeroExtPkg{}

	moduleCfg := wazero.NewModuleConfig().WithName("wasm").WithStdout(io.Discard).WithStderr(io.Discard)
	if f.compile {
		compiledWasm, err := f.rtm.CompileModule(ctx, wasmdata)
		if err != nil {
			return err
		}

		ePkg.module, err = f.rtm.InstantiateModule(ctx, compiledWasm, moduleCfg)
		if err != nil {
			return err
		}
	} else {
		ePkg.module, err = f.rtm.InstantiateWithConfig(ctx, wasmdata, moduleCfg)
		if err != nil {
			return err
		}
	}

	err = ePkg.importFuncs(map[string]*api.Function{
		"malloc":               &ePkg.funcMalloc,
		"free":                 &ePkg.funcFree,
		"WasmAbiVersion_0_0_1": &ePkg.funcVer,
		"WasmGetHeapInuse":     &ePkg.funcGetHeapInuse,
		"WasmGetHeapSys":       &ePkg.funcGetHeapSys,
		"WasmGetMallocs":       &ePkg.funcGetMallocs,
		"WasmGetFrees":         &ePkg.funcGetFrees,
		"WasmGC":               &ePkg.funcGc,
		"WasmOnReadValue":      &ePkg.funcOnReadValue,
	})
	if err != nil {
		return err
	}

	// Check WASM SDK version
	_, err = ePkg.funcVer.Call(ctx)
	if err != nil {
		return errors.New("unsupported WASM version")
	}
	res, err := ePkg.funcMalloc.Call(ctx, uint64(WasmPreallocatedBufferSize))
	if err != nil {
		return err
	}
	ePkg.allocatedBufs = append(ePkg.allocatedBufs, &allocatedBuf{
		addr: uint32(res[0]),
		offs: 0,
		cap:  WasmPreallocatedBufferSize,
	})

	backup, read := ePkg.module.Memory().Read(0, ePkg.module.Memory().Size())
	if !read {
		return fmt.Errorf("unable to backup memory")
	}

	ePkg.recoverMem = make([]byte, ePkg.module.Memory().Size())
	copy(ePkg.recoverMem[0:], backup[0:])

	ePkg.exts = make(map[string]api.Function)

	for _, name := range extNames {
		if !strings.HasPrefix(name, "Wasm") && name != "alloc" && name != "free" &&
			name != "calloc" && name != "realloc" && name != "malloc" && name != "_start" && name != "memory" {
			expFunc := ePkg.module.ExportedFunction(name)
			if expFunc != nil {
				ePkg.exts[name] = expFunc
			} else {
				return missingExportedFunction(name)
			}
		} else {
			return incorrectExtensionName(name)
		}
	}

	f.modules[pkgName] = ePkg
	return nil
}

func (f *wazeroExtEngine) Close(ctx context.Context) {
	for _, m := range f.modules {
		if m.module != nil {
			m.module.Close(ctx)
		}
	}
	if f.host != nil {
		f.host.Close(ctx)
	}
	if f.wasiCloser != nil {
		f.wasiCloser.Close(ctx)
	}
}

func (f *wazeroExtEngine) recover() {
	if !f.pkg.module.Memory().Write(0, f.pkg.recoverMem) {
		panic("unable to restore memory")
	}
}

func (f *wazeroExtEngine) Invoke(ctx context.Context, extension appdef.FullQName, io iextengine.IExtensionIO) (err error) {

	var ok bool
	f.pkg, ok = f.modules[extension.PkgPath()]
	if !ok {
		return errUndefinedPackage(extension.PkgPath())
	}

	funct := f.pkg.exts[extension.Entity()]
	if funct == nil {
		return invalidExtensionName(extension.Entity())
	}

	f.safeApi = safestate.Provide(io, f.safeApi)
	f.ctx = ctx

	for i := range f.pkg.allocatedBufs {
		f.pkg.allocatedBufs[i].offs = 0 // reuse pre-allocated memory
	}

	_, err = funct.Call(ctx)

	if err != nil {
		f.recover()
	}

	return err
}

func (f *wazeroExtEngine) decodeStr(ptr, size uint32) string {
	if bytes, ok := f.pkg.module.Memory().Read(ptr, size); ok {
		return string(bytes)
	}
	panic(ErrUnableToReadMemory)
}

func (f *wazeroExtEngine) hostGetKey(storagePtr, storageSize, entityPtr, entitySize uint32) (res uint64) {
	storageFull := f.decodeStr(storagePtr, storageSize)
	entitystr := f.decodeStr(entityPtr, entitySize)
	return uint64(f.safeApi.KeyBuilder(storageFull, entitystr))
}

func (f *wazeroExtEngine) hostPanic(namePtr, nameSize uint32) {
	panic(f.decodeStr(namePtr, nameSize))
}

func (f *wazeroExtEngine) hostReadValues(keyId uint64) {
	f.safeApi.ReadValues(isafeapi.TKeyBuilder(keyId), func(key isafeapi.TKey, value isafeapi.TValue) {
		_, err := f.pkg.funcOnReadValue.Call(f.ctx, uint64(key), uint64(value))
		if err != nil {
			panic(err.Error())
		}
	})
}

func (f *wazeroExtEngine) hostMustExist(keyId uint64) (result uint64) {
	return uint64(f.safeApi.MustGetValue(isafeapi.TKeyBuilder(keyId)))
}

const maxUint64 = ^uint64(0)

func (f *wazeroExtEngine) hostCanExist(keyId uint64) (result uint64) {
	v, ok := f.safeApi.QueryValue(isafeapi.TKeyBuilder(keyId))
	if !ok {
		return maxUint64
	}
	return uint64(v)
}

func (f *wazeroExtEngine) allocAndSend(buf []byte) (result uint64) {
	addrPkg, e := f.allocBuf(uint32(len(buf)))
	if e != nil {
		panic(e)
	}
	if !f.pkg.module.Memory().Write(addrPkg, buf) {
		panic(errMemoryOutOfRange)
	}
	return (uint64(addrPkg) << uint64(bitsInFourBytes)) | uint64(len(buf))
}

func (f *wazeroExtEngine) hostKeyAsString(id uint64, namePtr uint32, nameSize uint32) (result uint64) {
	v := f.safeApi.KeyAsString(isafeapi.TKey(id), f.decodeStr(namePtr, nameSize))
	return f.allocAndSend([]byte(v))
}

func (f *wazeroExtEngine) hostKeyAsBytes(id uint64, namePtr uint32, nameSize uint32) (result uint64) {
	v := f.safeApi.KeyAsBytes(isafeapi.TKey(id), f.decodeStr(namePtr, nameSize))
	return f.allocAndSend(v)
}

func (f *wazeroExtEngine) hostKeyAsInt32(id uint64, namePtr uint32, nameSize uint32) (result uint32) {
	return uint32(f.safeApi.KeyAsInt32(isafeapi.TKey(id), f.decodeStr(namePtr, nameSize)))
}

func (f *wazeroExtEngine) hostKeyAsInt64(id uint64, namePtr uint32, nameSize uint32) (result uint64) {
	return uint64(f.safeApi.KeyAsInt64(isafeapi.TKey(id), f.decodeStr(namePtr, nameSize)))
}

func (f *wazeroExtEngine) hostKeyAsBool(id uint64, namePtr uint32, nameSize uint32) (result uint64) {
	b := f.safeApi.KeyAsBool(isafeapi.TKey(id), f.decodeStr(namePtr, nameSize))
	if b {
		return uint64(1)
	}
	return uint64(0)
}

func (f *wazeroExtEngine) hostKeyAsQNamePkg(id uint64, namePtr uint32, nameSize uint32) (result uint64) {
	qname := f.safeApi.KeyAsQName(isafeapi.TKey(id), f.decodeStr(namePtr, nameSize))
	return f.allocAndSend([]byte(qname.FullPkgName))
}

func (f *wazeroExtEngine) hostKeyAsQNameEntity(id uint64, namePtr uint32, nameSize uint32) (result uint64) {
	qname := f.safeApi.KeyAsQName(isafeapi.TKey(id), f.decodeStr(namePtr, nameSize))
	return f.allocAndSend([]byte(qname.Entity))
}

func (f *wazeroExtEngine) hostKeyAsFloat32(key uint64, namePtr uint32, nameSize uint32) (result float32) {
	return f.safeApi.KeyAsFloat32(isafeapi.TKey(key), f.decodeStr(namePtr, nameSize))
}

func (f *wazeroExtEngine) hostKeyAsFloat64(key uint64, namePtr uint32, nameSize uint32) (result float64) {
	return f.safeApi.KeyAsFloat64(isafeapi.TKey(key), f.decodeStr(namePtr, nameSize))
}

func (f *wazeroExtEngine) hostValueGetAsString(value uint64, index uint32) (result uint64) {
	v := f.safeApi.ValueGetAsString(isafeapi.TValue(value), int(index))
	return f.allocAndSend([]byte(v))
}

func (f *wazeroExtEngine) hostValueGetAsQNameEntity(value uint64, index uint32) (result uint64) {
	qname := f.safeApi.ValueGetAsQName(isafeapi.TValue(value), int(index))
	return f.allocAndSend([]byte(qname.Entity))
}

func (f *wazeroExtEngine) hostValueGetAsQNamePkg(value uint64, index uint32) (result uint64) {
	qname := f.safeApi.ValueGetAsQName(isafeapi.TValue(value), int(index))
	return f.allocAndSend([]byte(qname.FullPkgName))
}

func (f *wazeroExtEngine) hostValueGetAsBytes(value uint64, index uint32) (result uint64) {
	return f.allocAndSend(f.safeApi.ValueGetAsBytes(isafeapi.TValue(value), int(index)))
}

func (f *wazeroExtEngine) hostValueGetAsBool(value uint64, index uint32) (result uint64) {
	b := f.safeApi.ValueGetAsBool(isafeapi.TValue(value), int(index))
	if b {
		return 1
	}
	return 0
}

func (f *wazeroExtEngine) hostValueGetAsInt32(value uint64, index uint32) (result int32) {
	return f.safeApi.ValueGetAsInt32(isafeapi.TValue(value), int(index))
}

func (f *wazeroExtEngine) hostValueGetAsInt64(value uint64, index uint32) (result uint64) {
	return uint64(f.safeApi.ValueGetAsInt64(isafeapi.TValue(value), int(index)))
}

func (f *wazeroExtEngine) hostValueGetAsFloat32(id uint64, index uint32) float32 {
	return f.safeApi.ValueGetAsFloat32(isafeapi.TValue(id), int(index))
}

func (f *wazeroExtEngine) hostValueGetAsFloat64(id uint64, index uint32) float64 {
	return f.safeApi.ValueGetAsFloat64(isafeapi.TValue(id), int(index))
}

func (f *wazeroExtEngine) hostValueGetAsValue(val uint64, index uint32) (result uint64) {
	return uint64(f.safeApi.ValueGetAsValue(isafeapi.TValue(val), int(index)))
}

func (f *wazeroExtEngine) hostValueAsString(id uint64, namePtr uint32, nameSize uint32) (result uint64) {
	s := f.safeApi.ValueAsString(isafeapi.TValue(id), f.decodeStr(namePtr, nameSize))
	return f.allocAndSend([]byte(s))
}

func (f *wazeroExtEngine) hostValueAsBytes(id uint64, namePtr uint32, nameSize uint32) (result uint64) {
	b := f.safeApi.ValueAsBytes(isafeapi.TValue(id), f.decodeStr(namePtr, nameSize))
	return f.allocAndSend(b)
}

func (f *wazeroExtEngine) hostValueAsInt32(id uint64, namePtr uint32, nameSize uint32) (result int32) {
	return f.safeApi.ValueAsInt32(isafeapi.TValue(id), f.decodeStr(namePtr, nameSize))
}

func (f *wazeroExtEngine) hostValueAsInt64(id uint64, namePtr uint32, nameSize uint32) (result uint64) {
	return uint64(f.safeApi.ValueAsInt64(isafeapi.TValue(id), f.decodeStr(namePtr, nameSize)))
}

func (f *wazeroExtEngine) hostValueAsBool(id uint64, namePtr uint32, nameSize uint32) (result uint64) {
	b := f.safeApi.ValueAsBool(isafeapi.TValue(id), f.decodeStr(namePtr, nameSize))
	if b {
		return 1
	}
	return 0
}

func (f *wazeroExtEngine) hostValueAsFloat32(id uint64, namePtr, nameSize uint32) float32 {
	return f.safeApi.ValueAsFloat32(isafeapi.TValue(id), f.decodeStr(namePtr, nameSize))
}

func (f *wazeroExtEngine) hostValueAsFloat64(id uint64, namePtr, nameSize uint32) float64 {
	return f.safeApi.ValueAsFloat64(isafeapi.TValue(id), f.decodeStr(namePtr, nameSize))
}

func (f *wazeroExtEngine) hostValueAsQNameEntity(id uint64, namePtr uint32, nameSize uint32) (result uint64) {
	qname := f.safeApi.ValueAsQName(isafeapi.TValue(id), f.decodeStr(namePtr, nameSize))
	return f.allocAndSend([]byte(qname.Entity))
}

func (f *wazeroExtEngine) hostValueAsQNamePkg(id uint64, namePtr uint32, nameSize uint32) (result uint64) {
	qname := f.safeApi.ValueAsQName(isafeapi.TValue(id), f.decodeStr(namePtr, nameSize))
	return f.allocAndSend([]byte(qname.FullPkgName))
}

func (f *wazeroExtEngine) hostValueAsValue(id uint64, namePtr uint32, nameSize uint32) (result uint64) {
	return uint64(f.safeApi.ValueAsValue(isafeapi.TValue(id), f.decodeStr(namePtr, nameSize)))
}

func (f *wazeroExtEngine) hostValueLength(id uint64) (result uint32) {
	return uint32(f.safeApi.ValueLen(isafeapi.TValue(id)))
}

func (f *wazeroExtEngine) allocBuf(size uint32) (addr uint32, err error) {
	for i := range f.pkg.allocatedBufs {
		if f.pkg.allocatedBufs[i].cap-f.pkg.allocatedBufs[i].offs >= size {
			addr = f.pkg.allocatedBufs[i].addr + f.pkg.allocatedBufs[i].offs
			f.pkg.allocatedBufs[i].offs += size
			return
		}
	}
	// no space in the allocated buffers

	var newBufferSize uint32 = WasmPreallocatedBufferIncrease
	if size > newBufferSize {
		newBufferSize = size
	}

	var res []uint64
	res, err = f.pkg.funcMalloc.Call(f.ctx, uint64(newBufferSize))
	if err != nil {
		return 0, err
	}
	addr = uint32(res[0])
	f.pkg.allocatedBufs = append(f.pkg.allocatedBufs, &allocatedBuf{
		addr: addr,
		offs: 0,
		cap:  newBufferSize,
	})
	return addr, nil
}

func (f *wazeroExtEngine) getFrees(packagePath string, ctx context.Context) (uint64, error) {
	pkg, ok := f.modules[packagePath]
	if !ok {
		return 0, errUndefinedPackage(packagePath)
	}
	res, err := pkg.funcGetFrees.Call(ctx)
	if err != nil {
		return 0, err
	}
	return res[0], nil
}

func (f *wazeroExtEngine) gc(packagePath string, ctx context.Context) error {
	pkg, ok := f.modules[packagePath]
	if !ok {
		return errUndefinedPackage(packagePath)
	}
	_, err := pkg.funcGc.Call(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (f *wazeroExtEngine) getHeapinuse(packagePath string, ctx context.Context) (uint64, error) {
	pkg, ok := f.modules[packagePath]
	if !ok {
		return 0, errUndefinedPackage(packagePath)
	}
	res, err := pkg.funcGetHeapInuse.Call(ctx)
	if err != nil {
		return 0, err
	}
	return res[0], nil
}

func (f *wazeroExtEngine) getHeapSys(packagePath string, ctx context.Context) (uint64, error) {
	pkg, ok := f.modules[packagePath]
	if !ok {
		return 0, errUndefinedPackage(packagePath)
	}
	res, err := pkg.funcGetHeapSys.Call(ctx)
	if err != nil {
		return 0, err
	}
	return res[0], nil
}

func (f *wazeroExtEngine) getMallocs(packagePath string, ctx context.Context) (uint64, error) {
	pkg, ok := f.modules[packagePath]
	if !ok {
		return 0, errUndefinedPackage(packagePath)
	}
	res, err := pkg.funcGetMallocs.Call(ctx)
	if err != nil {
		return 0, err
	}
	return res[0], nil
}

func (f *wazeroExtEngine) hostNewValue(keyId uint64) uint64 {
	return uint64(f.safeApi.NewValue(isafeapi.TKeyBuilder(keyId)))
}

func (f *wazeroExtEngine) hostUpdateValue(keyId, existingValueId uint64) (result uint64) {
	return uint64(f.safeApi.UpdateValue(isafeapi.TKeyBuilder(keyId), isafeapi.TValue(existingValueId)))
}

func (f *wazeroExtEngine) hostRowWriterPutString(id uint64, typ uint32, namePtr uint32, nameSize, valuePtr, valueSize uint32) {
	if typ == 0 {
		f.safeApi.KeyBuilderPutString(isafeapi.TKeyBuilder(id), f.decodeStr(namePtr, nameSize), f.decodeStr(valuePtr, valueSize))
	} else {
		f.safeApi.IntentPutString(isafeapi.TIntent(id), f.decodeStr(namePtr, nameSize), f.decodeStr(valuePtr, valueSize))
	}
}

func (f *wazeroExtEngine) hostRowWriterPutBytes(id uint64, typ uint32, namePtr uint32, nameSize, valuePtr, valueSize uint32) {
	var bytes []byte
	var ok bool
	bytes, ok = f.pkg.module.Memory().Read(valuePtr, valueSize)
	if !ok {
		panic(ErrUnableToReadMemory)
	}
	if typ == 0 {
		f.safeApi.KeyBuilderPutBytes(isafeapi.TKeyBuilder(id), f.decodeStr(namePtr, nameSize), bytes)
	} else {
		f.safeApi.IntentPutBytes(isafeapi.TIntent(id), f.decodeStr(namePtr, nameSize), bytes)
	}
}

func (f *wazeroExtEngine) hostRowWriterPutInt32(id uint64, typ uint32, namePtr uint32, nameSize uint32, value int32) {
	if typ == 0 {
		f.safeApi.KeyBuilderPutInt32(isafeapi.TKeyBuilder(id), f.decodeStr(namePtr, nameSize), value)
	} else {
		f.safeApi.IntentPutInt32(isafeapi.TIntent(id), f.decodeStr(namePtr, nameSize), value)
	}
}

func (f *wazeroExtEngine) hostRowWriterPutInt64(id uint64, typ uint32, namePtr uint32, nameSize uint32, value int64) {
	if typ == 0 {
		f.safeApi.KeyBuilderPutInt64(isafeapi.TKeyBuilder(id), f.decodeStr(namePtr, nameSize), value)
	} else {
		f.safeApi.IntentPutInt64(isafeapi.TIntent(id), f.decodeStr(namePtr, nameSize), value)
	}
}

func (f *wazeroExtEngine) hostRowWriterPutQName(id uint64, typ uint32, namePtr uint32, nameSize uint32, pkgPtr, pkgSize, entityPtr, entitySize uint32) {
	qname := isafeapi.QName{
		FullPkgName: f.decodeStr(pkgPtr, pkgSize),
		Entity:      f.decodeStr(entityPtr, entitySize),
	}
	if typ == 0 {
		f.safeApi.KeyBuilderPutQName(isafeapi.TKeyBuilder(id), f.decodeStr(namePtr, nameSize), qname)
	} else {
		f.safeApi.IntentPutQName(isafeapi.TIntent(id), f.decodeStr(namePtr, nameSize), qname)
	}
}

func (f *wazeroExtEngine) hostRowWriterPutBool(id uint64, typ uint32, namePtr uint32, nameSize uint32, value int32) {
	if typ == 0 {
		f.safeApi.KeyBuilderPutBool(isafeapi.TKeyBuilder(id), f.decodeStr(namePtr, nameSize), value > 0)
	} else {
		f.safeApi.IntentPutBool(isafeapi.TIntent(id), f.decodeStr(namePtr, nameSize), value > 0)
	}
}

func (f *wazeroExtEngine) hostRowWriterPutFloat32(id uint64, typ uint32, namePtr uint32, nameSize uint32, value float32) {
	if typ == 0 {
		f.safeApi.KeyBuilderPutFloat32(isafeapi.TKeyBuilder(id), f.decodeStr(namePtr, nameSize), value)
	} else {
		f.safeApi.IntentPutFloat32(isafeapi.TIntent(id), f.decodeStr(namePtr, nameSize), value)
	}
}

func (f *wazeroExtEngine) hostRowWriterPutFloat64(id uint64, typ uint32, namePtr, nameSize uint32, value float64) {
	if typ == 0 {
		f.safeApi.KeyBuilderPutFloat64(isafeapi.TKeyBuilder(id), f.decodeStr(namePtr, nameSize), value)
	} else {
		f.safeApi.IntentPutFloat64(isafeapi.TIntent(id), f.decodeStr(namePtr, nameSize), value)
	}
}

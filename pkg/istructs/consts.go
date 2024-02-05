/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 * Copyright (c) 2021-present unTill Pro, Ltd.
 * @author: Maxim Geraskin
 */

package istructs

import "github.com/voedger/voedger/pkg/appdef"

// *********************************************************************************************************
//
//				QName- & Names- related constants
//

// AppQNameQualifierChar: char to separate application owner (provider) from application name
const AppQNameQualifierChar = "/"

// NullAppQName is undefined (or empty) application name
var NullAppQName = NewAppQName(appdef.NullName, appdef.NullName)

var (
	// QNameForError is a marker of error in log
	QNameForError = appdef.NewQName(appdef.SysPackage, "Error")

	// QNameCommand is used in core-irates
	QNameCommand = appdef.NewQName(appdef.SysPackage, "Command")

	// QNameQuery is used in core-irates
	QNameQuery = appdef.NewQName(appdef.SysPackage, "Query")

	QNameCommandCUD = appdef.NewQName(appdef.SysPackage, "CUD")

	// QNameRaw denotes that Function argument comes as a JSON object
	QNameRaw = appdef.NewQName(appdef.SysPackage, "Raw")

	QNameCDoc    = appdef.NewQName(appdef.SysPackage, "CDoc")
	QNameWDoc    = appdef.NewQName(appdef.SysPackage, "WDoc")
	QNameODoc    = appdef.NewQName(appdef.SysPackage, "ODoc")
	QNameCRecord = appdef.NewQName(appdef.SysPackage, "CRecord")
	QNameWRecord = appdef.NewQName(appdef.SysPackage, "WRecord")
	QNameORecord = appdef.NewQName(appdef.SysPackage, "ORecord")
)

// *********************************************************************************************************
//
//				Record-related constants
//

// Null entities do not exist, Null-representations are returned

const NullRecordID = RecordID(0)
const NullOffset = Offset(0)
const FirstOffset = Offset(1)

// MinRawRecordID and MaxRawRecordID: range bounds for "raw" RecordIDs which are generated by client and must be re-generated
const MinRawRecordID = RecordID(1)
const MaxRawRecordID = RecordID(0xffff)

// RecordID generation
const (
	// RecordID = RegisterID * RegisterFactor + BaseRecordID
	RegisterFactor = 5000000000

	// ClusterDBSer is used to generate cluster-side IDs
	// ID = PrimaryDCBaseID + some sequenced number
	ClusterAsRegisterID = 0xFFFF - 1000 + iota
	ClusterAsCRecordRegisterID
)

var MinClusterRecordID = NewRecordID(NullRecordID)

// *********************************************************************************************************
//
//				Events-related constants
//

// It is 0x7FFF_FFFF_FFFF_FFFF for x64 architecture, ref. https://play.golang.org/p/HBoCflcoERV
// Used by IEvents.Read* methods
const ReadToTheEnd = int(^uint(0) >> 1)

// *********************************************************************************************************
//
//				Workspace-related constants
//

const NullWSID = WSID(0)

// WSID = ClusterID << WSIDClusterLShift + NextWSID()
const WSIDClusterLShift = 64 - 16 - 1

const MinReservedBaseRecordID = MaxRawRecordID + 1
const MaxReservedBaseRecordID = MinReservedBaseRecordID + 0xffff

// Singleton - CDoc which has at most one record
const FirstSingletonID = MinReservedBaseRecordID
const MaxSingletonID = FirstSingletonID + 0x1ff

// Used to test behaviour on providing the unexisting record ID
const NonExistingRecordID = MaxSingletonID + 1

// This is the first value which must be returned by the IDGenerator (in the Command Processor) for the given workspace
const FirstBaseRecordID = MaxReservedBaseRecordID + 1

// Pseudo Workspaces
const FirstPseudoBaseWSID = NullWSID
const MaxPseudoBaseWSID = WSID(0xffff)

// Application Workspaces
const FirstBaseAppWSID = MaxPseudoBaseWSID + 1

// User Workspaces
const FirstBaseUserWSID = FirstBaseAppWSID + 0xffff

// *********************************************************************************************************
//
//				Cluster-related constants
//

const (
	NullClusterID = ClusterID(iota)
	MainClusterID
)
const MaxClusterID = ClusterID(0xffff)

// Cluster application IDs

const (
	ClusterAppID_null = ClusterAppID(0) + iota
	ClusterAppID_sys_registry
	ClusterAppID_untill_airs_bp
	ClusterAppID_test1_app1
	ClusterAppID_test1_app2
	ClusterAppID_test2_app1
	ClusterAppID_test2_app2
	ClusterAppID_sys_blobber
	ClusterAppID_sys_router
	ClusterAppID_untill_resellerportal
	ClusterAppID_FakeLast
)

const NullClusterAppID = ClusterAppID_null
const FirstGeneratedAppID = ClusterAppID(0x100)

// Cluster application qnames

const SysOwner = "sys"

var AppQName_null = NullAppQName
var AppQName_sys_registry = NewAppQName(SysOwner, "registry")
var AppQName_untill_airs_bp = NewAppQName("untill", "airs-bp")
var AppQName_test1_app1 = NewAppQName("test1", "app1")
var AppQName_test1_app2 = NewAppQName("test1", "app2")
var AppQName_test2_app1 = NewAppQName("test2", "app1")
var AppQName_test2_app2 = NewAppQName("test2", "app2")
var AppQName_sys_blobber = NewAppQName(SysOwner, "blobber")
var AppQName_sys_router = NewAppQName(SysOwner, "router") // For ACME certificates
var AppQName_untill_resellerportal = NewAppQName("untill", "resellerportal")

// Cluster applications

var ClusterApps = map[AppQName]ClusterAppID{
	AppQName_null:                  ClusterAppID_null,
	AppQName_sys_registry:          ClusterAppID_sys_registry,
	AppQName_test1_app1:            ClusterAppID_test1_app1,
	AppQName_test1_app2:            ClusterAppID_test1_app2,
	AppQName_test2_app1:            ClusterAppID_test2_app1,
	AppQName_test2_app2:            ClusterAppID_test2_app2,
	AppQName_untill_airs_bp:        ClusterAppID_untill_airs_bp,
	AppQName_sys_blobber:           ClusterAppID_sys_blobber,
	AppQName_sys_router:            ClusterAppID_sys_router,
	AppQName_untill_resellerportal: ClusterAppID_untill_resellerportal,
}

const (
	RateLimitKind_byApp RateLimitKind = iota
	RateLimitKind_byWorkspace
	RateLimitKind_byID

	RateLimitKind_FakeLast
)

const DefaultAppWSAmount = 10

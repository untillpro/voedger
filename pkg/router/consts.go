/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package router

import (
	"sync/atomic"
	"time"
)

const (
	HTTPSPort                       = 443
	DefaultACMEServerReadTimeout    = 5 * time.Second
	DefaultACMEServerWriteTimeout   = 5 * time.Second
	subscriptionsCloseCheckInterval = 100 * time.Millisecond
	DefaultPort                     = 8822
	DefaultConnectionsLimit         = 10000

	//  covers the time from when the connection is accepted to when the
	// request body is fully read. It protects against slow or malicious
	// clients that send data too slowly (slowloris attacks).
	DefaultRouterReadTimeout = 15

	// corresponds to http.Server.WriteTimeout. For HTTP/1.x it covers the time
	// from when the connection is accepted until the response write completes.
	// A value of 0 disables the timeout entirely, which is our default.
	// It protects against slow clients that read responses too slowly, preventing
	// goroutines and connections from being held open indefinitely.
	DefaultRouterWriteTimeout = 0

	URLPlaceholder_wsid           = "wsid"
	URLPlaceholder_appOwner       = "appOwner"
	URLPlaceholder_appName        = "appName"
	URLPlaceholder_blobIDOrSUUID  = "blobIDOrSUUID"
	URLPlaceholder_resourceName   = "resourceName"
	URLPlaceholder_pkg            = "pkg"
	URLPlaceholder_table          = "table"
	URLPlaceholder_id             = "id"
	URLPlaceholder_command        = "command"
	URLPlaceholder_query          = "query"
	URLPlaceholder_view           = "view"
	URLPlaceholder_workspaceName  = "workspace"
	URLPlaceholder_rolePkg        = "rolePkg"
	URLPlaceholder_role           = "role"
	URLPlaceholder_channelID      = "channelID"
	URLPlaceholder_field          = "field"
	hours24                       = 24 * time.Hour
	DefaultRetryAfterSecondsOn503 = 1
	logAttrib_Origin              = "origin"
	logAttrib_ProjectionKey       = "projectionkey"
	logAttrib_ChannelID           = "channelid"
)

var (
	onRequestCtxClosed    func() = nil // used in tests
	reqID                        = atomic.Uint64{}
	globalServerStartTime        = time.Now().Format("01021504")
)

const (
	fieldLogin    = "login"
	fieldPassword = "password"
)

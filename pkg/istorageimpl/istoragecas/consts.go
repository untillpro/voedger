/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package istoragecas

import "time"

// ConnectionTimeout s.e.
const (
	initialConnectionTimeout = 30 * time.Second
	ConnectionTimeout        = 30 * time.Second
	attempts                 = 5
	retryAttempt             = 3
	SimpleWithReplication    = "{'class': 'SimpleStrategy', 'replication_factor': '1'}"
)

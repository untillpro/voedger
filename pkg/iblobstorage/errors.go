/*
 *  Copyright (c) 2021-present unTill Pro, Ltd.
 */

package iblobstorage

import "errors"

var (
	ErrBLOBNotFound          = errors.New("BLOB not found")
	ErrBLOBCorrupted         = errors.New("BLOB corrupted")
	ErrBLOBSizeQuotaExceeded = errors.New("BLOB size quote exceeded")
	ErrReadLimitReached      = errors.New("BLOB read limit reached")
)

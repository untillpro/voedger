/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package provider

import "errors"

var (
	ErrStorageInitError     = errors.New("storage init error")
	ErrStorageInitedAlready = errors.New("strorage inited already")
	ErrStorageNotInited     = errors.New("storage not inited")
)

/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 */

package coreutils

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/appdef"
)

func TestBasicUsage_SysError(t *testing.T) {
	require := require.New(t)

	t.Run("basic", func(t *testing.T) {
		testError := errors.New("test")
		err := WrapSysError(testError, http.StatusInternalServerError)
		var sysError SysError
		require.ErrorAs(err, &sysError)
		require.Equal("test", sysError.Message)
		require.Equal(http.StatusInternalServerError, sysError.HTTPStatus)
		require.Empty(sysError.Data)
		require.Empty(sysError.QName)
		require.Equal("test", sysError.Error())
		require.Equal(`{"sys.Error":{"HTTPStatus":500,"Message":"test"}}`, sysError.ToJSON())
	})

	t.Run("nil on nil", func(t *testing.T) {
		require.NoError(WrapSysError(nil, http.StatusInternalServerError))
	})

	t.Run("wrap error that is SysError already", func(t *testing.T) {
		err := SysError{
			HTTPStatus: http.StatusOK,
			QName:      appdef.NewQName("my", "test"),
			Message:    "test",
			Data:       "data",
		}
		wrapped := WrapSysError(err, http.StatusInternalServerError)
		require.Equal(err, wrapped)
	})

	t.Run("ToJSON", func(t *testing.T) {
		err := SysError{
			HTTPStatus: http.StatusOK,
			QName:      appdef.NewQName("my", "test"),
			Message:    "test",
			Data:       "data",
		}
		require.Equal(`{"sys.Error":{"HTTPStatus":200,"Message":"test","QName":"my.test","Data":"data"}}`, err.ToJSON())
	})

	t.Run("NewSysError", func(t *testing.T) {
		sysErr := NewSysError(http.StatusContinue).(SysError)
		require.Empty(sysErr.Data)
		require.Equal(http.StatusContinue, sysErr.HTTPStatus)
		require.Empty(sysErr.Message)
		require.Equal(appdef.NullQName, sysErr.QName)
	})

	t.Run("emit status code with desc if message is empty but code > 0", func(t *testing.T) {
		sysErr := NewSysError(http.StatusContinue).(SysError)
		require.Equal("100 Continue", sysErr.Error())
	})

	t.Run("empty", func(t *testing.T) {
		sysErr := SysError{}
		require.Empty(sysErr.Error())
	})
}

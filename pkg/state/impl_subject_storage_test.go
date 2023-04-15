/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 */

package state

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/untillpro/voedger/pkg/iauthnz"
	"github.com/untillpro/voedger/pkg/istructs"
)

func TestSubjectStorage_BasicUsage(t *testing.T) {
	require := require.New(t)
	principals := []iauthnz.Principal{{
		Kind: iauthnz.PrincipalKind_User,
		WSID: 42,
		Name: "john.doe@acme.com",
	}}
	token := "token"
	tokenFunc := func() string { return token }
	s := ProvideCommandProcessorStateFactory()(context.Background(), func() istructs.IAppStructs { return &nilAppStructs{} }, nil, nil, nil, nil, func() []iauthnz.Principal { return principals }, tokenFunc, 1)
	k, err := s.KeyBuilder(SubjectStorage, istructs.NullQName)
	require.Nil(err)

	v, err := s.MustExist(k)
	require.Nil(err)

	require.Equal(int64(principals[0].WSID), v.AsInt64(Field_ProfileWSID))
	require.Equal(int32(istructs.SubjectKind_User), v.AsInt32(Field_Kind))
	require.Equal(principals[0].Name, v.AsString(Field_Name))
	require.Equal(token, v.AsString(Field_Token))
	json, _ := v.ToJSON()
	require.JSONEq(`{"ProfileWSID":42,"Kind":1,"Name":"john.doe@acme.com","Token":"token"}`, json)
}

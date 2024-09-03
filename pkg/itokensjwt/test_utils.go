/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 */

package itokensjwt

import (
	"github.com/voedger/voedger/pkg/isecrets"
	itokens "github.com/voedger/voedger/pkg/itokens"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

var TestTokensJWTWithTime = func(time coreutils.ITime) itokens.ITokens {
	return ProvideITokens(SecretKeyExample, time)
}

var TestTokensJWT = func() itokens.ITokens {
	return ProvideITokens(SecretKeyExample, coreutils.MockTime)
}

func ProvideTestSecretsReader(realSecretsReader isecrets.ISecretReader) isecrets.ISecretReader {
	return &testISecretReader{realSecretReader: realSecretsReader}
}

type testISecretReader struct {
	realSecretReader isecrets.ISecretReader
}

func (tsr *testISecretReader) ReadSecret(name string) ([]byte, error) {
	if name == SecretKeyJWTName {
		return SecretKeyExample, nil
	}
	return tsr.realSecretReader.ReadSecret(name)
}

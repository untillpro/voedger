/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package state

import (
	"errors"
	"io/fs"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/isecrets"
	"github.com/voedger/voedger/pkg/istructs"
)

type appSecretsStorage struct {
	secretReader isecrets.ISecretReader
}

type appSecretsStorageKeyBuilder struct {
	baseKeyBuilder
	secret string
}

func (b *appSecretsStorageKeyBuilder) Storage() appdef.QName {
	return AppSecret
}
func (b *appSecretsStorageKeyBuilder) Equals(src istructs.IKeyBuilder) bool {
	kb, ok := src.(*appSecretsStorageKeyBuilder)
	if !ok {
		return false
	}
	if b.secret != kb.secret {
		return false
	}
	return true
}
func (b *appSecretsStorageKeyBuilder) PutString(name string, value string) {
	if name == Field_Secret {
		b.secret = value
		return
	}
	b.baseKeyBuilder.PutString(name, value)
}

type appSecretValue struct {
	baseStateValue
	content string
}

func (v *appSecretValue) AsString(name string) string {
	return v.content
}

func (s *appSecretsStorage) NewKeyBuilder(appdef.QName, istructs.IStateKeyBuilder) istructs.IStateKeyBuilder {
	return &appSecretsStorageKeyBuilder{}
}
func (s *appSecretsStorage) Get(key istructs.IStateKeyBuilder) (value istructs.IStateValue, err error) {
	k := key.(*appSecretsStorageKeyBuilder)
	if k.secret == "" {
		return nil, errors.New("secret name is not specified")
	}
	bb, e := s.secretReader.ReadSecret(k.secret)
	if errors.Is(e, fs.ErrNotExist) || errors.Is(e, isecrets.ErrSecretNameIsBlank) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &appSecretValue{
		content: string(bb),
	}, nil
}

/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package state

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/itokens"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	coreutils "github.com/voedger/voedger/pkg/utils"
	"github.com/voedger/voedger/pkg/utils/federation"
)

const (
	ContentType = "Content-Type"
)

type FederationCommandHandler = func(owner, appname string, wsid istructs.WSID, command appdef.QName, body string) (statusCode int, newIDs map[string]int64, result string, err error)
type federationCommandStorage struct {
	appStructs AppStructsFunc
	wsid       WSIDFunc
	federation federation.IFederation
	tokens     itokens.ITokens
	emulation  FederationCommandHandler
}

type federationCommandKeyBuilder struct {
	baseKeyBuilder
	expectedCodes string
	owner         string
	appname       string
	wsid          istructs.WSID
	command       appdef.QName
	body          string
	token         string
}

func (b *federationCommandKeyBuilder) PutString(name string, value string) {
	if name == Field_Owner {
		b.owner = value
	} else if name == Field_AppName {
		b.appname = value
	} else if name == Field_Body {
		b.body = value
	} else if name == Field_Token {
		b.token = value
	} else if name == Field_ExpectedCodes {
		b.expectedCodes = value
	} else {
		b.baseKeyBuilder.PutString(name, value)
	}
}

func (b *federationCommandKeyBuilder) PutInt64(name string, value int64) {
	if name == Field_WSID {
		b.wsid = istructs.WSID(value)
	} else {
		b.baseKeyBuilder.PutInt64(name, value)
	}
}

func (b *federationCommandKeyBuilder) PutQName(name string, value appdef.QName) {
	if name == Field_Command {
		b.command = value
	} else {
		b.baseKeyBuilder.PutQName(name, value)
	}
}

func (b *federationCommandKeyBuilder) Storage() appdef.QName {
	return FederationCommand
}

func (b *federationCommandKeyBuilder) Equals(src istructs.IKeyBuilder) bool {
	_, ok := src.(*federationCommandKeyBuilder)
	if !ok {
		return false
	}
	kb := src.(*federationCommandKeyBuilder)
	if b.owner != kb.owner {
		return false
	}
	if b.appname != kb.appname {
		return false
	}
	if b.wsid != kb.wsid {
		return false
	}
	if b.command != kb.command {
		return false
	}
	if b.body != kb.body {
		return false
	}
	if b.token != kb.token {
		return false
	}
	if b.expectedCodes != kb.expectedCodes {
		return false
	}
	return true
}

func (s *federationCommandStorage) NewKeyBuilder(appdef.QName, istructs.IStateKeyBuilder) istructs.IStateKeyBuilder {
	return &federationCommandKeyBuilder{}
}
func (s *federationCommandStorage) Get(key istructs.IStateKeyBuilder) (istructs.IStateValue, error) {
	appqname := s.appStructs().AppQName()
	var owner string
	var appname string
	var wsid istructs.WSID
	var command appdef.QName
	var body string
	opts := make([]coreutils.ReqOptFunc, 0)

	kb := key.(*federationCommandKeyBuilder)

	for _, ec := range strings.Split(kb.expectedCodes, ",") {
		if ec == "" {
			continue
		}
		code, err := strconv.Atoi(ec)
		if err != nil {
			return nil, err
		}
		opts = append(opts, coreutils.WithExpectedCode(code))
	}

	if kb.owner != "" {
		owner = kb.owner
	} else {
		owner = appqname.Owner()
	}

	if kb.appname != "" {
		appname = kb.appname
	} else {
		appname = appqname.Name()
	}

	if kb.wsid != 0 {
		wsid = kb.wsid
	} else {
		wsid = s.wsid()
	}

	if kb.command != appdef.NullQName {
		command = kb.command
	} else {
		return nil, errCommandNotSpecified
	}

	if kb.body != "" {
		body = kb.body
	}

	appOwnerAndName := owner + appdef.AppQNameQualifierChar + appname

	relativeUrl := fmt.Sprintf("api/%s/%d/c.%s", appOwnerAndName, wsid, command)

	var resStatus int
	var resBody string
	var newIDs map[string]int64
	var err error
	var result map[string]interface{}

	if s.emulation != nil {
		resStatus, newIDs, resBody, err = s.emulation(owner, appname, wsid, command, body)
		if err != nil {
			return nil, err
		}
		result = map[string]interface{}{}
		if resBody != "" {
			err = json.Unmarshal([]byte(resBody), &result)
			if err != nil {
				return nil, err
			}
		}
	} else {

		if kb.token != "" {
			opts = append(opts, coreutils.WithAuthorizeBy(kb.token))
		} else {
			appQName := appdef.NewAppQName(owner, appname)
			systemPrincipalToken, err := payloads.GetSystemPrincipalToken(s.tokens, appQName)
			if err != nil {
				return nil, err
			}
			opts = append(opts, coreutils.WithAuthorizeBy(systemPrincipalToken))
		}

		resp, err := s.federation.Func(relativeUrl, body, opts...)
		if err != nil {
			return nil, err
		}

		newIDs = resp.NewIDs
		resStatus = resp.HTTPResp.StatusCode
		result = resp.CmdResult

	}

	return &fcCmdValue{
		statusCode: resStatus,
		newIds:     &fcCmdNewIds{newIds: newIDs},
		result:     &jsonValue{json: result},
		body:       resBody,
	}, nil
}
func (s *federationCommandStorage) Read(key istructs.IStateKeyBuilder, callback istructs.ValueCallback) (err error) {
	v, err := s.Get(key)
	if err != nil {
		return err
	}
	return callback(nil, v)
}

type fcCmdValue struct {
	baseStateValue
	statusCode int
	newIds     istructs.IStateValue
	result     istructs.IStateValue
	body       string
}

func (v *fcCmdValue) AsInt32(name string) int32 {
	if name == Field_StatusCode {
		return int32(v.statusCode)
	}
	return v.baseStateValue.AsInt32(name)
}

func (v *fcCmdValue) AsValue(name string) istructs.IStateValue {
	if name == Field_NewIDs {
		return v.newIds
	}
	if name == Field_Result {
		return v.result
	}
	return v.baseStateValue.AsValue(name)
}

type fcCmdNewIds struct {
	baseStateValue
	newIds map[string]int64
}

func (v *fcCmdNewIds) AsInt64(name string) int64 {
	if id, ok := v.newIds[name]; ok {
		return id
	}
	panic(errInt64FieldUndefined(name))
}

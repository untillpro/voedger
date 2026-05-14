/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package registry

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/sys"
)

func CheckAppWSID(login string, urlWSID istructs.WSID, numAppWorkspaces istructs.NumAppWorkspaces) error {
	crc16 := coreutils.CRC16([]byte(login))
	appWSID := istructs.WSID(istructs.NumAppWorkspaces(crc16)%numAppWorkspaces) + istructs.FirstBaseAppWSID
	expectedAppWSID := istructs.NewWSID(urlWSID.ClusterID(), appWSID)
	if expectedAppWSID != urlWSID {
		return coreutils.NewHTTPErrorf(http.StatusForbidden, "wrong AppWSID: ", expectedAppWSID, " expected, ", urlWSID, " got")
	}
	return nil
}

// istructs.NullRecordID means not found
func GetCDocLoginID(st istructs.IState, appWSID istructs.WSID, appName string, login string) (cdocLoginID istructs.RecordID, err error) {
	kb, err := st.KeyBuilder(sys.Storage_View, QNameViewLoginIdx)
	if err != nil {
		return istructs.NullRecordID, err
	}
	loginHash := GetLoginHash(login)
	kb.PutInt64(field_AppWSID, int64(appWSID)) // nolint G115
	kb.PutString(field_AppIDLoginHash, appName+"/"+loginHash)
	loginIdx, ok, err := st.CanExist(kb)
	if err != nil || !ok {
		return istructs.NullRecordID, err
	}
	return loginIdx.AsRecordID(field_CDocLoginID), nil
}

func GetCDocLogin(login string, st istructs.IState, appWSID istructs.WSID, appName string) (cdocLogin istructs.IStateValue, loginExists bool, err error) {
	cdocLoginID, err := GetCDocLoginID(st, appWSID, appName, login)
	if err != nil {
		return nil, false, err
	}
	if cdocLoginID == istructs.NullRecordID {
		return nil, false, nil
	}

	kb, err := st.KeyBuilder(sys.Storage_Record, QNameCDocLogin)
	if err != nil {
		return nil, false, err
	}
	kb.PutRecordID(sys.Storage_Record_Field_ID, cdocLoginID)
	cdocLogin, err = st.MustExist(kb)
	if err != nil {
		return nil, false, err
	}
	if !cdocLogin.AsBool(appdef.SystemField_IsActive) {
		logger.Verbose("cdoc.registry.Login", cdocLoginID, "is deactivated, treating as missing login")
		return nil, false, nil
	}
	return cdocLogin, true, nil
}

func GetLoginHash(login string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(login)))
}

func ChangePassword(login string, st istructs.IState, intents istructs.IIntents, wsid istructs.WSID, appName string, newPwd string) error {
	cdocLogin, loginExists, err := GetCDocLogin(login, st, wsid, appName)
	if err != nil {
		return err
	}

	if !loginExists {
		return errLoginDoesNotExist(login)
	}

	return ChangePasswordCDocLogin(cdocLogin, newPwd, intents, st)
}

func UpdateGlobalRoles(login string, st istructs.IState, intents istructs.IIntents, wsid istructs.WSID, appName string, globalRoles string) error {
	cdocLogin, loginExists, err := GetCDocLogin(login, st, wsid, appName)
	if err != nil {
		return err
	}
	if !loginExists {
		return errLoginDoesNotExist(login)
	}
	kb, err := st.KeyBuilder(sys.Storage_Record, appdef.NullQName)
	if err != nil {
		return err
	}
	loginUpdater, err := intents.UpdateValue(kb, cdocLogin)
	if err != nil {
		return err
	}
	loginUpdater.PutString(field_GlobalRoles, globalRoles)
	return nil
}

func errLoginDoesNotExist(login string) error {
	return coreutils.NewHTTPErrorf(http.StatusUnauthorized, fmt.Errorf("login %s does not exist", login))
}

func ChangePasswordCDocLogin(cdocLogin istructs.IStateValue, newPwd string, intents istructs.IIntents, st istructs.IState) error {
	kb, err := st.KeyBuilder(sys.Storage_Record, appdef.NullQName)
	if err != nil {
		return err
	}
	loginUpdater, err := intents.UpdateValue(kb, cdocLogin)
	if err != nil {
		return err
	}
	newPwdSaltedHash, err := GetPasswordSaltedHash(newPwd)
	if err != nil {
		return err
	}
	loginUpdater.PutBytes(field_PwdHash, newPwdSaltedHash)
	return nil
}

func GetPasswordSaltedHash(pwd string) (pwdSaltedHash []byte, err error) {
	if pwdSaltedHash, err = bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.MinCost); err != nil {
		err = fmt.Errorf("password salting & hashing failed: %w", err)
	}
	return
}

func CheckPassword(cdocLogin istructs.IStateValue, pwd string) (isPasswordOK bool, err error) {
	if err := bcrypt.CompareHashAndPassword(cdocLogin.AsBytes(field_PwdHash), []byte(pwd)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}
		return false, fmt.Errorf("failed to authenticate: %w", err)
	}
	return true, nil
}

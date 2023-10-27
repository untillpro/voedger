/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package registry

import (
	"crypto/sha256"
	"fmt"
	"net/http"

	"github.com/untillpro/goutils/logger"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/state"
	coreutils "github.com/voedger/voedger/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

func CheckAppWSID(login string, urlWSID istructs.WSID, appWSAmount istructs.AppWSAmount) error {
	crc16 := coreutils.CRC16([]byte(login))
	appWSID := istructs.WSID(crc16%uint16(appWSAmount)) + istructs.FirstBaseAppWSID
	expectedAppWSID := istructs.NewWSID(urlWSID.ClusterID(), appWSID)
	if expectedAppWSID != urlWSID {
		return coreutils.NewHTTPErrorf(http.StatusForbidden, "wrong url WSID: ", expectedAppWSID, " expected, ", urlWSID, " got")
	}
	return nil
}

// istructs.NullRecordID means not found
func GetCDocLoginID(st istructs.IState, appWSID istructs.WSID, appName string, login string) (cdocLoginID istructs.RecordID, err error) {
	kb, err := st.KeyBuilder(state.View, QNameViewLoginIdx)
	if err != nil {
		return istructs.NullRecordID, err
	}
	loginHash := GetLoginHash(login)
	kb.PutInt64(field_AppWSID, int64(appWSID))
	kb.PutString(field_AppIDLoginHash, appName+"/"+loginHash)
	logger.Info(2)
	loginIdx, ok, err := st.CanExist(kb)
	if err != nil {
		logger.Error(2, err)
		return istructs.NullRecordID, err
	}
	logger.Info(3)
	if !ok {
		return istructs.NullRecordID, nil
	}
	return loginIdx.AsRecordID(field_CDocLoginID), nil

}

func GetCDocLogin(login string, st istructs.IState, appWSID istructs.WSID, appName string) (cdocLogin istructs.IStateValue, doesLoginExist bool, err error) {
	cdocLoginID, err := GetCDocLoginID(st, appWSID, appName, login)
	doesLoginExist = true
	if err != nil {
		return nil, doesLoginExist, err
	}
	if cdocLoginID == istructs.NullRecordID {
		doesLoginExist = false
		return nil, doesLoginExist, err
	}

	kb, err := st.KeyBuilder(state.Record, QNameCDocLogin)
	if err != nil {
		return nil, doesLoginExist, err
	}
	kb.PutRecordID(state.Field_ID, cdocLoginID)
	logger.Info(4)
	cdocLogin, err = st.MustExist(kb)
	if err != nil {
		logger.Error(4, err)
	}
	return
}

func GetLoginHash(login string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(login)))
}

func ChangePassword(login string, st istructs.IState, intents istructs.IIntents, wsid istructs.WSID, appName string, newPwd string) error {
	cdocLogin, doesLoginExist, err := GetCDocLogin(login, st, wsid, appName)
	if err != nil {
		return err
	}

	if !doesLoginExist {
		return errLoginDoesNotExist(login)
	}

	return ChangePasswordCDocLogin(cdocLogin, newPwd, intents, st)
}

func errLoginDoesNotExist(login string) error {
	return coreutils.NewHTTPErrorf(http.StatusUnauthorized, fmt.Errorf("login %s does not exist", login))
}

func ChangePasswordCDocLogin(cdocLogin istructs.IStateValue, newPwd string, intents istructs.IIntents, st istructs.IState) error {
	kb, err := st.KeyBuilder(state.Record, appdef.NullQName)
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
	isPasswordOK = true
	if err := bcrypt.CompareHashAndPassword(cdocLogin.AsBytes(field_PwdHash), []byte(pwd)); err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			isPasswordOK = false
			return isPasswordOK, nil
		}
		return isPasswordOK, fmt.Errorf("failed to authenticate: %w", err)
	}
	return isPasswordOK, err
}

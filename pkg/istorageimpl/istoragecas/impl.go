/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package istoragecas

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gocql/gocql"
	"github.com/untillpro/goutils/logger"
	istorage "github.com/voedger/voedger/pkg/istorage"
)

type appStorageProviderType struct {
	casPar  CassandraParamsType
	cluster *gocql.ClusterConfig
}

func newStorageProvider(casPar CassandraParamsType) (prov *appStorageProviderType) {
	provider := appStorageProviderType{
		casPar: casPar,
	}
	provider.cluster = gocql.NewCluster(strings.Split(casPar.Hosts, ",")...)
	if casPar.Port > 0 {
		provider.cluster.Port = casPar.Port
	}
	if casPar.NumRetries <= 0 {
		casPar.NumRetries = retryAttempt
	}
	retryPolicy := gocql.SimpleRetryPolicy{NumRetries: casPar.NumRetries}
	provider.cluster.Consistency = gocql.Quorum
	provider.cluster.ConnectTimeout = initialConnectionTimeout
	provider.cluster.Timeout = ConnectionTimeout
	provider.cluster.RetryPolicy = &retryPolicy
	provider.cluster.Authenticator = gocql.PasswordAuthenticator{Username: casPar.Username, Password: casPar.Pwd}
	provider.cluster.CQLVersion = casPar.cqlVersion()
	provider.cluster.ProtoVersion = casPar.ProtoVersion
	return &provider
}

func (p appStorageProviderType) AppStorage(appName istorage.SafeAppName) (storage istorage.IAppStorage, err error) {
	session, err := getSession(p.cluster)
	if err != nil {
		// notest
		return nil, err
	}
	defer session.Close()
	keyspaceExists, err := isKeyspaceExists(appName.String(), session)
	if err != nil {
		return nil, err
	}
	if !keyspaceExists {
		return nil, istorage.ErrStorageDoesNotExist
	}
	if storage, err = newStorage(p.cluster, appName.String()); err != nil {
		return nil, fmt.Errorf("can't create application «%s» keyspace: %w", appName, err)
	}
	return storage, nil
}

func isKeyspaceExists(name string, session *gocql.Session) (bool, error) {
	dummy := ""
	q := "select keyspace_name from system_schema.keyspaces where keyspace_name = ?;"
	logScript(q)
	if err := session.Query(q, name).Scan(&dummy); err != nil {
		if err == gocql.ErrNotFound {
			return false, nil
		}
		// notest
		return false, err
	}
	return true, nil
}

func logScript(q string) {
	if logger.IsVerbose() {
		logger.Verbose("executing script:", q)
	}
}

func (p appStorageProviderType) Init(appName istorage.SafeAppName) error {
	session, err := getSession(p.cluster)
	if err != nil {
		// notest
		return err
	}
	defer session.Close()
	keyspace := appName.String()
	keyspaceExists, err := isKeyspaceExists(keyspace, session)
	if err != nil {
		// notest
		return err
	}
	if keyspaceExists {
		return istorage.ErrStorageAlreadyExists
	}

	// create keyspace
	//
	q := fmt.Sprintf("create keyspace %s with replication = %s;", keyspace, p.casPar.KeyspaceWithReplication)
	logScript(q)
	err = session.
		Query(q).
		Consistency(gocql.Quorum).
		Exec()
	if err != nil {
		return fmt.Errorf("failed to create keyspace %s: %w", keyspace, err)
	}

	// prepare storage tables
	q = fmt.Sprintf(`create table if not exists %s.values (p_key blob, c_col blob, value blob, primary key ((p_key), c_col))`, keyspace)
	logScript(q)
	if err = session.Query(q).
		Consistency(gocql.Quorum).Exec(); err != nil {
		return fmt.Errorf("can't create table «values»: %w", err)
	}
	return nil
}

type appStorageType struct {
	cluster  *gocql.ClusterConfig
	session  *gocql.Session
	keyspace string
}

func getSession(cluster *gocql.ClusterConfig) (*gocql.Session, error) {
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("can't create session: %w", err)
	}
	return session, err
}

func newStorage(cluster *gocql.ClusterConfig, keyspace string) (storage istorage.IAppStorage, err error) {
	session, err := getSession(cluster)
	if err != nil {
		return nil, err
	}

	return &appStorageType{
		cluster:  cluster,
		session:  session,
		keyspace: keyspace,
	}, nil
}

func safeCcols(value []byte) []byte {
	if value == nil {
		return []byte{}
	}
	return value
}

func (s *appStorageType) Put(pKey []byte, cCols []byte, value []byte) (err error) {
	q := fmt.Sprintf("insert into %s.values (p_key, c_col, value) values (?,?,?)", s.keyspace)
	return s.session.Query(q,
		pKey,
		safeCcols(cCols),
		value).
		Consistency(gocql.Quorum).
		Exec()
}

func (s *appStorageType) PutBatch(items []istorage.BatchItem) (err error) {
	batch := s.session.NewBatch(gocql.LoggedBatch)
	batch.SetConsistency(gocql.Quorum)
	stmt := fmt.Sprintf("insert into %s.values (p_key, c_col, value) values (?,?,?)", s.keyspace)
	for _, item := range items {
		batch.Query(stmt, item.PKey, safeCcols(item.CCols), item.Value)
	}
	return s.session.ExecuteBatch(batch)
}

func scanViewQuery(ctx context.Context, q *gocql.Query, cb istorage.ReadCallback) (err error) {
	q.Consistency(gocql.Quorum)
	scanner := q.Iter().Scanner()
	sc := scannerCloser(scanner)
	for scanner.Next() {
		clustCols := make([]byte, 0)
		viewRecord := make([]byte, 0)
		err = scanner.Scan(&clustCols, &viewRecord)
		if err != nil {
			return sc(err)
		}
		err = cb(clustCols, viewRecord)
		if err != nil {
			return sc(err)
		}
		if ctx.Err() != nil {
			return nil // TCK contract
		}
	}
	return sc(nil)
}

func (s *appStorageType) Read(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb istorage.ReadCallback) (err error) {
	if (len(startCCols) > 0) && (len(finishCCols) > 0) && (bytes.Compare(startCCols, finishCCols) >= 0) {
		return nil // absurd range
	}

	qText := fmt.Sprintf("select c_col, value from %s.values where p_key=?", s.keyspace)

	var q *gocql.Query
	if len(startCCols) == 0 {
		if len(finishCCols) == 0 {
			// opened range
			q = s.session.Query(qText, pKey)
		} else {
			// left-opened range
			q = s.session.Query(qText+" and c_col<?", pKey, finishCCols)
		}
	} else if len(finishCCols) == 0 {
		// right-opened range
		q = s.session.Query(qText+" and c_col>=?", pKey, startCCols)
	} else {
		// closed range
		q = s.session.Query(qText+" and c_col>=? and c_col<?", pKey, startCCols, finishCCols)
	}

	return scanViewQuery(ctx, q, cb)
}

func (s *appStorageType) Get(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
	*data = (*data)[0:0]
	q := fmt.Sprintf("select value from %s.values where p_key=? and c_col=?", s.keyspace)
	err = s.session.Query(q, pKey, safeCcols(cCols)).
		Consistency(gocql.Quorum).
		Scan(data)
	if errors.Is(err, gocql.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (s *appStorageType) GetBatch(pKey []byte, items []istorage.GetBatchItem) (err error) {
	ccToIdx := make(map[string][]int)
	values := make([]interface{}, 0, len(items)+1)
	values = append(values, pKey)

	stmt := strings.Builder{}
	stmt.WriteString("select c_col, value from ")
	stmt.WriteString(s.keyspace)
	stmt.WriteString(".values where p_key=? and ")
	stmt.WriteString("c_col in (")
	for i, item := range items {
		items[i].Ok = false
		values = append(values, item.CCols)
		ccToIdx[string(item.CCols)] = append(ccToIdx[string(item.CCols)], i)
		stmt.WriteRune('?')
		if i < len(items)-1 {
			stmt.WriteRune(',')
		}
	}
	stmt.WriteRune(')')

	scanner := s.session.Query(stmt.String(), values...).
		Consistency(gocql.Quorum).
		Iter().
		Scanner()
	sc := scannerCloser(scanner)

	for scanner.Next() {
		ccols := make([]byte, 0)
		value := make([]byte, 0)
		err = scanner.Scan(&ccols, &value)
		if err != nil {
			return sc(err)
		}
		ii, ok := ccToIdx[string(ccols)]
		if ok {
			for _, i := range ii {
				items[i].Ok = true
				*items[i].Data = append((*items[i].Data)[0:0], value...)
			}
		}
	}

	return sc(nil)
}

func scannerCloser(scanner gocql.Scanner) func(error) error {
	return func(err error) error {
		if scannerErr := scanner.Err(); scannerErr != nil {
			if err != nil {
				err = fmt.Errorf("%s %w", err.Error(), scannerErr)
			} else {
				err = scannerErr
			}
		}
		return err
	}
}

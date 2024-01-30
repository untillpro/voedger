/*
 * Copyright (c) 2023-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package apps

import (
	"github.com/voedger/voedger/pkg/extensionpoints"
	"github.com/voedger/voedger/pkg/istorageimpl/istoragecas"
)

const (
	EPSchemasFS             extensionpoints.EPKey = "SchemasFS"
	storageTypeCas1         string                = "cas1"
	storageTypeCas3         string                = "cas3"
	storageTypeMem          string                = "mem"
	cas1ReplicationStrategy string                = "{'class': 'SimpleStrategy', 'replication_factor': '1'}"
	cas3ReplicationStrategy string                = "{ 'class': 'NetworkTopologyStrategy', 'dc1': 2, 'dc2': 1}"
)

const (
	defaultGrafanaPort    = 3000
	defaultPrometheusPort = 9090
	defaultCassandraPort  = 9042
)

var defaultCasParams = istoragecas.CassandraParamsType{
	Hosts:    "db-node-1,db-node-2,db-node-3",
	Port:     defaultCassandraPort,
	Username: "cassandra",
	Pwd:      "cassandra",
}

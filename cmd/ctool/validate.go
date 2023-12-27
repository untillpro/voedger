/*
* Copyright (c) 2023-present Sigma-Soft, Ltd.
* @author Dmitry Molchanovsky
 */

package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validates the configuration and status of the cluster for errors",
		RunE:  validate,
	}
}

func validate(cmd *cobra.Command, arg []string) error {

	cluster := newCluster()

	// nolint
	mkCommandDirAndLogFile(cmd, cluster)

	if !cluster.exists {
		return ErrClusterConfNotFound
	}

	err := cluster.validate()
	if err == nil {
		loggerInfoGreen("cluster configuration is ok")
	}

	if !cluster.Draft && !cluster.Cmd.isEmpty() {
		err = errors.Join(err, ErrUncompletedCommandFound)
	}

	if e := cluster.checkVersion(); e != nil {
		err = errors.Join(err, e)
	}

	return err
}

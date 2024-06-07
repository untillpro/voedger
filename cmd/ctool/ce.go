/*
* Copyright (c) 2023-present Sigma-Soft, Ltd.
* @author Dmitry Molchanovsky
 */
package main

import (
	"fmt"
	"os"
	"path/filepath"

	coreutils "github.com/voedger/voedger/pkg/utils"
)

var ceSuccessPhrases = map[string]string{
	ckInit:    "CE cluster is deployed successfully.",
	ckUpgrade: "CE cluster is upgraded successfully.",
	ckAcme:    "ACME domain list successfully modified",
}

func ceClusterControllerFunction(c *clusterType) error {

	var err error

	switch c.Cmd.Kind {
	case ckInit, ckUpgrade, ckAcme:

		if err = deployCeMonStack(); err != nil {
			return err
		}

		if err = deployVoedgerCe(); err != nil {
			return err
		}

		if err = addVoedgerUser(c); err != nil {
			return err
		}

		if c.Cmd.Kind == ckInit {
			if err = resetMonPassword(c); err != nil {
				return err
			}
		}

	default:
		err = ErrUnknownCommand
	}

	if err == nil {

		if succesPhrase, exists := ceSuccessPhrases[c.Cmd.Kind]; exists {
			loggerInfoGreen(succesPhrase)
		}

		c.success()
	}

	return err
}

func deployCeMonStack() error {

	loggerInfo("Deploying monitoring stack...")
	return newScriptExecuter("", "").run("ce/mon-prepare.sh")
}

func deployVoedgerCe() error {

	loggerInfo("Deploying voedger CE...")
	return newScriptExecuter("", "").run("ce/ce-start.sh")
}

func addVoedgerUser(c *clusterType) error {

	loggerInfo("Adding user voedger to Grafana on ce-node")
	if err := addGrafanUser(c.nodeByHost(ceNodeName), voedger); err != nil {
		return err
	}

	return nil
}

func resetMonPassword(c *clusterType) error {

	loggerInfo("Voedger's password resetting to monitoring stack")
	if err := setMonPassword(c, voedger); err != nil {
		return err
	}

	return nil
}

func ceNodeControllerFunction(n *nodeType) error {

	loggerInfo(fmt.Sprintf("Deploying docker on a %s %s host...", n.nodeName(), n.address()))
	if err := newScriptExecuter(n.cluster.sshKey, "").
		run("ce/docker-install.sh"); err != nil {
		return err
	}

	if err := copyCtoolToCeNode(n); err != nil {
		return err
	}

	n.success()
	return nil
}

// nolint
func deployCeCluster(cluster *clusterType) error {
	return nil
}

func copyCtoolToCeNode(node *nodeType) error {

	ctoolPath, err := os.Executable()

	ok, e := coreutils.Exists(node.cluster.configFileName)

	if e != nil {
		return e
	}

	if !ok {
		if e := node.cluster.saveToJSON(); err != nil {
			return e
		}
	}

	if err != nil {
		node.Error = err.Error()
		return err
	}

	loggerInfo(fmt.Sprintf("Copying ctool and configuration file to %s", ctoolPath))
	if err := newScriptExecuter("", "").
		run("ce/copy-ctool.sh", filepath.Dir(ctoolPath)); err != nil {
		node.Error = err.Error()
		return err
	}

	return nil
}

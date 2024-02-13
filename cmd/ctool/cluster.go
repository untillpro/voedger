/*
* Copyright (c) 2023-present Sigma-Soft, Ltd.
* @author Dmitry Molchanovsky
 */

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/untillpro/goutils/logger"
)

var dryRun bool

func newCluster() *clusterType {
	var cluster = clusterType{
		DesiredClusterVersion: version,
		ActualClusterVersion:  "",
		exists:                false,
		Draft:                 true,
		sshKey:                sshKey,
		dryRun:                dryRun,
		SshPort:               sshPort,
		Cmd:                   newCmd("", make([]string, 0)),
		SkipStacks:            make([]string, 0),
		ReplacedAddresses:     make([]string, 0),
		Cron:                  &cronType{},
		Acme:                  &acmeType{Domains: make([]string, 0)},
	}

	if len(acmeDomains) != 0 {
		cluster.Acme.Domains = strings.Split(acmeDomains, comma)
	}

	if err := cluster.setEnv(); err != nil {
		loggerError(err.Error())
		return nil
	}

	dir, _ := os.Getwd()

	cluster.configFileName = filepath.Join(dir, clusterConfFileName)

	// Preparation of a configuration file for Dry Run mode
	if cluster.dryRun {

		dryRunDir := filepath.Join(dir, dryRunDir)
		if _, err := os.Stat(dryRunDir); os.IsNotExist(err) {
			err := os.Mkdir(dryRunDir, rwxrwxrwx)
			if err != nil {
				loggerError(err.Error())
				return nil
			}
		}
		dryRunClusterConfigFileName := filepath.Join(dryRunDir, clusterConfFileName)

		// Remove the old dry run configuration file
		// Under tests, you do not need to delete for the possibility of testing command sequences
		if !testing.Testing() {
			if fileExists(dryRunClusterConfigFileName) {
				os.Remove(dryRunClusterConfigFileName)
			}
		}

		if fileExists(cluster.configFileName) {
			if err := copyFile(cluster.configFileName, dryRunClusterConfigFileName); err != nil {
				loggerError(err.Error())
				return nil
			}
		}

		cluster.configFileName = dryRunClusterConfigFileName
	}

	if cluster.clusterConfigFileExists() {
		cluster.exists = true
		if err := cluster.loadFromJSON(); err != nil {
			loggerError(err.Error())
			return nil
		}
	}

	return &cluster
}

func newCmd(cmdKind string, cmdArgs []string) *cmdType {
	return &cmdType{
		Kind: cmdKind,
		Args: cmdArgs,
	}
}

func newNodeState(address string, nodeVersion string) *nodeStateType {
	return &nodeStateType{Address: address, NodeVersion: nodeVersion}
}

type nodeStateType struct {
	Address     string `json:"Address,omitempty"`
	NodeVersion string `json:"NodeVersion,omitempty"`
}

func (n *nodeStateType) clear() {
	n.Address = ""
	n.NodeVersion = ""
}

func (n *nodeStateType) isEmpty() bool {
	return n.Address == "" && n.NodeVersion == ""
}

type nodeType struct {
	cluster          *clusterType
	NodeRole         string
	idx              int            // the sequence number of the node, starts with 1
	Error            string         `json:"Error,omitempty"`
	ActualNodeState  *nodeStateType `json:"ActualNodeState,omitempty"`
	DesiredNodeState *nodeStateType `json:"DesiredNodeState,omitempty"`
}

func (n *nodeType) address() string {
	if n.ActualNodeState != nil && len(n.ActualNodeState.Address) > 0 {
		return n.ActualNodeState.Address
	} else if n.DesiredNodeState != nil && len(n.DesiredNodeState.Address) > 0 {
		return n.DesiredNodeState.Address
	}

	err := fmt.Errorf(errEmptyNodeAddress, n.nodeName(), ErrEmptyNodeAddress)
	loggerError(err.Error)
	panic(err)
}

// nolint
func (n *nodeType) nodeName() string {
	if n.cluster.Edition == clusterEditionSE {
		switch n.idx {
		case 1:
			return "app-node-1"
		case 2:
			return "app-node-2"
		case 3:
			return "db-node-1"
		case 4:
			return "db-node-2"
		case 5:
			return "db-node-3"
		default:
			return "node"
		}
	} else if n.cluster.Edition == clusterEditionCE {
		return "CENode"
	} else {
		return "node"
	}

}

// the minimum amount of RAM required by the node (as string)
// nolint
func (n *nodeType) minAmountOfRAM() string {
	switch n.NodeRole {
	case nrAppNode:
		return minRamOnAppNode
	case nrDBNode:
		return minRamOnDBNode
	default:
		return minRamCENode
	}
}

func (n *nodeType) nodeControllerFunction() error {
	if dryRun {
		if n.DesiredNodeState != nil {
			n.success()
			return nil
		}
	}

	switch n.NodeRole {
	case nrDBNode, nrAppNode:
		return seNodeControllerFunction(n)
	case nrCENode:
		return ceNodeControllerFunction(n)
	default:
		return ErrNodeControllerFunctionNotAssigned
	}
}

func (n *nodeType) success() {
	n.ActualNodeState = newNodeState(n.DesiredNodeState.Address, n.desiredNodeVersion(n.cluster))
	n.DesiredNodeState.clear()
	n.Error = ""
}

// nolint
func (n *nodeType) fail(err string) {
	n.Error = err
}

// initializing a new action attempt on a node
// the error is being reset
// the attempt counter is incremented
func (n *nodeType) newAttempt() {
	n.Error = ""
}

func (n *nodeType) desiredNodeVersion(c *clusterType) string {
	if n.DesiredNodeState != nil && !n.DesiredNodeState.isEmpty() {
		return n.DesiredNodeState.NodeVersion
	}
	return c.DesiredClusterVersion
}

// nolint
func (n *nodeType) actualNodeVersion() string {
	return n.ActualNodeState.NodeVersion
}

func (n *nodeType) label(key string) string {
	switch n.NodeRole {
	case nrCENode:
		return "ce"
	case nrAppNode:
		if key != swarmAppLabelKey {
			return fmt.Sprintf("AppNode%d", n.idx)
		}
		return "AppNode"
	case nrDBNode:
		return fmt.Sprintf("DBNode%d", n.idx-seNodeCount)
	}

	err := fmt.Errorf(errInvalidNodeRole, n.address(), ErrInvalidNodeRole)
	loggerError(err.Error)
	panic(err)
}

// nolint
func (ns *nodeType) check(c *clusterType) error {
	if ns.actualNodeVersion() != ns.desiredNodeVersion(c) {
		return fmt.Errorf(errDifferentNodeVersion, ns.actualNodeVersion(), ns.desiredNodeVersion(c), ErrIncorrectVersion)
	}
	return nil
}

// nolint
type nodesType []*nodeType

// returns a list of node addresses
// you can specify the role of nodes to get addresses
// if role = "", the full list of all cluster nodes will be returned
// nolint
func (n *nodesType) hosts(nodeRole string) []string {
	var h []string
	for _, N := range *n {
		if nodeRole == "" || N.NodeRole == nodeRole {
			h = append(h, N.ActualNodeState.Address)
		}
	}
	return h
}

type cmdArgsType []string

type cmdType struct {
	Kind       string
	Args       cmdArgsType
	SkipStacks []string
}

func (a *cmdArgsType) replace(sourceValue, destValue string) {
	for i, v := range *a {
		if v == sourceValue {
			(*a)[i] = destValue
		}
	}
}

func (c *cmdType) apply(cluster *clusterType) error {

	var err error

	// nolint
	defer cluster.saveToJSON()

	if err = cluster.validate(); err != nil {
		loggerError(err.Error)
		return err
	}

	cluster.Draft = false

	var wg sync.WaitGroup
	wg.Add(len(cluster.Nodes))

	for i := 0; i < len(cluster.Nodes); i++ {
		go func(node *nodeType) {
			defer wg.Done()
			if err := node.nodeControllerFunction(); err != nil {
				loggerError(err.Error)
			}
		}(&cluster.Nodes[i])
	}

	wg.Wait()

	if cluster.existsNodeError() {
		return ErrPreparingClusterNodes
	}

	return cluster.clusterControllerFunction()
}

func (c *cmdType) clear() {
	c.Kind = ""
	c.Args = []string{}
}

func (c *cmdType) isEmpty() bool {

	return c.Kind == "" && len(c.Args) == 0
}

func (c *cmdType) validate(cluster *clusterType) error {
	switch c.Kind {
	case ckInit:
		return validateInitCmd(c, cluster)
	case ckUpgrade:
		return validateUpgradeCmd(c, cluster)
	case ckReplace:
		return validateReplaceCmd(c, cluster)
	case ckBackup:
		return validateBackupCmd(c, cluster)
	case ckAcme:
		return validateAcmeCmd(c, cluster)
	default:
		return ErrUnknownCommand
	}
}

// init [CE] [ipAddr1]
// or
// init [SE] [ipAddr1] [ipAddr2] [ipAddr3] [ipAddr4] [ipAddr5]
// nolint
func validateInitCmd(cmd *cmdType, cluster *clusterType) error {

	if len(cmd.Args) == 0 {
		return ErrMissingCommandArguments
	}

	if cmd.Args[0] != clusterEditionCE && cmd.Args[0] != clusterEditionSE {
		return ErrInvalidClusterEdition
	}
	if cmd.Args[0] == clusterEditionCE && len(cmd.Args) != 1+initCeArgCount {
		return ErrInvalidNumberOfArguments
	}

	return nil
}

// update [desiredVersion]
func validateUpgradeCmd(cmd *cmdType, cluster *clusterType) error {
	return nil
}

func validateReplaceCmd(cmd *cmdType, cluster *clusterType) error {

	if len(cmd.Args) == 0 {
		return ErrMissingCommandArguments
	}

	if len(cmd.Args) != 2 {
		return ErrInvalidNumberOfArguments
	}

	var err error

	if n := cluster.nodeByHost(cmd.Args[0]); n == nil {
		err = errors.Join(err, fmt.Errorf(errHostNotFoundInCluster, cmd.Args[0], ErrHostNotFoundInCluster))
	}

	if n := cluster.nodeByHost(cmd.Args[1]); n != nil {
		err = errors.Join(err, fmt.Errorf(ErrHostAlreadyExistsInCluster.Error(), cmd.Args[1]))
	}

	return err
}

func validateBackupCmd(cmd *cmdType, cluster *clusterType) error {
	if len(cmd.Args) == 0 {
		return ErrMissingCommandArguments
	}

	if cluster.Draft {
		return ErrClusterConfNotFound
	}

	if len(cmd.Args) <= 1 {
		return ErrMissingCommandArguments
	}

	switch cmd.Args[0] {
	case "node":
		return validateBackupNodeCmd(cmd, cluster)
	case "cron":
		return validateBackupCronCmd(cmd, cluster)
	default:
		return ErrUnknownCommand
	}
}

func validateAcmeCmd(cmd *cmdType, cluster *clusterType) error {

	if len(cmd.Args) == 0 {
		return ErrMissingCommandArguments
	}

	if cluster.Draft {
		return ErrClusterConfNotFound
	}

	switch cmd.Args[0] {
	case "add":
		return validateAcmeAddCmd(cmd, cluster)
	case "remove":
		return validateAcmeRemoveCmd(cmd, cluster)
	default:
		return ErrUnknownCommand
	}

}

func validateAcmeAddCmd(cmd *cmdType, cluster *clusterType) error {

	if cluster.Draft {
		return ErrClusterConfNotFound
	}

	if len(cmd.Args) != 2 {
		return ErrInvalidNumberOfArguments
	}

	return nil
}

func validateAcmeRemoveCmd(cmd *cmdType, cluster *clusterType) error {

	if cluster.Draft {
		return ErrClusterConfNotFound
	}

	if len(cmd.Args) != 2 {
		return ErrInvalidNumberOfArguments
	}

	domains := strings.Split(cmd.Args[1], comma)
	domainsMap := make(map[string]bool)
	for _, s := range cluster.Acme.Domains {
		domainsMap[s] = true
	}

	var notFound []string
	for _, s := range domains {
		if !domainsMap[s] {
			notFound = append(notFound, s)
		}
	}
	if len(notFound) > 0 {
		return fmt.Errorf(errDomainsNotFound, strings.Join(notFound, comma), ErrDomainsNotFound)
	}

	return nil
}

type cronType struct {
	Backup string `json:"Backup,omitempty"`
}

type acmeType struct {
	Domains []string `json:"Domains,omitempty"`
}

func (a *acmeType) domains() string {
	return strings.Join(a.Domains, comma)
}

// adds new domains to the ACME Domains list from a string "Domain1,Domain2,Domain3"
func (a *acmeType) addDomains(domainsStr string) {
	domains := strings.Split(domainsStr, comma)
	for _, d := range domains {
		if !strings.Contains(strings.Join(a.Domains, comma), d) {
			a.Domains = append(a.Domains, d)
		}
	}
}

// removes domains from the ACME Domains list from a string "Domain1,Domain2,Domain3"
func (a *acmeType) removeDomains(domainsStr string) {
	domains := strings.Split(domainsStr, comma)
	for _, d := range domains {
		for i, v := range a.Domains {
			if v == d {
				a.Domains = append(a.Domains[:i], a.Domains[i+1:]...)
			}
		}
	}
}

type clusterType struct {
	configFileName        string
	sshKey                string
	exists                bool //the cluster is loaded from "cluster.json" at the start of ctool
	dryRun                bool
	Edition               string
	ActualClusterVersion  string
	DesiredClusterVersion string    `json:"DesiredClusterVersion,omitempty"`
	SshPort               string    `json:"SSHPort,omitempty"`
	Acme                  *acmeType `json:"Acme,omitempty"`
	Cmd                   *cmdType  `json:"Cmd,omitempty"`
	LastAttemptError      string    `json:"LastAttemptError,omitempty"`
	SkipStacks            []string  `json:"SkipStacks,omitempty"`
	Cron                  *cronType `json:"Cron,omitempty"`
	Nodes                 []nodeType
	ReplacedAddresses     []string `json:"ReplacedAddresses,omitempty"`
	Draft                 bool     `json:"Draft,omitempty"`
}

func (c *clusterType) clusterControllerFunction() error {
	if dryRun {
		c.success()
		return nil
	}

	switch c.Edition {
	case clusterEditionCE:
		return ceClusterControllerFunction(c)
	case clusterEditionSE:
		return seClusterControllerFunction(c)
	default:
		return ErrClusterControllerFunctionNotAssigned
	}
}

func prettyprint(b []byte) ([]byte, error) {
	var out bytes.Buffer
	err := json.Indent(&out, b, "", "  ")

	return out.Bytes(), err
}

func equalIPs(ip1, ip2 string) bool {
	netIP1 := net.ParseIP(ip1)
	netIP2 := net.ParseIP(ip2)

	if netIP1 == nil || netIP2 == nil {
		return false
	}

	return netIP1.Equal(netIP2)
}

func (c *clusterType) nodeByHost(addrOrHostName string) *nodeType {
	for i, n := range c.Nodes {
		if addrOrHostName == n.nodeName() || equalIPs(n.ActualNodeState.Address, addrOrHostName) {
			return &c.Nodes[i]
		}
	}
	return nil
}

func (c *clusterType) applyCmd(cmd *cmdType) error {
	if err := cmd.validate(c); err != nil {
		return err
	}

	if !c.Draft && c != nil && !c.Cmd.isEmpty() {
		return ErrUncompletedCommandFound
	}

	// nolint
	defer c.saveToJSON()

	switch cmd.Kind {
	case ckAcme:
		if cmd.Args[0] == "add" && len(cmd.Args) == 2 {
			c.Acme.addDomains(cmd.Args[1])
			if err := c.setEnv(); err != nil {
				return err
			}
		} else if cmd.Args[0] == "remove" && len(cmd.Args) == 2 {
			c.Acme.removeDomains(cmd.Args[1])
			if err := c.setEnv(); err != nil {
				return err
			}
		}
	case ckReplace:
		oldAddr := cmd.Args[0]
		newAddr := cmd.Args[1]

		if c.addressInReplacedList(newAddr) {
			return fmt.Errorf(errAddressInReplacedList, newAddr, ErrAddressCannotBeUsed)
		}

		node := c.nodeByHost(oldAddr)
		if node == nil {
			return fmt.Errorf(errHostNotFoundInCluster, oldAddr, ErrHostNotFoundInCluster)
		}

		if oldAddr == node.nodeName() {
			oldAddr = node.ActualNodeState.Address
			cmd.Args.replace(node.nodeName(), oldAddr)
		}

		if !dryRun {
			if err := nodeIsDown(node); err != nil {
				return fmt.Errorf(errCannotReplaceALiveNode, oldAddr, ErrCommandCannotBeExecuted)
			}

			if err := hostIsAvailable(c, newAddr); err != nil {
				return fmt.Errorf(errHostIsNotAvailable, newAddr, ErrHostIsNotAvailable)
			}

			if len(c.Cron.Backup) > 0 && node.NodeRole == nrDBNode {
				if err := checkBackupFolderOnHost(c, newAddr); err != nil {
					return err
				}
			}
		}

		node.DesiredNodeState = newNodeState(newAddr, node.desiredNodeVersion(c))

		if node.ActualNodeState != nil {
			node.ActualNodeState.clear()
		}
	case ckUpgrade:
		c.DesiredClusterVersion = version
		for i := range c.Nodes {
			c.Nodes[i].DesiredNodeState.NodeVersion = version
			c.Nodes[i].DesiredNodeState.Address = c.Nodes[i].ActualNodeState.Address
		}
	}

	c.Cmd = cmd

	return nil
}

func (c *clusterType) updateNodeIndexes() {
	for i := range c.Nodes {
		c.Nodes[i].idx = i + 1
	}
}

// TODO: Filename should be an argument
func (c *clusterType) saveToJSON() error {

	if c.Cmd != nil && c.Cmd.isEmpty() {
		c.Cmd = nil
	}
	for i := 0; i < len(c.Nodes); i++ {
		if c.Nodes[i].DesiredNodeState != nil && c.Nodes[i].DesiredNodeState.isEmpty() {
			c.Nodes[i].DesiredNodeState = nil
		}
		if c.Nodes[i].ActualNodeState != nil && c.Nodes[i].ActualNodeState.isEmpty() {
			c.Nodes[i].ActualNodeState = nil
		}
	}

	b, err := json.Marshal(c)
	if err == nil {
		b, err = prettyprint(b)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(c.configFileName, b, rwxrwxrwx)
	}
	return err
}

// The address was replaced in the cluster
func (c *clusterType) addressInReplacedList(address string) bool {
	for _, value := range c.ReplacedAddresses {
		if value == address {
			return true
		}
	}
	return false
}

func (c *clusterType) clusterConfigFileExists() bool {
	_, err := os.Stat(c.configFileName)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func (c *clusterType) loadFromJSON() error {

	defer c.updateNodeIndexes()
	defer func() {
		if c.Cmd == nil {
			c.Cmd = newCmd("", []string{})
		}
		for i := 0; i < len(c.Nodes); i++ {
			if c.Nodes[i].ActualNodeState == nil {
				c.Nodes[i].ActualNodeState = newNodeState("", "")
			}
			if c.Nodes[i].DesiredNodeState == nil {
				c.Nodes[i].DesiredNodeState = newNodeState("", "")
			}
		}
	}()

	if !c.clusterConfigFileExists() {
		return ErrClusterConfNotFound
	}

	b, err := os.ReadFile(c.configFileName)
	if err == nil {
		oldDraft := c.Draft
		c.Draft = false
		err = json.Unmarshal(b, c)
		if err != nil {
			c.Draft = oldDraft
		}
	}

	for i := 0; i < len(c.Nodes); i++ {
		c.Nodes[i].cluster = c
	}

	if err == nil {
		err = c.setEnv()
	}

	return err
}

// Installation of the necessary variables of the environment
func (c *clusterType) setEnv() error {

	logger.Verbose(fmt.Sprintf("Set env VOEDGER_NODE_SSH_PORT = %s", c.SshPort))
	if err := os.Setenv("VOEDGER_NODE_SSH_PORT", c.SshPort); err != nil {
		return err
	}

	logger.Verbose(fmt.Sprintf("Set env VOEDGER_ACME_DOMAINS = %s", c.Acme.domains()))
	if err := os.Setenv("VOEDGER_ACME_DOMAINS", c.Acme.domains()); err != nil {
		return err
	}

	return nil
}

// nolint
func (c *clusterType) readFromInitArgs(cmd *cobra.Command, args []string) error {

	defer c.updateNodeIndexes()
	// nolint
	defer c.saveToJSON()

	skipStacks, err := cmd.Flags().GetStringSlice("skip-stack")
	if err != nil {
		fmt.Println("Error getting skip-stack values:", err)
		return err
	}
	c.SkipStacks = skipStacks

	if cmd == initCECmd { // CE args
		c.Edition = clusterEditionCE
		c.Nodes = make([]nodeType, 1)
		c.Nodes[0].NodeRole = nrCENode
		c.Nodes[0].cluster = c
		c.Nodes[0].DesiredNodeState = newNodeState("", c.DesiredClusterVersion)
		c.Nodes[0].ActualNodeState = newNodeState("", "")
		if len(args) > 0 {
			c.Nodes[0].DesiredNodeState.Address = args[0]
		} else {
			c.Nodes[0].DesiredNodeState.Address = "0.0.0.0"
		}
	} else { // SE args
		c.Edition = clusterEditionSE
		c.Nodes = make([]nodeType, 5)

		for i := 0; i < initSeArgCount; i++ {
			if i < seNodeCount {
				c.Nodes[i].NodeRole = nrAppNode
			} else {
				c.Nodes[i].NodeRole = nrDBNode
			}
			c.Nodes[i].DesiredNodeState = newNodeState(args[i], c.DesiredClusterVersion)
			c.Nodes[i].ActualNodeState = newNodeState("", "")
			c.Nodes[i].cluster = c
		}

	}
	return nil
}

// nolint
func (c *clusterType) validate() error {

	var err error

	for _, n := range c.Nodes {
		if n.DesiredNodeState != nil && len(n.DesiredNodeState.Address) > 0 && net.ParseIP(n.DesiredNodeState.Address) == nil {
			err = errors.Join(err, errors.New(n.DesiredNodeState.Address+" "+ErrInvalidIpAddress.Error()))
		}
		if n.ActualNodeState != nil && len(n.ActualNodeState.Address) > 0 && net.ParseIP(n.ActualNodeState.Address) == nil {
			err = errors.Join(err, errors.New(n.ActualNodeState.Address+" "+ErrInvalidIpAddress.Error()))
		}
	}

	if c.Edition != clusterEditionCE && c.Edition != clusterEditionSE {
		err = errors.Join(err, ErrInvalidClusterEdition)
	}

	return err
}

func (c *clusterType) success() {
	c.ActualClusterVersion = c.DesiredClusterVersion
	c.DesiredClusterVersion = ""
	if c.Cmd != nil {
		c.Cmd.clear()
	}
	c.LastAttemptError = ""
}

// nolint
func (c *clusterType) fail(error string) {
	c.LastAttemptError = error
}

// nolint
func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := user.Current()
		if err != nil {
			return "", err
		}

		path = filepath.Join(homeDir.HomeDir, path[2:])
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return absPath, nil
}

func (c *clusterType) existsNodeError() bool {
	for _, n := range c.Nodes {
		if len(n.Error) > 0 {
			return true
		}
	}
	return false
}

func (c *clusterType) checkVersion() error {

	loggerInfo("Ctool version: ", version)

	var clusterVersion string

	if c.clusterConfigFileExists() && !c.Cmd.isEmpty() {
		clusterVersion = c.DesiredClusterVersion
	}

	if len(clusterVersion) == 0 {
		clusterVersion = c.ActualClusterVersion
	}

	// The cluster configuration is still missing
	if clusterVersion == "" {
		loggerInfo("Cluster version is missing")
		return nil
	}

	loggerInfo("Cluster version: ", clusterVersion)

	vr := compareVersions(version, clusterVersion)
	if vr == 1 {
		return fmt.Errorf(errCtoolVersionNewerThanClusterVersion, version, clusterVersion, ErrIncorrectVersion)
	} else if vr == -1 {
		return fmt.Errorf(errClusterVersionNewerThanCtoolVersion, clusterVersion, version, clusterVersion, ErrIncorrectVersion)
	}

	return nil
}

func (c *clusterType) needUpgrade() (bool, error) {
	vr := compareVersions(version, c.ActualClusterVersion)
	if vr == -1 {
		return false, fmt.Errorf(errClusterVersionNewerThanCtoolVersion, c.ActualClusterVersion, version, c.ActualClusterVersion, ErrIncorrectVersion)
	} else if vr == 1 {
		return true, nil
	}

	return false, nil

}

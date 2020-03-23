/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package iptables

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/sets"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	utilversion "k8s.io/kubernetes/pkg/util/version"
	utilexec "k8s.io/utils/exec"
)

type RulePosition string

const (
	Prepend RulePosition = "-I"
	Append  RulePosition = "-A"
)

// An injectable interface for running iptables commands.  Implementations must be goroutine-safe.
type Interface interface {
	// EnsureChain checks if the specified chain exists and, if not, creates it.  If the chain existed, return true.
	EnsureChain(table Table, chain Chain) (bool, error)
	// FlushChain clears the specified chain.  If the chain did not exist, return error.
	FlushChain(table Table, chain Chain) error
	// DeleteChain deletes the specified chain.  If the chain did not exist, return error.
	DeleteChain(table Table, chain Chain) error
	// EnsureRule checks if the specified rule is present and, if not, creates it.  If the rule existed, return true.
	EnsureRule(position RulePosition, table Table, chain Chain, args ...string) (bool, error)
	// DeleteRule checks if the specified rule is present and, if so, deletes it.
	DeleteRule(table Table, chain Chain, args ...string) error
	// IsIpv6 returns true if this is managing ipv6 tables
	IsIpv6() bool
	// SaveInto calls `iptables-save` for table and stores result in a given buffer.
	SaveInto(table Table, buffer *bytes.Buffer) error
	// Restore runs `iptables-restore` passing data through []byte.
	// table is the Table to restore
	// data should be formatted like the output of SaveInto()
	// flush sets the presence of the "--noflush" flag. see: FlushFlag
	// counters sets the "--counters" flag. see: RestoreCountersFlag
	Restore(table Table, data []byte, flush FlushFlag, counters RestoreCountersFlag) error
	// RestoreAll is the same as Restore except that no table is specified.
	RestoreAll(data []byte, flush FlushFlag, counters RestoreCountersFlag) error
	// Monitor detects when the given iptables tables have been flushed by an external
	// tool (e.g. a firewall reload) by creating canary chains and polling to see if
	// they have been deleted. (Specifically, it polls tables[0] every interval until
	// the canary has been deleted from there, then waits a short additional time for
	// the canaries to be deleted from the remaining tables as well. You can optimize
	// the polling by listing a relatively empty table in tables[0]). When a flush is
	// detected, this calls the reloadFunc so the caller can reload their own iptables
	// rules. If it is unable to create the canary chains (either initially or after
	// a reload) it will log an error and stop monitoring.
	// (This function should be called from a goroutine.)
	Monitor(canary Chain, tables []Table, reloadFunc func(), interval time.Duration, stopCh <-chan struct{})
}

type Protocol byte

const (
	ProtocolIpv4 Protocol = iota + 1
	ProtocolIpv6
)

type Table string

const (
	TableNAT    Table = "nat"
	TableFilter Table = "filter"
	TableMangle Table = "mangle"
)

type Chain string

const (
	ChainPostrouting Chain = "POSTROUTING"
	ChainPrerouting  Chain = "PREROUTING"
	ChainOutput      Chain = "OUTPUT"
	ChainInput       Chain = "INPUT"
	ChainForward     Chain = "FORWARD"
)

const (
	cmdIPTablesSave     string = "iptables-save"
	cmdIPTablesRestore  string = "iptables-restore"
	cmdIPTables         string = "iptables"
	cmdIP6TablesRestore string = "ip6tables-restore"
	cmdIP6TablesSave    string = "ip6tables-save"
	cmdIP6Tables        string = "ip6tables"
)

// Option flag for Restore
type RestoreCountersFlag bool

const RestoreCounters RestoreCountersFlag = true
const NoRestoreCounters RestoreCountersFlag = false

// Option flag for Flush
type FlushFlag bool

const FlushTables FlushFlag = true
const NoFlushTables FlushFlag = false

// Versions of iptables less than this do not support the -C / --check flag
// (test whether a rule exists).
var MinCheckVersion = utilversion.MustParseGeneric("1.4.11")

// Minimum iptables versions supporting the -w and -w<seconds> flags
var WaitMinVersion = utilversion.MustParseGeneric("1.4.20")
var WaitSecondsMinVersion = utilversion.MustParseGeneric("1.4.22")
var WaitRestoreMinVersion = utilversion.MustParseGeneric("1.6.2")

const WaitString = "-w"
const WaitSecondsValue = "5"

const LockfilePath16x = "/run/xtables.lock"

// runner implements Interface in terms of exec("iptables").
type runner struct {
	mu              sync.Mutex
	exec            utilexec.Interface
	protocol        Protocol
	hasCheck        bool
	hasListener     bool
	waitFlag        []string
	restoreWaitFlag []string
	lockfilePath    string

	reloadFuncs []func()
}

// newInternal returns a new Interface which will exec iptables, and allows the
// caller to change the iptables-restore lockfile path
func newInternal(exec utilexec.Interface, protocol Protocol, lockfilePath string) Interface {
	version, err := getIPTablesVersion(exec, protocol)
	if err != nil {
		glog.Warningf("Error checking iptables version, assuming version at least %s: %v", MinCheckVersion, err)
		version = MinCheckVersion
	}

	if lockfilePath == "" {
		lockfilePath = LockfilePath16x
	}

	runner := &runner{
		exec:            exec,
		protocol:        protocol,
		hasCheck:        version.AtLeast(MinCheckVersion),
		hasListener:     false,
		waitFlag:        getIPTablesWaitFlag(version),
		restoreWaitFlag: getIPTablesRestoreWaitFlag(version, exec, protocol),
		lockfilePath:    lockfilePath,
	}
	return runner
}

// New returns a new Interface which will exec iptables.
func New(exec utilexec.Interface, protocol Protocol) Interface {
	return newInternal(exec, protocol, "")
}

// EnsureChain is part of Interface.
func (runner *runner) EnsureChain(table Table, chain Chain) (bool, error) {
	fullArgs := makeFullArgs(table, chain)

	runner.mu.Lock()
	defer runner.mu.Unlock()

	out, err := runner.run(opCreateChain, fullArgs)
	if err != nil {
		if ee, ok := err.(utilexec.ExitError); ok {
			if ee.Exited() && ee.ExitStatus() == 1 {
				return true, nil
			}
		}
		return false, fmt.Errorf("error creating chain %q: %v: %s", chain, err, out)
	}
	return false, nil
}

// FlushChain is part of Interface.
func (runner *runner) FlushChain(table Table, chain Chain) error {
	fullArgs := makeFullArgs(table, chain)

	runner.mu.Lock()
	defer runner.mu.Unlock()

	out, err := runner.run(opFlushChain, fullArgs)
	if err != nil {
		return fmt.Errorf("error flushing chain %q: %v: %s", chain, err, out)
	}
	return nil
}

// DeleteChain is part of Interface.
func (runner *runner) DeleteChain(table Table, chain Chain) error {
	fullArgs := makeFullArgs(table, chain)

	runner.mu.Lock()
	defer runner.mu.Unlock()

	// TODO: we could call iptables -S first, ignore the output and check for non-zero return (more like DeleteRule)
	out, err := runner.run(opDeleteChain, fullArgs)
	if err != nil {
		return fmt.Errorf("error deleting chain %q: %v: %s", chain, err, out)
	}
	return nil
}

// EnsureRule is part of Interface.
func (runner *runner) EnsureRule(position RulePosition, table Table, chain Chain, args ...string) (bool, error) {
	fullArgs := makeFullArgs(table, chain, args...)

	runner.mu.Lock()
	defer runner.mu.Unlock()

	exists, err := runner.checkRule(table, chain, args...)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}
	out, err := runner.run(operation(position), fullArgs)
	if err != nil {
		return false, fmt.Errorf("error appending rule: %v: %s", err, out)
	}
	return false, nil
}

// DeleteRule is part of Interface.
func (runner *runner) DeleteRule(table Table, chain Chain, args ...string) error {
	fullArgs := makeFullArgs(table, chain, args...)

	runner.mu.Lock()
	defer runner.mu.Unlock()

	exists, err := runner.checkRule(table, chain, args...)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	out, err := runner.run(opDeleteRule, fullArgs)
	if err != nil {
		return fmt.Errorf("error deleting rule: %v: %s", err, out)
	}
	return nil
}

func (runner *runner) IsIpv6() bool {
	return runner.protocol == ProtocolIpv6
}

// SaveInto is part of Interface.
func (runner *runner) SaveInto(table Table, buffer *bytes.Buffer) error {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	// run and return
	iptablesSaveCmd := iptablesSaveCommand(runner.protocol)
	args := []string{"-t", string(table)}
	glog.V(4).Infof("running %s %v", iptablesSaveCmd, args)
	cmd := runner.exec.Command(iptablesSaveCmd, args...)
	// Since CombinedOutput() doesn't support redirecting it to a buffer,
	// we need to workaround it by redirecting stdout and stderr to buffer
	// and explicitly calling Run() [CombinedOutput() underneath itself
	// creates a new buffer, redirects stdout and stderr to it and also
	// calls Run()].
	cmd.SetStdout(buffer)
	cmd.SetStderr(buffer)
	return cmd.Run()
}

// Restore is part of Interface.
func (runner *runner) Restore(table Table, data []byte, flush FlushFlag, counters RestoreCountersFlag) error {
	// setup args
	args := []string{"-T", string(table)}
	return runner.restoreInternal(args, data, flush, counters)
}

// RestoreAll is part of Interface.
func (runner *runner) RestoreAll(data []byte, flush FlushFlag, counters RestoreCountersFlag) error {
	// setup args
	args := make([]string, 0)
	return runner.restoreInternal(args, data, flush, counters)
}

type iptablesLocker interface {
	Close() error
}

// restoreInternal is the shared part of Restore/RestoreAll
func (runner *runner) restoreInternal(args []string, data []byte, flush FlushFlag, counters RestoreCountersFlag) error {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	if !flush {
		args = append(args, "--noflush")
	}
	if counters {
		args = append(args, "--counters")
	}

	// Grab the iptables lock to prevent iptables-restore and iptables
	// from stepping on each other.  iptables-restore 1.6.2 will have
	// a --wait option like iptables itself, but that's not widely deployed.
	if len(runner.restoreWaitFlag) == 0 {
		locker, err := grabIptablesLocks(runner.lockfilePath)
		if err != nil {
			return err
		}
		defer func(locker iptablesLocker) {
			if err := locker.Close(); err != nil {
				glog.Errorf("Failed to close iptables locks: %v", err)
			}
		}(locker)
	}

	// run the command and return the output or an error including the output and error
	fullArgs := append(runner.restoreWaitFlag, args...)
	iptablesRestoreCmd := iptablesRestoreCommand(runner.protocol)
	glog.V(4).Infof("running %s %v", iptablesRestoreCmd, fullArgs)
	cmd := runner.exec.Command(iptablesRestoreCmd, fullArgs...)
	cmd.SetStdin(bytes.NewBuffer(data))
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v (%s)", err, b)
	}
	return nil
}

func iptablesSaveCommand(protocol Protocol) string {
	if protocol == ProtocolIpv6 {
		return cmdIP6TablesSave
	} else {
		return cmdIPTablesSave
	}
}

func iptablesRestoreCommand(protocol Protocol) string {
	if protocol == ProtocolIpv6 {
		return cmdIP6TablesRestore
	} else {
		return cmdIPTablesRestore
	}
}

func iptablesCommand(protocol Protocol) string {
	if protocol == ProtocolIpv6 {
		return cmdIP6Tables
	} else {
		return cmdIPTables
	}
}

func (runner *runner) run(op operation, args []string) ([]byte, error) {
	return runner.runContext(nil, op, args)
}

func (runner *runner) runContext(ctx context.Context, op operation, args []string) ([]byte, error) {
	iptablesCmd := iptablesCommand(runner.protocol)
	fullArgs := append(runner.waitFlag, string(op))
	fullArgs = append(fullArgs, args...)
	glog.V(5).Infof("running iptables %s %v", string(op), args)
	if ctx == nil {
		return runner.exec.Command(iptablesCmd, fullArgs...).CombinedOutput()
	}
	return runner.exec.CommandContext(ctx, iptablesCmd, fullArgs...).CombinedOutput()
	// Don't log err here - callers might not think it is an error.
}

// Returns (bool, nil) if it was able to check the existence of the rule, or
// (<undefined>, error) if the process of checking failed.
func (runner *runner) checkRule(table Table, chain Chain, args ...string) (bool, error) {
	if runner.hasCheck {
		return runner.checkRuleUsingCheck(makeFullArgs(table, chain, args...))
	}
	return runner.checkRuleWithoutCheck(table, chain, args...)
}

var hexnumRE = regexp.MustCompile("0x0+([0-9])")

func trimhex(s string) string {
	return hexnumRE.ReplaceAllString(s, "0x$1")
}

// Executes the rule check without using the "-C" flag, instead parsing iptables-save.
// Present for compatibility with <1.4.11 versions of iptables.  This is full
// of hack and half-measures.  We should nix this ASAP.
func (runner *runner) checkRuleWithoutCheck(table Table, chain Chain, args ...string) (bool, error) {
	iptablesSaveCmd := iptablesSaveCommand(runner.protocol)
	glog.V(1).Infof("running %s -t %s", iptablesSaveCmd, string(table))
	out, err := runner.exec.Command(iptablesSaveCmd, "-t", string(table)).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("error checking rule: %v", err)
	}

	// Sadly, iptables has inconsistent quoting rules for comments. Just remove all quotes.
	// Also, quoted multi-word comments (which are counted as a single arg)
	// will be unpacked into multiple args,
	// in order to compare against iptables-save output (which will be split at whitespace boundary)
	// e.g. a single arg('"this must be before the NodePort rules"') will be unquoted and unpacked into 7 args.
	var argsCopy []string
	for i := range args {
		tmpField := strings.Trim(args[i], "\"")
		tmpField = trimhex(tmpField)
		argsCopy = append(argsCopy, strings.Fields(tmpField)...)
	}
	argset := sets.NewString(argsCopy...)

	for _, line := range strings.Split(string(out), "\n") {
		var fields = strings.Fields(line)

		// Check that this is a rule for the correct chain, and that it has
		// the correct number of argument (+2 for "-A <chain name>")
		if !strings.HasPrefix(line, fmt.Sprintf("-A %s", string(chain))) || len(fields) != len(argsCopy)+2 {
			continue
		}

		// Sadly, iptables has inconsistent quoting rules for comments.
		// Just remove all quotes.
		for i := range fields {
			fields[i] = strings.Trim(fields[i], "\"")
			fields[i] = trimhex(fields[i])
		}

		// TODO: This misses reorderings e.g. "-x foo ! -y bar" will match "! -x foo -y bar"
		if sets.NewString(fields...).IsSuperset(argset) {
			return true, nil
		}
		glog.V(5).Infof("DBG: fields is not a superset of args: fields=%v  args=%v", fields, args)
	}

	return false, nil
}

// Executes the rule check using the "-C" flag
func (runner *runner) checkRuleUsingCheck(args []string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	out, err := runner.runContext(ctx, opCheckRule, args)
	if ctx.Err() == context.DeadlineExceeded {
		return false, fmt.Errorf("timed out while checking rules")
	}
	if err == nil {
		return true, nil
	}
	if ee, ok := err.(utilexec.ExitError); ok {
		// iptables uses exit(1) to indicate a failure of the operation,
		// as compared to a malformed commandline, for example.
		if ee.Exited() && ee.ExitStatus() == 1 {
			return false, nil
		}
	}
	return false, fmt.Errorf("error checking rule: %v: %s", err, out)
}

const (
	// Max time we wait for an iptables flush to complete after we notice it has started
	iptablesFlushTimeout = 5 * time.Second
	// How often we poll while waiting for an iptables flush to complete
	iptablesFlushPollTime = 100 * time.Millisecond
)

// Monitor is part of Interface
func (runner *runner) Monitor(canary Chain, tables []Table, reloadFunc func(), interval time.Duration, stopCh <-chan struct{}) {
	for {
		for _, table := range tables {
			if _, err := runner.EnsureChain(table, canary); err != nil {
				glog.Warningf("Could not set up iptables canary %s/%s: %v", string(table), string(canary), err)
				return
			}
		}

		// Poll until stopCh is closed or iptables is flushed
		err := utilwait.PollUntil(interval, func() (bool, error) {
			if runner.chainExists(tables[0], canary) {
				return false, nil
			}
			glog.V(2).Infof("iptables canary %s/%s deleted", string(tables[0]), string(canary))

			// Wait for the other canaries to be deleted too before returning
			// so we don't start reloading too soon.
			err := utilwait.PollImmediate(iptablesFlushPollTime, iptablesFlushTimeout, func() (bool, error) {
				for i := 1; i < len(tables); i++ {
					if runner.chainExists(tables[i], canary) {
						return false, nil
					}
				}
				return true, nil
			})
			if err != nil {
				glog.Warning("Inconsistent iptables state detected.")
			}
			return true, nil
		}, stopCh)

		if err != nil {
			// stopCh was closed
			for _, table := range tables {
				_ = runner.DeleteChain(table, canary)
			}
			return
		}

		glog.V(2).Infof("Reloading after iptables flush")
		reloadFunc()
	}
}

// chainExists is used internally by Monitor; none of the public Interface methods can be
// used to distinguish "chain exists" from "chain does not exist" with no side effects
func (runner *runner) chainExists(table Table, chain Chain) bool {
	fullArgs := makeFullArgs(table, chain)

	runner.mu.Lock()
	defer runner.mu.Unlock()

	_, err := runner.run(opListChain, fullArgs)
	return err == nil
}

type operation string

const (
	opCreateChain operation = "-N"
	opFlushChain  operation = "-F"
	opDeleteChain operation = "-X"
	opListChain   operation = "-L"
	opAppendRule  operation = "-A"
	opCheckRule   operation = "-C"
	opDeleteRule  operation = "-D"
)

func makeFullArgs(table Table, chain Chain, args ...string) []string {
	return append([]string{string(chain), "-t", string(table)}, args...)
}

// getIPTablesVersion runs "iptables --version" and parses the returned version
func getIPTablesVersion(exec utilexec.Interface, protocol Protocol) (*utilversion.Version, error) {
	// this doesn't access mutable state so we don't need to use the interface / runner
	iptablesCmd := iptablesCommand(protocol)
	bytes, err := exec.Command(iptablesCmd, "--version").CombinedOutput()
	if err != nil {
		return nil, err
	}
	versionMatcher := regexp.MustCompile("v([0-9]+(\\.[0-9]+)+)")
	match := versionMatcher.FindStringSubmatch(string(bytes))
	if match == nil {
		return nil, fmt.Errorf("no iptables version found in string: %s", bytes)
	}
	version, err := utilversion.ParseGeneric(match[1])
	if err != nil {
		return nil, fmt.Errorf("iptables version %q is not a valid version string: %v", match[1], err)
	}
	return version, nil
}

// Checks if iptables version has a "wait" flag
func getIPTablesWaitFlag(version *utilversion.Version) []string {
	switch {
	case version.AtLeast(WaitSecondsMinVersion):
		return []string{WaitString, WaitSecondsValue}
	case version.AtLeast(WaitMinVersion):
		return []string{WaitString}
	default:
		return nil
	}
}

// Checks if iptables-restore has a "wait" flag
func getIPTablesRestoreWaitFlag(version *utilversion.Version, exec utilexec.Interface, protocol Protocol) []string {
	if version.AtLeast(WaitRestoreMinVersion) {
		return []string{WaitString, WaitSecondsValue}
	}

	// Older versions may have backported features; if iptables-restore supports
	// --version, assume it also supports --wait
	vstring, err := getIPTablesRestoreVersionString(exec, protocol)
	if err != nil || vstring == "" {
		glog.V(3).Infof("couldn't get iptables-restore version; assuming it doesn't support --wait")
		return nil
	}
	if _, err := utilversion.ParseGeneric(vstring); err != nil {
		glog.V(3).Infof("couldn't parse iptables-restore version; assuming it doesn't support --wait")
		return nil
	}
	return []string{WaitString}
}

// getIPTablesRestoreVersionString runs "iptables-restore --version" to get the version string
// in the form "X.X.X"
func getIPTablesRestoreVersionString(exec utilexec.Interface, protocol Protocol) (string, error) {
	// this doesn't access mutable state so we don't need to use the interface / runner

	// iptables-restore hasn't always had --version, and worse complains
	// about unrecognized commands but doesn't exit when it gets them.
	// Work around that by setting stdin to nothing so it exits immediately.
	iptablesRestoreCmd := iptablesRestoreCommand(protocol)
	cmd := exec.Command(iptablesRestoreCmd, "--version")
	cmd.SetStdin(bytes.NewReader([]byte{}))
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	versionMatcher := regexp.MustCompile("v([0-9]+(\\.[0-9]+)+)")
	match := versionMatcher.FindStringSubmatch(string(bytes))
	if match == nil {
		return "", fmt.Errorf("no iptables version found in string: %s", bytes)
	}
	return match[1], nil
}

// IsNotFoundError returns true if the error indicates "not found".  It parses
// the error string looking for known values, which is imperfect but works in
// practice.
func IsNotFoundError(err error) bool {
	es := err.Error()
	if strings.Contains(es, "No such file or directory") {
		return true
	}
	if strings.Contains(es, "No chain/target/match by that name") {
		return true
	}
	return false
}

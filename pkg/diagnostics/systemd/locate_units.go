package systemd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

func GetSystemdUnits(logger *log.Logger) map[string]types.SystemdUnit {
	systemdUnits := map[string]types.SystemdUnit{}

	logger.Notice("discBeginSysd", "Performing systemd discovery")
	for _, name := range []string{"openshift", "openshift-master", "openshift-node", "openshift-sdn-master", "openshift-sdn-node", "docker", "openvswitch", "iptables", "etcd", "kubernetes"} {
		systemdUnits[name] = discoverSystemdUnit(logger, name)

		if systemdUnits[name].Exists {
			logger.Debugf("discUnit", "Saw systemd unit %s", name)
		}
	}

	logger.Debugf("discUnits", "%v", systemdUnits)
	return systemdUnits
}

func discoverSystemdUnit(logger *log.Logger, name string) types.SystemdUnit {
	unit := types.SystemdUnit{Name: name, Exists: false}
	if output, err := exec.Command("systemctl", "show", name).Output(); err != nil {
		logger.Errorf("discCtlErr", "Error running `systemctl show %s`: %s\nCannot analyze systemd units.", name, err.Error())

	} else {
		attr := make(map[string]string)
		for _, line := range strings.Split(string(output), "\n") {
			elements := strings.SplitN(line, "=", 2) // Looking for "Foo=Bar" settings
			if len(elements) == 2 {                  // found that, record it...
				attr[elements[0]] = elements[1]
			}
		}

		if val := attr["LoadState"]; val != "loaded" {
			logger.Debugf("discUnitENoExist", "systemd unit '%s' does not exist. LoadState is '%s'", name, val)
			return unit // doesn't exist - leave everything blank

		} else {
			unit.Exists = true
		}

		if val := attr["UnitFileState"]; val == "enabled" {
			logger.Debugf("discUnitEnabled", "systemd unit '%s' is enabled - it will start automatically at boot.", name)
			unit.Enabled = true

		} else {
			logger.Debugf("discUnitNoEnable", "systemd unit '%s' is not enabled - it does not start automatically at boot. UnitFileState is '%s'", name, val)
		}

		if val := attr["ActiveState"]; val == "active" {
			logger.Debugf("discUnitActive", "systemd unit '%s' is currently running", name)
			unit.Active = true

		} else {
			logger.Debugf("discUnitNoActive", "systemd unit '%s' is not currently running. ActiveState is '%s'; exit code was %d.", name, val, unit.ExitStatus)
		}

		fmt.Sscanf(attr["StatusErrno"], "%d", &unit.ExitStatus) // ignore errors...
	}
	return unit
}

package types

import (
	"bytes"
	"cmp"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/gocarina/gocsv"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/openshift-kni/commatrix/pkg/consts"
	"github.com/openshift-kni/commatrix/pkg/utils"
)

type Env int

const (
	Baremetal Env = iota
	Cloud
)

type Deployment int

const (
	SNO Deployment = iota
	MNO
)

const (
	FormatJSON = "json"
	FormatYAML = "yaml"
	FormatCSV  = "csv"
	FormatNFT  = "nft"
)

type ComMatrix struct {
	Matrix []ComDetails
}

type ComDetails struct {
	Direction string `json:"direction" yaml:"direction" csv:"Direction"`
	Protocol  string `json:"protocol" yaml:"protocol" csv:"Protocol"`
	Port      int    `json:"port" yaml:"port" csv:"Port"`
	Namespace string `json:"namespace" yaml:"namespace" csv:"Namespace"`
	Service   string `json:"service" yaml:"service" csv:"Service"`
	Pod       string `json:"pod" yaml:"pod" csv:"Pod"`
	Container string `json:"container" yaml:"container" csv:"Container"`
	NodeRole  string `json:"nodeRole" yaml:"nodeRole" csv:"Node Role"`
	Optional  bool   `json:"optional" yaml:"optional" csv:"Optional"`
}

type ContainerInfo struct {
	Containers []struct {
		Labels struct {
			ContainerName string `json:"io.kubernetes.container.name"`
			PodName       string `json:"io.kubernetes.pod.name"`
			PodNamespace  string `json:"io.kubernetes.pod.namespace"`
		} `json:"labels"`
	} `json:"containers"`
}

func GetEnv(envStr string) (Env, error) {
	switch envStr {
	case "baremetal":
		return Baremetal, nil
	case "cloud":
		return Cloud, nil
	default:
		return -1, fmt.Errorf("invalid cluster environment: %s", envStr)
	}
}

func GetDeployment(deploymentStr string) (Deployment, error) {
	switch deploymentStr {
	case "mno":
		return MNO, nil
	case "sno":
		return SNO, nil
	default:
		return -1, fmt.Errorf("invalid deployment type: %s", deploymentStr)
	}
}

func (m *ComMatrix) ToCSV() ([]byte, error) {
	out := make([]byte, 0)
	w := bytes.NewBuffer(out)
	csvwriter := csv.NewWriter(w)

	err := gocsv.MarshalCSV(&m.Matrix, csvwriter)
	if err != nil {
		return nil, err
	}

	csvwriter.Flush()

	return w.Bytes(), nil
}

func (m *ComMatrix) ToJSON() ([]byte, error) {
	out, err := json.MarshalIndent(m.Matrix, "", "    ")
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (m *ComMatrix) ToYAML() ([]byte, error) {
	out, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (m *ComMatrix) String() string {
	var result strings.Builder
	for _, details := range m.Matrix {
		result.WriteString(details.String() + "\n")
	}

	return result.String()
}

func (m *ComMatrix) WriteMatrixToFileByType(utilsHelpers utils.UtilsInterface, fileNamePrefix, format string, deployment Deployment, destDir string) error {
	if format == FormatNFT {
		masterMatrix, workerMatrix := m.SeparateMatrixByRole()
		err := masterMatrix.writeMatrixToFile(utilsHelpers, fileNamePrefix+"-master", format, destDir)
		if err != nil {
			return err
		}
		if deployment == MNO {
			err := workerMatrix.writeMatrixToFile(utilsHelpers, fileNamePrefix+"-worker", format, destDir)
			if err != nil {
				return err
			}
		}
		return nil
	}

	err := m.writeMatrixToFile(utilsHelpers, fileNamePrefix, format, destDir)
	if err != nil {
		return err
	}
	return nil
}

func (m *ComMatrix) print(format string) ([]byte, error) {
	switch format {
	case FormatJSON:
		return m.ToJSON()
	case FormatCSV:
		return m.ToCSV()
	case FormatYAML:
		return m.ToYAML()
	case FormatNFT:
		return m.ToNFTables()
	default:
		return nil, fmt.Errorf("invalid format: %s. Please specify json, csv, yaml, or nft", format)
	}
}

func (m *ComMatrix) SeparateMatrixByRole() (ComMatrix, ComMatrix) {
	var masterMatrix, workerMatrix ComMatrix
	for _, entry := range m.Matrix {
		if entry.NodeRole == "master" {
			masterMatrix.Matrix = append(masterMatrix.Matrix, entry)
		} else if entry.NodeRole == "worker" {
			workerMatrix.Matrix = append(workerMatrix.Matrix, entry)
		}
	}

	return masterMatrix, workerMatrix
}

func (m *ComMatrix) writeMatrixToFile(utilsHelpers utils.UtilsInterface, fileName, format string, destDir string) error {
	res, err := m.print(format)
	if err != nil {
		return err
	}

	comMatrixFileName := filepath.Join(destDir, fmt.Sprintf("%s.%s", fileName, format))
	return utilsHelpers.WriteFile(comMatrixFileName, res)
}

// Diff returns the diff ComMatrix.
func (m *ComMatrix) Diff(other ComMatrix) ComMatrix {
	diff := []ComDetails{}
	for _, cd1 := range m.Matrix {
		found := false
		for _, cd2 := range other.Matrix {
			if cd1.Equals(cd2) {
				found = true
				break
			}
		}
		if !found {
			diff = append(diff, cd1)
		}
	}

	return ComMatrix{Matrix: diff}
}

func (m *ComMatrix) Contains(cd ComDetails) bool {
	for _, cd1 := range m.Matrix {
		if cd1.Equals(cd) {
			return true
		}
	}

	return false
}

func (m *ComMatrix) ToNFTables() ([]byte, error) {
	var tcpPorts []string
	var udpPorts []string
	for _, line := range m.Matrix {
		if line.Protocol == "TCP" {
			tcpPorts = append(tcpPorts, fmt.Sprint(line.Port))
		} else if line.Protocol == "UDP" {
			udpPorts = append(udpPorts, fmt.Sprint(line.Port))
		}
	}

	tcpPortsStr := strings.Join(tcpPorts, ", ")
	udpPortsStr := strings.Join(udpPorts, ", ")

	result := fmt.Sprintf(`#!/usr/sbin/nft -f

	table inet openshift_filter {
		chain OPENSHIFT {
			type filter hook input priority 1; policy accept;

			# Allow loopback traffic
			iif lo accept
	
			# Allow established and related traffic
			ct state established,related accept
	
			# Allow ICMP on ipv4
			ip protocol icmp accept
			# Allow ICMP on ipv6
			ip6 nexthdr ipv6-icmp accept

			# Allow specific TCP and UDP ports
			tcp dport  { %s } accept
			udp dport { %s } accept

			# Logging and default drop
			log prefix "firewall " drop
		}
	}`, tcpPortsStr, udpPortsStr)

	return []byte(result), nil
}

// SortAndRemoveDuplicates removes duplicates in the matrix and sort it.
func (m *ComMatrix) SortAndRemoveDuplicates() {
	allKeys := make(map[string]bool)
	res := []ComDetails{}
	for _, item := range m.Matrix {
		str := fmt.Sprintf("%s-%d-%s", item.NodeRole, item.Port, item.Protocol)
		if _, value := allKeys[str]; !value {
			allKeys[str] = true
			res = append(res, item)
		}
	}
	m.Matrix = res

	slices.SortFunc(m.Matrix, func(a, b ComDetails) int {
		res := cmp.Compare(a.NodeRole, b.NodeRole)
		if res != 0 {
			return res
		}

		res = cmp.Compare(a.Protocol, b.Protocol)
		if res != 0 {
			return res
		}

		return cmp.Compare(a.Port, b.Port)
	})
}

func (cd ComDetails) String() string {
	return fmt.Sprintf("%s,%s,%d,%s,%s,%s,%s,%s,%v", cd.Direction, cd.Protocol, cd.Port, cd.Namespace, cd.Service, cd.Pod, cd.Container, cd.NodeRole, cd.Optional)
}

func (cd ComDetails) Equals(other ComDetails) bool {
	strComDetail1 := fmt.Sprintf("%s-%d-%s", cd.NodeRole, cd.Port, cd.Protocol)
	strComDetail2 := fmt.Sprintf("%s-%d-%s", other.NodeRole, other.Port, other.Protocol)

	return strComDetail1 == strComDetail2
}

func GetComMatrixHeadersByFormat(format string) (string, error) {
	typ := reflect.TypeOf(ComDetails{})

	var tagsList []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get(format)
		if tag == "" {
			return "", fmt.Errorf("field %v has no tag of format %s", field, format)
		}
		tagsList = append(tagsList, tag)
	}

	return strings.Join(tagsList, ","), nil
}

func GetNodeRole(node *corev1.Node) (string, error) {
	if _, ok := node.Labels[consts.RoleLabel+"master"]; ok {
		return "master", nil
	}

	if _, ok := node.Labels[consts.RoleLabel+"control-plane"]; ok {
		return "master", nil
	}

	if _, ok := node.Labels[consts.RoleLabel+"worker"]; ok {
		return "worker", nil
	}

	for label := range node.Labels {
		if strings.HasPrefix(label, consts.RoleLabel) {
			return strings.TrimPrefix(label, consts.RoleLabel), nil
		}
	}

	return "", fmt.Errorf("unable to determine role for node %s", node.Name)
}

package admin

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	DefaultPolicyFile                = "openshift.local.config/master/policy.json"
	CreateBootstrapPolicyFileCommand = "create-bootstrap-policy-file"
)

type CreateBootstrapPolicyFileOptions struct {
	File string
}

func NewCommandCreateBootstrapPolicyFile(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateBootstrapPolicyFileOptions{}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create the default bootstrap policy",
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.CreateBootstrapPolicyFile(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	flags := cmd.Flags()

	flags.StringVar(&options.File, "filename", DefaultPolicyFile, "The policy template file that will be written with roles and bindings.")

	// autocompletion hints
	cmd.MarkFlagFilename("filename")

	return cmd
}

func (o CreateBootstrapPolicyFileOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.File) == 0 {
		return errors.New("filename must be provided")
	}

	return nil
}

func (o CreateBootstrapPolicyFileOptions) CreateBootstrapPolicyFile() error {
	if err := os.MkdirAll(path.Dir(o.File), os.FileMode(0755)); err != nil {
		return err
	}

	policyList := &corev1.List{}
	policyList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("List"))
	policy := bootstrappolicy.Policy()
	rbacEncoder := scheme.Codecs.LegacyCodec(rbacv1.SchemeGroupVersion)

	for i := range policy.ClusterRoles {
		versionedObject, err := runtime.Encode(rbacEncoder, &policy.ClusterRoles[i])
		if err != nil {
			return err
		}
		policyList.Items = append(policyList.Items, runtime.RawExtension{Raw: versionedObject})
	}

	for i := range policy.ClusterRoleBindings {
		versionedObject, err := runtime.Encode(rbacEncoder, &policy.ClusterRoleBindings[i])
		if err != nil {
			return err
		}
		policyList.Items = append(policyList.Items, runtime.RawExtension{Raw: versionedObject})
	}

	// iterate in a defined order
	for _, namespace := range sets.StringKeySet(policy.Roles).List() {
		roles := policy.Roles[namespace]
		for i := range roles {
			versionedObject, err := runtime.Encode(rbacEncoder, &roles[i])
			if err != nil {
				return err
			}
			policyList.Items = append(policyList.Items, runtime.RawExtension{Raw: versionedObject})
		}
	}

	// iterate in a defined order
	for _, namespace := range sets.StringKeySet(policy.RoleBindings).List() {
		roleBindings := policy.RoleBindings[namespace]
		for i := range roleBindings {
			versionedObject, err := runtime.Encode(rbacEncoder, &roleBindings[i])
			if err != nil {
				return err
			}
			policyList.Items = append(policyList.Items, runtime.RawExtension{Raw: versionedObject})
		}
	}

	buffer := &bytes.Buffer{}
	if err := (&printers.JSONPrinter{}).PrintObj(policyList, buffer); err != nil {
		return err
	}

	if err := ioutil.WriteFile(o.File, buffer.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}

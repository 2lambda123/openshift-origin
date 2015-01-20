/*
Copyright 2014 Google Inc. All rights reserved.

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

package clientcmd

import (
	"github.com/spf13/pflag"

	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
)

// ConfigOverrides holds values that should override whatever information is pulled from the actual Config object.  You can't
// simply use an actual Config object, because Configs hold maps, but overrides are restricted to "at most one"
type ConfigOverrides struct {
	AuthInfo       clientcmdapi.AuthInfo
	ClusterInfo    clientcmdapi.Cluster
	Namespace      string
	CurrentContext string
	ClusterName    string
	AuthInfoName   string
}

// ConfigOverrideFlags holds the flag names to be used for binding command line flags.  Notice that this structure tightly
// corresponds to ConfigOverrides
type ConfigOverrideFlags struct {
	AuthOverrideFlags    AuthOverrideFlags
	ClusterOverrideFlags ClusterOverrideFlags
	Namespace            string
	CurrentContext       string
	ClusterName          string
	AuthInfoName         string
}

// AuthOverrideFlags holds the flag names to be used for binding command line flags for AuthInfo objects
type AuthOverrideFlags struct {
	AuthPath          string
	AuthPathShort     string
	ClientCertificate string
	ClientKey         string
	Token             string
}

// ClusterOverride holds the flag names to be used for binding command line flags for Cluster objects
type ClusterOverrideFlags struct {
	APIServer             string
	APIServerShort        string
	APIVersion            string
	CertificateAuthority  string
	InsecureSkipTLSVerify string
}

const (
	FlagClusterName  = "cluster"
	FlagAuthInfoName = "user"
	FlagContext      = "context"
	FlagNamespace    = "namespace"
	FlagAPIServer    = "server"
	FlagAPIVersion   = "api-version"
	FlagAuthPath     = "auth-path"
	FlagInsecure     = "insecure-skip-tls-verify"
	FlagCertFile     = "client-certificate"
	FlagKeyFile      = "client-key"
	FlagCAFile       = "certificate-authority"
	FlagBearerToken  = "token"
)

// RecommendedAuthOverrideFlags is a convenience method to return recommended flag names prefixed with a string of your choosing
func RecommendedAuthOverrideFlags(prefix string) AuthOverrideFlags {
	return AuthOverrideFlags{
		AuthPath:          prefix + FlagAuthPath,
		ClientCertificate: prefix + FlagCertFile,
		ClientKey:         prefix + FlagKeyFile,
		Token:             prefix + FlagBearerToken,
	}
}

// RecommendedClusterOverrideFlags is a convenience method to return recommended flag names prefixed with a string of your choosing
func RecommendedClusterOverrideFlags(prefix string) ClusterOverrideFlags {
	return ClusterOverrideFlags{
		APIServer:             prefix + FlagAPIServer,
		APIVersion:            prefix + FlagAPIVersion,
		CertificateAuthority:  prefix + FlagCAFile,
		InsecureSkipTLSVerify: prefix + FlagInsecure,
	}
}

// RecommendedConfigOverrideFlags is a convenience method to return recommended flag names prefixed with a string of your choosing
func RecommendedConfigOverrideFlags(prefix string) ConfigOverrideFlags {
	return ConfigOverrideFlags{
		AuthOverrideFlags:    RecommendedAuthOverrideFlags(prefix),
		ClusterOverrideFlags: RecommendedClusterOverrideFlags(prefix),
		Namespace:            prefix + FlagNamespace,
		CurrentContext:       prefix + FlagContext,
		ClusterName:          prefix + FlagClusterName,
		AuthInfoName:         prefix + FlagAuthInfoName,
	}
}

// BindAuthInfoFlags is a convenience method to bind the specified flags to their associated variables
func BindAuthInfoFlags(authInfo *clientcmdapi.AuthInfo, flags *pflag.FlagSet, flagNames AuthOverrideFlags) {
	// TODO short flag names are impossible to prefix.  code gets cleaner if we remove them
	if len(flagNames.AuthPathShort) > 0 {
		flags.StringVarP(&authInfo.AuthPath, flagNames.AuthPath, flagNames.AuthPathShort, "", "Path to the auth info file. If missing, prompt the user. Only used if using https.")
	} else {
		flags.StringVar(&authInfo.AuthPath, flagNames.AuthPath, "", "Path to the auth info file. If missing, prompt the user. Only used if using https.")
	}
	flags.StringVar(&authInfo.ClientCertificate, flagNames.ClientCertificate, "", "Path to a client key file for TLS.")
	flags.StringVar(&authInfo.ClientKey, flagNames.ClientKey, "", "Path to a client key file for TLS.")
	flags.StringVar(&authInfo.Token, flagNames.Token, "", "Bearer token for authentication to the API server.")
}

// BindClusterFlags is a convenience method to bind the specified flags to their associated variables
func BindClusterFlags(clusterInfo *clientcmdapi.Cluster, flags *pflag.FlagSet, flagNames ClusterOverrideFlags) {
	// TODO short flag names are impossible to prefix.  code gets cleaner if we remove them
	if len(flagNames.APIServerShort) > 0 {
		flags.StringVarP(&clusterInfo.Server, flagNames.APIServer, flagNames.APIServerShort, "", "The address of the Kubernetes API server")
	} else {
		flags.StringVar(&clusterInfo.Server, flagNames.APIServer, "", "The address of the Kubernetes API server")
	}
	flags.StringVar(&clusterInfo.APIVersion, flagNames.APIVersion, "", "The API version to use when talking to the server")
	flags.StringVar(&clusterInfo.CertificateAuthority, flagNames.CertificateAuthority, "", "Path to a cert. file for the certificate authority.")
	flags.BoolVar(&clusterInfo.InsecureSkipTLSVerify, flagNames.InsecureSkipTLSVerify, false, "If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure.")
}

// BindOverrideFlags is a convenience method to bind the specified flags to their associated variables
func BindOverrideFlags(overrides *ConfigOverrides, flags *pflag.FlagSet, flagNames ConfigOverrideFlags) {
	BindAuthInfoFlags(&overrides.AuthInfo, flags, flagNames.AuthOverrideFlags)
	BindClusterFlags(&overrides.ClusterInfo, flags, flagNames.ClusterOverrideFlags)
	// TODO not integrated yet
	// flags.StringVar(&overrides.Namespace, flagNames.Namespace, "", "If present, the namespace scope for this CLI request.")
	flags.StringVar(&overrides.CurrentContext, flagNames.CurrentContext, "", "The name of the kubeconfig context to use")
	flags.StringVar(&overrides.ClusterName, flagNames.ClusterName, "", "The name of the kubeconfig cluster to use")
	flags.StringVar(&overrides.AuthInfoName, flagNames.AuthInfoName, "", "The name of the kubeconfig user to use")
}

package clientcmd

import (
	"strings"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
)

const (
	unknownReason                     = 0
	noServerFoundReason               = 1
	certificateAuthorityUnknownReason = 2

	certificateAuthorityUnknownMsg = "The server uses a certificate signed by unknown authority. You may need to use the --certificate-authority flag to provide the path to a certificate file for the certificate authority, or --insecure-skip-tls-verify to bypass the certificate check and use insecure connections."
	notConfiguredMsg               = `OpenShift is not configured. You need to run the login command in order to create a default config for your server and credentials:
  osc login
You can also run this command again providing the path to a config file directly, either through the --config flag of the OPENSHIFTCONFIG environment variable.
`
)

func GetPrettyMessageFor(err error) string {
	if err == nil {
		return ""
	}

	reason := detectReason(err)

	switch reason {
	case noServerFoundReason:
		return notConfiguredMsg

	case certificateAuthorityUnknownReason:
		return certificateAuthorityUnknownMsg
	}

	return err.Error()
}

func IsNoServerFound(err error) bool {
	return detectReason(err) == noServerFoundReason
}

func IsCertificateAuthorityUnknown(err error) bool {
	return detectReason(err) == certificateAuthorityUnknownReason
}

func IsForbidden(err error) bool {
	return kerrors.IsForbidden(err)
}

func detectReason(err error) int {
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "certificate signed by unknown authority"):
			return certificateAuthorityUnknownReason
		case clientcmd.IsConfigurationInvalid(err), strings.Contains(err.Error(), "no server found for"):
			return noServerFoundReason
		}
	}
	return unknownReason
}

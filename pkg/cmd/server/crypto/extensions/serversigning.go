package extensions

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

var (
	// OpenShiftServerSigningOID is the OpenShift assigned OID arc for certificates signed by the OpenShift server.
	OpenShiftServerSigningOID = oid(OpenShiftOID, 100)
	// OpenShiftServerSigningServiceOID describes the IANA arc for extensions to server certificates generated by the
	// OpenShift service signing mechanism. All elements in this arc should only be used when signing server certificates
	// for use under a service.
	OpenShiftServerSigningServiceOID = oid(OpenShiftServerSigningOID, 2)
	// OpenShiftServerSigningServiceUIDOID is an x509 extension that is applied to server certificates generated for services
	// representing the UID of the service this certificate was generated for. This value is not guaranteed to match the
	// current service UID if the certificates are in the process of being rotated out. The value MUST be an ASN.1
	// PrintableString or UTF8String.
	OpenShiftServerSigningServiceUIDOID = oid(OpenShiftServerSigningServiceOID, 1)
)

// ServiceServerCertificateExtension returns a CertificateExtensionFunc that will add the
// service UID as an x509 v3 extension to the server certificate.
func ServiceServerCertificateExtension(svc *kapi.Service) crypto.CertificateExtensionFunc {
	return func(cert *x509.Certificate) error {
		uid, err := asn1.Marshal(svc.UID)
		if err != nil {
			return err
		}
		cert.ExtraExtensions = append(cert.ExtraExtensions, pkix.Extension{
			Id:       OpenShiftServerSigningServiceUIDOID,
			Critical: false,
			Value:    uid,
		})
		return nil
	}
}

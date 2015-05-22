package admin

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

const CreateKeyPairCommandName = "create-key-pair"

type CreateKeyPairOptions struct {
	PublicKeyFile  string
	PrivateKeyFile string

	Overwrite bool
	Output    cmdutil.Output
}

const create_key_pair_long = `
Create a 2048-bit RSA key pair, and generate PEM-encoded public/private key files.

Example: Creating service account signing and authenticating key files:

    $ CONFIG=openshift.local.config/master
    $ %[1]s --public-key=$CONFIG/serviceaccounts.public.key --private-key=$CONFIG/serviceaccounts.private.key
`

func NewCommandCreateKeyPair(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateKeyPairOptions{Output: cmdutil.Output{out}}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a public/private key pair",
		Long:  fmt.Sprintf(create_key_pair_long, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			err := options.CreateKeyPair()
			kcmdutil.CheckErr(err)
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()

	flags.StringVar(&options.PublicKeyFile, "public-key", "", "The public key file.")
	flags.StringVar(&options.PrivateKeyFile, "private-key", "", "The private key file.")

	flags.BoolVar(&options.Overwrite, "overwrite", false, "Overwrite existing key files if found. If false, either file existing will prevent creation.")

	return cmd
}

func (o CreateKeyPairOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.PublicKeyFile) == 0 {
		return errors.New("--public-key must be provided")
	}
	if len(o.PrivateKeyFile) == 0 {
		return errors.New("--private-key must be provided")
	}
	if o.PublicKeyFile == o.PrivateKeyFile {
		return errors.New("--public-key and --private-key must be different")
	}

	return nil
}

func (o CreateKeyPairOptions) CreateKeyPair() error {
	glog.V(2).Infof("Creating a key pair with: %#v", o)

	if !o.Overwrite {
		if _, err := os.Stat(o.PrivateKeyFile); err == nil {
			fmt.Fprintf(o.Output.Get(), "Keeping existing private key file %s\n", o.PrivateKeyFile)
			return nil
		}
		if _, err := os.Stat(o.PublicKeyFile); err == nil {
			fmt.Fprintf(o.Output.Get(), "Keeping existing public key file %s\n", o.PublicKeyFile)
			return nil
		}
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	if err := writePrivateKeyFile(o.PrivateKeyFile, privateKey); err != nil {
		return err
	}

	if err := writePublicKeyFile(o.PublicKeyFile, &privateKey.PublicKey); err != nil {
		return err
	}

	fmt.Fprintf(o.Output.Get(), "Generated new key pair as %s and %s\n", o.PublicKeyFile, o.PrivateKeyFile)

	return nil
}

func writePublicKeyFile(path string, key *rsa.PublicKey) error {
	// ensure parent dir
	if err := os.MkdirAll(filepath.Dir(path), os.FileMode(0755)); err != nil {
		return err
	}

	derBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return err
	}

	b := bytes.Buffer{}
	if err := pem.Encode(&b, &pem.Block{Type: "RSA PUBLIC KEY", Bytes: derBytes}); err != nil {
		return err
	}

	return ioutil.WriteFile(path, b.Bytes(), os.FileMode(0600))
}

func writePrivateKeyFile(path string, key *rsa.PrivateKey) error {
	// ensure parent dir
	if err := os.MkdirAll(filepath.Dir(path), os.FileMode(0755)); err != nil {
		return err
	}

	b := bytes.Buffer{}
	err := pem.Encode(&b, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, b.Bytes(), os.FileMode(0600))
}

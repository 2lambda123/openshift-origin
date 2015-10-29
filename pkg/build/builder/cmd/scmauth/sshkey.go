package scmauth

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
)

const SSHPrivateKeyMethodName = "ssh-privatekey"

// SSHPrivateKey implements SCMAuth interface for using SSH private keys.
type SSHPrivateKey struct{}

// Setup creates a wrapper script for SSH command to be able to use the provided
// SSH key while accessing private repository.
func (_ SSHPrivateKey) Setup(baseDir string) (*url.URL, error) {
	script, err := ioutil.TempFile("", "gitssh")
	if err != nil {
		return nil, err
	}
	defer script.Close()
	if err := script.Chmod(0711); err != nil {
		return nil, err
	}
	if _, err := script.WriteString("#!/bin/sh\nssh -i " +
		filepath.Join(baseDir, SSHPrivateKeyMethodName) +
		" -o StrictHostKeyChecking=false \"$@\"\n"); err != nil {
		return nil, err
	}
	// set environment variable to tell git to use the SSH wrapper
	if err := os.Setenv("GIT_SSH", script.Name()); err != nil {
		return nil, err
	}
	return nil, nil
}

// Name returns the name of this auth method.
func (_ SSHPrivateKey) Name() string {
	return SSHPrivateKeyMethodName
}

// Handles returns true if the file is an SSH private key
func (_ SSHPrivateKey) Handles(name string) bool {
	return name == SSHPrivateKeyMethodName
}

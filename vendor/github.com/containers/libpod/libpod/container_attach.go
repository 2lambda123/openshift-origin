package libpod

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/containers/libpod/pkg/kubeutils"
	"github.com/containers/libpod/utils"
	"github.com/docker/docker/pkg/term"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"k8s.io/client-go/tools/remotecommand"
)

/* Sync with stdpipe_t in conmon.c */
const (
	AttachPipeStdin  = 1
	AttachPipeStdout = 2
	AttachPipeStderr = 3
)

// AttachStreams contains streams that will be attached to the container
type AttachStreams struct {
	// OutputStream will be attached to container's STDOUT
	OutputStream io.WriteCloser
	// ErrorStream will be attached to container's STDERR
	ErrorStream io.WriteCloser
	// InputStream will be attached to container's STDIN
	InputStream io.Reader
	// AttachOutput is whether to attach to STDOUT
	// If false, stdout will not be attached
	AttachOutput bool
	// AttachError is whether to attach to STDERR
	// If false, stdout will not be attached
	AttachError bool
	// AttachInput is whether to attach to STDIN
	// If false, stdout will not be attached
	AttachInput bool
}

// Attach to the given container
// Does not check if state is appropriate
func (c *Container) attach(streams *AttachStreams, keys string, resize <-chan remotecommand.TerminalSize, startContainer bool) error {
	if !streams.AttachOutput && !streams.AttachError && !streams.AttachInput {
		return errors.Wrapf(ErrInvalidArg, "must provide at least one stream to attach to")
	}

	// Check the validity of the provided keys first
	var err error
	detachKeys := []byte{}
	if len(keys) > 0 {
		detachKeys, err = term.ToBytes(keys)
		if err != nil {
			return errors.Wrapf(err, "invalid detach keys")
		}
	}

	logrus.Debugf("Attaching to container %s", c.ID())

	return c.attachContainerSocket(resize, detachKeys, streams, startContainer)
}

// attachContainerSocket connects to the container's attach socket and deals with the IO
// TODO add a channel to allow interrupting
func (c *Container) attachContainerSocket(resize <-chan remotecommand.TerminalSize, detachKeys []byte, streams *AttachStreams, startContainer bool) error {
	kubeutils.HandleResizing(resize, func(size remotecommand.TerminalSize) {
		controlPath := filepath.Join(c.bundlePath(), "ctl")
		controlFile, err := os.OpenFile(controlPath, unix.O_WRONLY, 0)
		if err != nil {
			logrus.Debugf("Could not open ctl file: %v", err)
			return
		}
		defer controlFile.Close()

		logrus.Debugf("Received a resize event: %+v", size)
		if _, err = fmt.Fprintf(controlFile, "%d %d %d\n", 1, size.Height, size.Width); err != nil {
			logrus.Warnf("Failed to write to control file to resize terminal: %v", err)
		}
	})
	logrus.Debug("connecting to socket ", c.AttachSocketPath())

	conn, err := net.DialUnix("unixpacket", nil, &net.UnixAddr{Name: c.AttachSocketPath(), Net: "unixpacket"})
	if err != nil {
		return errors.Wrapf(err, "failed to connect to container's attach socket: %v", c.AttachSocketPath())
	}
	defer conn.Close()

	if startContainer {
		if err := c.start(); err != nil {
			return err
		}
	}

	receiveStdoutError := make(chan error)
	go func() {
		receiveStdoutError <- redirectResponseToOutputStreams(streams.OutputStream, streams.ErrorStream, streams.AttachOutput, streams.AttachError, conn)
	}()

	stdinDone := make(chan error)
	go func() {
		var err error
		if streams.AttachInput {
			_, err = utils.CopyDetachable(conn, streams.InputStream, detachKeys)
			conn.CloseWrite()
		}
		stdinDone <- err
	}()

	select {
	case err := <-receiveStdoutError:
		return err
	case err := <-stdinDone:
		if _, ok := err.(utils.DetachError); ok {
			return nil
		}
		if streams.AttachOutput || streams.AttachError {
			return <-receiveStdoutError
		}
	}
	return nil
}

func redirectResponseToOutputStreams(outputStream, errorStream io.Writer, writeOutput, writeError bool, conn io.Reader) error {
	var err error
	buf := make([]byte, 8192+1) /* Sync with conmon STDIO_BUF_SIZE */
	for {
		nr, er := conn.Read(buf)
		if nr > 0 {
			var dst io.Writer
			var doWrite bool
			switch buf[0] {
			case AttachPipeStdout:
				dst = outputStream
				doWrite = writeOutput
			case AttachPipeStderr:
				dst = errorStream
				doWrite = writeError
			default:
				logrus.Infof("Received unexpected attach type %+d", buf[0])
			}

			if doWrite {
				nw, ew := dst.Write(buf[1:nr])
				if ew != nil {
					err = ew
					break
				}
				if nr != nw+1 {
					err = io.ErrShortWrite
					break
				}
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	return err
}

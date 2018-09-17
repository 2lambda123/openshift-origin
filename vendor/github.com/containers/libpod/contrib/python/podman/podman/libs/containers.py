"""Models for manipulating containers and storage."""
import collections
import functools
import getpass
import json
import signal
import time

from ._containers_attach import Mixin as AttachMixin
from ._containers_start import Mixin as StartMixin


class Container(AttachMixin, StartMixin, collections.UserDict):
    """Model for a container."""

    def __init__(self, client, id, data):
        """Construct Container Model."""
        super(Container, self).__init__(data)

        self._client = client
        self._id = id

        with client() as podman:
            self._refresh(podman)

        assert self._id == data['id'],\
            'Requested container id({}) does not match store id({})'.format(
                self._id, data['id']
            )

    def __getitem__(self, key):
        """Get items from parent dict."""
        return super().__getitem__(key)

    def _refresh(self, podman):
        ctnr = podman.GetContainer(self._id)
        super().update(ctnr['container'])

        for k, v in self.data.items():
            setattr(self, k, v)
        if 'containerrunning' in self.data:
            setattr(self, 'running', self.data['containerrunning'])
            self.data['running'] = self.data['containerrunning']

        return self

    def refresh(self):
        """Refresh status fields for this container."""
        with self._client() as podman:
            return self._refresh(podman)

    def processes(self):
        """Show processes running in container."""
        with self._client() as podman:
            results = podman.ListContainerProcesses(self._id)
        yield from results['container']

    def changes(self):
        """Retrieve container changes."""
        with self._client() as podman:
            results = podman.ListContainerChanges(self._id)
        return results['container']

    def kill(self, signal=signal.SIGTERM, wait=25):
        """Send signal to container.

        default signal is signal.SIGTERM.
        wait n of seconds, 0 waits forever.
        """
        with self._client() as podman:
            podman.KillContainer(self._id, signal)
            timeout = time.time() + wait
            while True:
                self._refresh(podman)
                if self.status != 'running':
                    return self

                if wait and timeout < time.time():
                    raise TimeoutError()

                time.sleep(0.5)

    def _lower_hook(self):
        """Convert all keys to lowercase."""

        @functools.wraps(self._lower_hook)
        def wrapped(input_):
            return {k.lower(): v for (k, v) in input_.items()}

        return wrapped

    def inspect(self):
        """Retrieve details about containers."""
        with self._client() as podman:
            results = podman.InspectContainer(self._id)
        obj = json.loads(results['container'], object_hook=self._lower_hook())
        return collections.namedtuple('ContainerInspect', obj.keys())(**obj)

    def export(self, target):
        """Export container from store to tarball.

        TODO: should there be a compress option, like images?
        """
        with self._client() as podman:
            results = podman.ExportContainer(self._id, target)
        return results['tarfile']

    def commit(self,
               image_name,
               *args,
               changes=[],
               message='',
               pause=True,
               **kwargs):
        """Create image from container.

        All changes overwrite existing values.
          See inspect() to obtain current settings.

        Changes:
            CMD=/usr/bin/zsh
            ENTRYPOINT=/bin/sh date
            ENV=TEST=test_containers.TestContainers.test_commit
            EXPOSE=8888/tcp
            LABEL=unittest=test_commit
            USER=bozo:circus
            VOLUME=/data
            WORKDIR=/data/application
        """
        # TODO: Clean up *args, **kwargs after Commit() is complete
        try:
            author = kwargs.get('author', getpass.getuser())
        except Exception:  # pylint: disable=broad-except
            author = ''

        for c in changes:
            if c.startswith('LABEL=') and c.count('=') < 2:
                raise ValueError(
                    'LABEL should have the format: LABEL=label=value, not {}'.
                    format(c))

        with self._client() as podman:
            results = podman.Commit(self._id, image_name, changes, author,
                                    message, pause)
        return results['image']

    def stop(self, timeout=25):
        """Stop container, return id on success."""
        with self._client() as podman:
            podman.StopContainer(self._id, timeout)
            return self._refresh(podman)

    def remove(self, force=False):
        """Remove container, return id on success.

        force=True, stop running container.
        """
        with self._client() as podman:
            results = podman.RemoveContainer(self._id, force)
        return results['container']

    def restart(self, timeout=25):
        """Restart container with timeout, return id on success."""
        with self._client() as podman:
            podman.RestartContainer(self._id, timeout)
            return self._refresh(podman)

    def rename(self, target):
        """Rename container, return id on success."""
        with self._client() as podman:
            # TODO: Need arguments
            results = podman.RenameContainer()
        # TODO: fixup objects cached information
        return results['container']

    def resize_tty(self, width, height):
        """Resize container tty."""
        with self._client() as podman:
            # TODO: magic re: attach(), arguments
            podman.ResizeContainerTty()

    def pause(self):
        """Pause container, return id on success."""
        with self._client() as podman:
            podman.PauseContainer(self._id)
            return self._refresh(podman)

    def unpause(self):
        """Unpause container, return id on success."""
        with self._client() as podman:
            podman.UnpauseContainer(self._id)
            return self._refresh(podman)

    def update_container(self, *args, **kwargs):
        """TODO: Update container..., return id on success."""
        with self._client() as podman:
            podman.UpdateContainer()
            return self._refresh(podman)

    def wait(self):
        """Wait for container to finish, return 'returncode'."""
        with self._client() as podman:
            results = podman.WaitContainer(self._id)
        return int(results['exitcode'])

    def stats(self):
        """Retrieve resource stats from the container."""
        with self._client() as podman:
            results = podman.GetContainerStats(self._id)
        obj = results['container']
        return collections.namedtuple('StatDetail', obj.keys())(**obj)

    def logs(self, *args, **kwargs):
        """Retrieve container logs."""
        with self._client() as podman:
            results = podman.GetContainerLogs(self._id)
        yield from results['container']


class Containers():
    """Model for Containers collection."""

    def __init__(self, client):
        """Construct model for Containers collection."""
        self._client = client

    def list(self):
        """List of containers in the container store."""
        with self._client() as podman:
            results = podman.ListContainers()
        for cntr in results['containers']:
            yield Container(self._client, cntr['id'], cntr)

    def delete_stopped(self):
        """Delete all stopped containers."""
        with self._client() as podman:
            results = podman.DeleteStoppedContainers()
        return results['containers']

    def get(self, id_):
        """Retrieve container details from store."""
        with self._client() as podman:
            cntr = podman.GetContainer(id_)
        return Container(self._client, cntr['container']['id'],
                         cntr['container'])

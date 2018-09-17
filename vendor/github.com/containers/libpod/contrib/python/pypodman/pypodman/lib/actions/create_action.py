"""Remote client command for creating container from image."""
import argparse
import sys
from builtins import vars

import podman
from pypodman.lib import AbstractActionBase


class UnitAction(argparse.Action):
    """Validate number given is positive integer, with optional suffix."""

    def __call__(self, parser, namespace, values, option_string=None):
        """Validate input."""
        if isinstance(values, str):
            if not values[:-1].isdigit():
                msg = 'unit must be a positive integer, with optional suffix'
                raise argparse.ArgumentError(self, msg)
            if not values[-1] in ('b', 'k', 'm', 'g'):
                msg = 'unit only supports suffices of: b, k, m, g'
                raise argparse.ArgumentError(self, msg)
        elif values <= 0:
            msg = 'number must be a positive integer.'
            raise argparse.ArgumentError(self, msg)

        setattr(namespace, self.dest, values)


def add_options(parser):
    """Add options for Create command."""
    parser.add_argument(
        '--add-host',
        action='append',
        metavar='HOST',
        help=('Add a line to /etc/hosts. The format is hostname:ip.'
              ' The option can be set multiple times.'),
    )
    parser.add_argument(
        '--attach',
        '-a',
        action='append',
        metavar='FD',
        help=('Attach to STDIN, STDOUT or STDERR. The option can be set'
              ' for each of stdin, stdout, and stderr.'))
    parser.add_argument(
        '--annotation',
        action='append',
        help=('Add an annotation to the container. The format is'
              ' key=value. The option can be set multiple times.'))
    parser.add_argument(
        '--blkio-weight',
        choices=range(10, 1000),
        metavar='[10-1000]',
        help=('Block IO weight (relative weight) accepts a'
              ' weight value between 10 and 1000.'))
    parser.add_argument(
        '--blkio-weight-device',
        action='append',
        metavar='WEIGHT',
        help=('Block IO weight (relative device weight,'
              ' format: DEVICE_NAME:WEIGHT).'))
    parser.add_argument(
        '--cap-add',
        action='append',
        metavar='CAP',
        help=('Add Linux capabilities'
              'The option can be set multiple times.'))
    parser.add_argument(
        '--cap-drop',
        action='append',
        metavar='CAP',
        help=('Drop Linux capabilities'
              'The option can be set multiple times.'))
    parser.add_argument(
        '--cgroup-parent',
        metavar='PATH',
        help=('Path to cgroups under which the cgroup for the'
              ' container will be created. If the path is not'
              ' absolute, the path is considered to be relative'
              ' to the cgroups path of the init process. Cgroups'
              ' will be created if they do not already exist.'))
    parser.add_argument(
        '--cidfile',
        metavar='PATH',
        help='Write the container ID to the file, on the remote host.')
    parser.add_argument(
        '--conmon-pidfile',
        metavar='PATH',
        help=('Write the pid of the conmon process to a file,'
              ' on the remote host.'))
    parser.add_argument(
        '--cpu-count',
        type=int,
        metavar='COUNT',
        help=('Limit the number of CPUs available'
              ' for execution by the container.'))
    parser.add_argument(
        '--cpu-period',
        type=int,
        metavar='PERIOD',
        help=('Limit the CPU CFS (Completely Fair Scheduler) period.'))
    parser.add_argument(
        '--cpu-quota',
        type=int,
        metavar='QUOTA',
        help=('Limit the CPU CFS (Completely Fair Scheduler) quota.'))
    parser.add_argument(
        '--cpu-rt-period',
        type=int,
        metavar='PERIOD',
        help=('Limit the CPU real-time period in microseconds.'))
    parser.add_argument(
        '--cpu-rt-runtime',
        type=int,
        metavar='LIMIT',
        help=('Limit the CPU real-time runtime in microseconds.'))
    parser.add_argument(
        '--cpu-shares',
        type=int,
        metavar='SHARES',
        help=('CPU shares (relative weight)'))
    parser.add_argument(
        '--cpus',
        type=int,
        help=('Number of CPUs. The default is 0 which means no limit'))
    parser.add_argument(
        '--cpuset-cpus',
        metavar='LIST',
        help=('CPUs in which to allow execution (0-3, 0,1)'))
    parser.add_argument(
        '--cpuset-mems',
        metavar='NODES',
        help=('Memory nodes (MEMs) in which to allow execution (0-3, 0,1).'
              ' Only effective on NUMA systems'))
    parser.add_argument(
        '--detach',
        '-d',
        choices=['True', 'False'],
        help=('Detached mode: run the container in the background and'
              ' print the new container ID. The default is false.'))
    parser.add_argument(
        '--detach-keys',
        metavar='KEY(s)',
        help=('Override the key sequence for detaching a container.'
              ' Format is a single character [a-Z] or ctrl-<value> where'
              ' <value> is one of: a-z, @, ^, [, , or _.'))
    parser.add_argument(
        '--device',
        action='append',
        help=('Add a host device to the container'
              'The option can be set multiple times.'),
    )
    parser.add_argument(
        '--device-read-bps',
        action='append',
        metavar='LIMIT',
        help=('Limit read rate (bytes per second) from a device'
              ' (e.g. --device-read-bps=/dev/sda:1mb)'
              'The option can be set multiple times.'),
    )
    parser.add_argument(
        '--device-read-iops',
        action='append',
        metavar='LIMIT',
        help=('Limit read rate (IO per second) from a device'
              ' (e.g. --device-read-iops=/dev/sda:1000)'
              'The option can be set multiple times.'),
    )
    parser.add_argument(
        '--device-write-bps',
        action='append',
        metavar='LIMIT',
        help=('Limit write rate (bytes per second) to a device'
              ' (e.g. --device-write-bps=/dev/sda:1mb)'
              'The option can be set multiple times.'),
    )
    parser.add_argument(
        '--device-write-iops',
        action='append',
        metavar='LIMIT',
        help=('Limit write rate (IO per second) to a device'
              ' (e.g. --device-write-iops=/dev/sda:1000)'
              'The option can be set multiple times.'),
    )
    parser.add_argument(
        '--dns',
        action='append',
        metavar='SERVER',
        help=('Set custom DNS servers.'
              'The option can be set multiple times.'),
    )
    parser.add_argument(
        '--dns-option',
        action='append',
        metavar='OPT',
        help=('Set custom DNS options.'
              'The option can be set multiple times.'),
    )
    parser.add_argument(
        '--dns-search',
        action='append',
        metavar='DOMAIN',
        help=('Set custom DNS search domains.'
              'The option can be set multiple times.'),
    )
    parser.add_argument(
        '--entrypoint',
        help=('Overwrite the default ENTRYPOINT of the image.'),
    )
    parser.add_argument(
        '--env',
        '-e',
        action='append',
        help=('Set environment variables.'),
    )
    parser.add_argument(
        '--env-file',
        help=('Read in a line delimited file of environment variables,'
              ' on the remote host.'),
    )
    parser.add_argument(
        '--expose',
        metavar='PORT(s)',
        help=('Expose a port, or a range of ports'
              ' (e.g. --expose=3300-3310) to set up port redirection.'),
    )
    parser.add_argument(
        '--gidmap',
        metavar='MAP',
        help=('GID map for the user namespace'),
    )
    parser.add_argument(
        '--group-add',
        action='append',
        metavar='GROUP',
        help=('Add additional groups to run as'))
    parser.add_argument('--hostname', help='Container host name')

    volume_group = parser.add_mutually_exclusive_group()
    volume_group.add_argument(
        '--image-volume',
        choices=['bind', 'tmpfs', 'ignore'],
        metavar='MODE',
        help='Tells podman how to handle the builtin image volumes')
    volume_group.add_argument(
        '--builtin-volume',
        choices=['bind', 'tmpfs', 'ignore'],
        metavar='MODE',
        help='Tells podman how to handle the builtin image volumes')
    parser.add_argument(
        '--interactive',
        '-i',
        choices=['True', 'False'],
        help='Keep STDIN open even if not attached. The default is false')
    parser.add_argument('--ipc', help='Create namespace')
    parser.add_argument(
        '--kernel-memory',
        action=UnitAction,
        metavar='UNIT',
        help=('Kernel memory limit (format: <number>[<unit>],'
              ' where unit = b, k, m or g)'))
    parser.add_argument(
        '--label',
        '-l',
        help=('Add metadata to a container'
              ' (e.g., --label com.example.key=value)'))
    parser.add_argument(
        '--label-file', help='Read in a line delimited file of labels')
    parser.add_argument(
        '--log-driver',
        choices=['json-file', 'journald'],
        help='Logging driver for the container.')
    parser.add_argument(
        '--log-opt', action='append', help='Logging driver specific options')
    parser.add_argument(
        '--mac-address', help='Container MAC address (e.g. 92:d0:c6:0a:29:33)')
    parser.add_argument(
        '--memory',
        '-m',
        action=UnitAction,
        metavar='UNIT',
        help='Memory limit (format: [], where unit = b, k, m or g)')
    parser.add_argument(
        '--memory-reservation',
        action=UnitAction,
        metavar='UNIT',
        help='Memory soft limit (format: [], where unit = b, k, m or g)')
    parser.add_argument(
        '--memory-swap',
        action=UnitAction,
        metavar='UNIT',
        help=('A limit value equal to memory plus swap.'
              'Must be used with the --memory flag'))
    parser.add_argument(
        '--memory-swappiness',
        choices=range(0, 100),
        metavar='[0-100]',
        help="Tune a container's memory swappiness behavior")
    parser.add_argument('--name', help='Assign a name to the container')
    parser.add_argument(
        '--network',
        metavar='BRIDGE',
        help=('Set the Network mode for the container.'))
    parser.add_argument(
        '--oom-kill-disable',
        choices=['True', 'False'],
        help='Whether to disable OOM Killer for the container or not')
    parser.add_argument(
        '--oom-score-adj',
        choices=range(-1000, 1000),
        metavar='[-1000-1000]',
        help="Tune the host's OOM preferences for containers")
    parser.add_argument('--pid', help='Set the PID mode for the container')
    parser.add_argument(
        '--pids-limit',
        type=int,
        metavar='LIMIT',
        help=("Tune the container's pids limit."
              " Set -1 to have unlimited pids for the container."))
    parser.add_argument('--pod', help='Run container in an existing pod')
    parser.add_argument(
        '--privileged',
        choices=['True', 'False'],
        help='Give extended privileges to this container.')
    parser.add_argument(
        '--publish',
        '-p',
        metavar='PORT(s)',
        help="Publish a container's port, or range of ports, to the host")
    parser.add_argument(
        '--publish-all',
        '-P',
        action='store_true',
        help=("Publish all exposed ports to random"
              " ports on the host interfaces"))
    parser.add_argument(
        '--quiet',
        '-q',
        action='store_true',
        help='Suppress output information when pulling images')
    parser.add_argument(
        '--read-only',
        choices=['True', 'False'],
        help="Mount the container's root filesystem as read only.")
    parser.add_argument(
        '--rm',
        choices=['True', 'False'],
        help='Automatically remove the container when it exits.')
    parser.add_argument(
        '--rootfs',
        action='store_true',
        help=('If specified, the first argument refers to an'
              ' exploded container on the file system of remote host.'))
    parser.add_argument(
        '--security-opt',
        action='append',
        metavar='OPT',
        help='Set security options.')
    parser.add_argument(
        '--shm-size',
        action=UnitAction,
        metavar='UNIT',
        help='Size of /dev/shm')
    parser.add_argument(
        '--stop-signal', metavar='SIGTERM', help='Signal to stop a container')
    parser.add_argument(
        '--stop-timeout',
        metavar='TIMEOUT',
        help='Seconds to wait on stopping container.')
    parser.add_argument(
        '--subgidname',
        metavar='MAP',
        help='Name for GID map from the /etc/subgid file')
    parser.add_argument(
        '--subuidname',
        metavar='MAP',
        help='Name for UID map from the /etc/subuid file')
    parser.add_argument(
        '--sysctl',
        action='append',
        help='Configure namespaced kernel parameters at runtime')
    parser.add_argument('--tmpfs', help='Create a tmpfs mount')
    parser.add_argument(
        '--tty',
        '-t',
        choices=['True', 'False'],
        help='Allocate a pseudo-TTY for standard input of container.')
    parser.add_argument(
        '--uidmap', metavar='MAP', help='UID map for the user namespace')
    parser.add_argument('--ulimit', metavar='OPT', help='Ulimit options')
    parser.add_argument(
        '--user',
        '-u',
        help=('Sets the username or UID used and optionally'
              ' the groupname or GID for the specified command.'))
    parser.add_argument(
        '--userns',
        choices=['host', 'ns'],
        help='Set the usernamespace mode for the container')
    parser.add_argument(
        '--uts',
        choices=['host', 'ns'],
        help='Set the UTS mode for the container')
    parser.add_argument('--volume', '-v', help='Create a bind mount.')
    parser.add_argument(
        '--volumes-from',
        action='append',
        help='Mount volumes from the specified container(s).')
    parser.add_argument(
        '--workdir', '-w', help='Working directory inside the container')


class Create(AbstractActionBase):
    """Class for creating container from image."""

    @classmethod
    def subparser(cls, parent):
        """Add Create command to parent parser."""
        parser = parent.add_parser(
            'create', help='create container from image')

        add_options(parser)

        parser.add_argument('image', nargs='*', help='source image id.')
        parser.set_defaults(class_=cls, method='create')

    def __init__(self, args):
        """Construct Create class."""
        super().__init__(args)
        if not args.image:
            raise ValueError('You must supply at least one image id'
                             ' or name to be retrieved.')

    def create(self):
        """Create container."""
        # Dump all unset arguments before transmitting to service
        opts = {k: v for k, v in vars(self._args).items() if v is not None}

        # image id(s) used only on client
        del opts['image']

        for ident in self._args.image:
            try:
                img = self.client.images.get(ident)
                img.container(**opts)
                print(ident)
            except podman.ImageNotFound as e:
                sys.stdout.flush()
                print(
                    'Image {} not found.'.format(e.name),
                    file=sys.stderr,
                    flush=True)
            except podman.ErrorOccurred as e:
                sys.stdout.flush()
                print(
                    '{}'.format(e.reason).capitalize(),
                    file=sys.stderr,
                    flush=True)

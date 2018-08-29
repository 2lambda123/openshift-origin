% podman-wait "1"

## NAME
podman\-wait - Waits on one or more containers to stop and prints exit code

## SYNOPSIS
**podman wait** [*options*] *container*

## DESCRIPTION
Waits on one or more containers to stop.  The container can be referred to by its
name or ID.  In the case of multiple containers, podman will wait on each consecutively.
After the container stops, the container's return code is printed.

## OPTIONS

**--help, -h**

  Print usage statement

**--latest, -l**

  Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods.

## EXAMPLES

  podman wait mywebserver

  podman wait --latest

  podman wait 860a4b23

  podman wait mywebserver myftpserver

## SEE ALSO
podman(1), crio(8)

## HISTORY
September 2017, Originally compiled by Brent Baude<bbaude@redhat.com>

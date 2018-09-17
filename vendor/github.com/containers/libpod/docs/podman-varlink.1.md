% podman-varlink "1"

## NAME
podman\-varlink - Runs the varlink backend interface

## SYNOPSIS
**podman varlink** [*options*] *uri*

## DESCRIPTION
Starts the varlink service listening on *uri* that allows varlink clients to interact with podman.  This should generally be done
with systemd.  See _Configuration_ below.

## GLOBAL OPTIONS

**--help, -h**
  Print usage statement

## OPTIONS
**--timeout, -t**

The time until the varlink session expires in _milliseconds_. The default is 1
second. A value of `0` means no timeout and the session will not expire.

## EXAMPLES

Run the podman varlink service manually and accept the default timeout.

```
$ podman varlink unix:/run/podman/io.podman
```

Run the podman varlink service manually with a 5 second timeout.

```
$ podman varlink --timeout 5000 unix:/run/podman/io.podman
```

## CONFIGURATION

Users of the podman varlink service should enable the _io.podman.socket_ and _io.podman.service_.
This is the preferred method for running the varlink service.

You can do this via systemctl.

```
$ systemctl enable --now io.podman.socket
```

## SEE ALSO
podman(1), systemctl(1)

## HISTORY
April 2018, Originally compiled by Brent Baude<bbaude@redhat.com>

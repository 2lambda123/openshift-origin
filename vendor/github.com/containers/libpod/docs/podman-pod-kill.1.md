% podman-pod-kill(1)

## NAME
podman\-pod\-kill - Kills all containers in one or more pods with a signal

## SYNOPSIS
**podman pod kill** [*options*] *pod* ...

## DESCRIPTION
The main process of each container inside the pods specified will be sent SIGKILL, or any signal specified with option --signal.

## OPTIONS
**--all, -a**

Sends signal to all containers associated with a pod.

**--latest, -l**

Instead of providing the pod name or ID, use the last created pod. If you use methods other than Podman
to run pods such as CRI-O, the last started pod could be from either of those methods.

**--signal, s**

Signal to send to the containers in the pod. For more information on Linux signals, refer to *man signal(7)*.


## EXAMPLE

podman pod kill mywebserver

podman pod kill 860a4b23

podman pod kill --signal TERM 860a4b23

podman pod kill --latest

podman pod kill --all

## SEE ALSO
podman-pod(1), podman-pod-stop(1)

## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>

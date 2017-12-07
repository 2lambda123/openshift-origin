## kubectl drain

Drain node in preparation for maintenance

### Synopsis


Drain node in preparation for maintenance. 

The given node will be marked unschedulable to prevent new pods from arriving. 'drain' evicts the pods if the APIServer supports eviction (http://kubernetes.io/docs/admin/disruptions/). Otherwise, it will use normal DELETE to delete the pods. The 'drain' evicts or deletes all pods except mirror pods (which cannot be deleted through the API server).  If there are DaemonSet-managed pods, drain will not proceed without --ignore-daemonsets, and regardless it will not delete any DaemonSet-managed pods, because those pods would be immediately replaced by the DaemonSet controller, which ignores unschedulable markings.  If there are any pods that are neither mirror pods nor managed by ReplicationController, ReplicaSet, DaemonSet, StatefulSet or Job, then drain will not delete any pods unless you use --force.  --force will also allow deletion to proceed if the managing resource of one or more pods is missing. 

'drain' waits for graceful termination. You should not operate on the machine until the command completes. 

When you are ready to put the node back into service, use kubectl uncordon, which will make the node schedulable again. 

! http://kubernetes.io/images/docs/kubectl_drain.svg

```
kubectl drain NODE
```

### Examples

```
  # Drain node "foo", even if there are pods not managed by a ReplicationController, ReplicaSet, Job, DaemonSet or StatefulSet on it.
  $ kubectl drain foo --force
  
  # As above, but abort if there are pods not managed by a ReplicationController, ReplicaSet, Job, DaemonSet or StatefulSet, and use a grace period of 15 minutes.
  $ kubectl drain foo --grace-period=900
```

### Options

```
      --delete-local-data   Continue even if there are pods using emptyDir (local data that will be deleted when the node is drained).
      --dry-run             If true, only print the object that would be sent, without sending it.
      --force               Continue even if there are pods not managed by a ReplicationController, ReplicaSet, Job, DaemonSet or StatefulSet.
      --grace-period int    Period of time in seconds given to each pod to terminate gracefully. If negative, the default value specified in the pod will be used. (default -1)
      --ignore-daemonsets   Ignore DaemonSet-managed pods.
  -l, --selector string     Selector (label query) to filter on
      --timeout duration    The length of time to wait before giving up, zero means infinite
```

### Options inherited from parent commands

```
      --alsologtostderr                  log to standard error as well as files
      --as string                        Username to impersonate for the operation
      --as-group stringArray             Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --cache-dir string                 Default HTTP cache directory (default "/home/username/.kube/http-cache")
      --certificate-authority string     Path to a cert file for the certificate authority
      --client-certificate string        Path to a client certificate file for TLS
      --client-key string                Path to a client key file for TLS
      --cluster string                   The name of the kubeconfig cluster to use
      --context string                   The name of the kubeconfig context to use
      --insecure-skip-tls-verify         If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string                Path to the kubeconfig file to use for CLI requests.
      --log-backtrace-at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                   If non-empty, write log files in this directory
      --logtostderr                      log to standard error instead of files
      --match-server-version             Require server version to match client version
  -n, --namespace string                 If present, the namespace scope for this CLI request
      --password string                  Password for basic authentication to the API server
      --request-timeout string           The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                    The address and port of the Kubernetes API server
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
      --token string                     Bearer token for authentication to the API server
      --user string                      The name of the kubeconfig user to use
      --username string                  Username for basic authentication to the API server
  -v, --v Level                          log level for V logs
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO
* [kubectl](kubectl.md)	 - kubectl controls the Kubernetes cluster manager

###### Auto generated by spf13/cobra on 29-Nov-2017

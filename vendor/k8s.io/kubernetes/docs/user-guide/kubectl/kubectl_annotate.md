## kubectl annotate

Update the annotations on a resource

### Synopsis


Update the annotations on one or more resources 

All Kubernetes objects support the ability to store additional data with the object as annotations. Annotations are key/value pairs that can be larger than labels and include arbitrary string values such as structured JSON. Tools and system extensions may use annotations to store their own data. 

Attempting to set an annotation that already exists will fail unless --overwrite is set. If --resource-version is specified and does not match the current resource version on the server the command will fail.

Valid resource types include: 

  * all  
  * certificatesigningrequests (aka 'csr')  
  * clusterrolebindings  
  * clusterroles  
  * componentstatuses (aka 'cs')  
  * configmaps (aka 'cm')  
  * controllerrevisions  
  * cronjobs  
  * customresourcedefinition (aka 'crd')  
  * daemonsets (aka 'ds')  
  * deployments (aka 'deploy')  
  * endpoints (aka 'ep')  
  * events (aka 'ev')  
  * horizontalpodautoscalers (aka 'hpa')  
  * ingresses (aka 'ing')  
  * jobs  
  * limitranges (aka 'limits')  
  * namespaces (aka 'ns')  
  * networkpolicies (aka 'netpol')  
  * nodes (aka 'no')  
  * persistentvolumeclaims (aka 'pvc')  
  * persistentvolumes (aka 'pv')  
  * poddisruptionbudgets (aka 'pdb')  
  * podpreset  
  * pods (aka 'po')  
  * podsecuritypolicies (aka 'psp')  
  * podtemplates  
  * replicasets (aka 'rs')  
  * replicationcontrollers (aka 'rc')  
  * resourcequotas (aka 'quota')  
  * rolebindings  
  * roles  
  * secrets  
  * serviceaccounts (aka 'sa')  
  * services (aka 'svc')  
  * statefulsets (aka 'sts')  
  * storageclasses (aka 'sc')

```
kubectl annotate [--overwrite] (-f FILENAME | TYPE NAME) KEY_1=VAL_1 ... KEY_N=VAL_N [--resource-version=version]
```

### Examples

```
  # Update pod 'foo' with the annotation 'description' and the value 'my frontend'.
  # If the same annotation is set multiple times, only the last value will be applied
  kubectl annotate pods foo description='my frontend'
  
  # Update a pod identified by type and name in "pod.json"
  kubectl annotate -f pod.json description='my frontend'
  
  # Update pod 'foo' with the annotation 'description' and the value 'my frontend running nginx', overwriting any existing value.
  kubectl annotate --overwrite pods foo description='my frontend running nginx'
  
  # Update all pods in the namespace
  kubectl annotate pods --all description='my frontend running nginx'
  
  # Update pod 'foo' only if the resource is unchanged from version 1.
  kubectl annotate pods foo description='my frontend running nginx' --resource-version=1
  
  # Update pod 'foo' by removing an annotation named 'description' if it exists.
  # Does not require the --overwrite flag.
  kubectl annotate pods foo description-
```

### Options

```
      --all                           Select all resources, including uninitialized ones, in the namespace of the specified resource types.
      --allow-missing-template-keys   If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats. (default true)
      --dry-run                       If true, only print the object that would be sent, without sending it.
  -f, --filename stringSlice          Filename, directory, or URL to files identifying the resource to update the annotation
      --include-uninitialized         If true, the kubectl command applies to uninitialized objects. If explicitly set to false, this flag overrides other flags that make the kubectl commands apply to uninitialized objects, e.g., "--all". Objects with empty metadata.initializers are regarded as initialized.
      --local                         If true, annotation will NOT contact api-server but run locally.
      --no-headers                    When using the default or custom-column output format, don't print headers (default print headers).
  -o, --output string                 Output format. One of: json|yaml|wide|name|custom-columns=...|custom-columns-file=...|go-template=...|go-template-file=...|jsonpath=...|jsonpath-file=... See custom columns [http://kubernetes.io/docs/user-guide/kubectl-overview/#custom-columns], golang template [http://golang.org/pkg/text/template/#pkg-overview] and jsonpath template [http://kubernetes.io/docs/user-guide/jsonpath].
      --overwrite                     If true, allow annotations to be overwritten, otherwise reject annotation updates that overwrite existing annotations.
      --record                        Record current kubectl command in the resource annotation. If set to false, do not record the command. If set to true, record the command. If not set, default to updating the existing annotation value only if one already exists.
  -R, --recursive                     Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.
      --resource-version string       If non-empty, the annotation update will only succeed if this is the current resource-version for the object. Only valid when specifying a single resource.
  -l, --selector string               Selector (label query) to filter on, not including uninitialized ones, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2).
  -a, --show-all                      When printing, show all resources (default hide terminated pods.)
      --show-labels                   When printing, show all labels as the last column (default hide labels column)
      --sort-by string                If non-empty, sort list types using this field specification.  The field specification is expressed as a JSONPath expression (e.g. '{.metadata.name}'). The field in the API resource specified by this JSONPath expression must be an integer or a string.
      --template string               Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
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

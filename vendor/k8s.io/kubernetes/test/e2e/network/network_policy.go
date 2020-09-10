/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package network

import (
	"context"
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	imageutils "k8s.io/kubernetes/test/utils/image"

	"fmt"

	"github.com/onsi/ginkgo"
)

/*
The following Network Policy tests verify that policy object definitions
are correctly enforced by a networking plugin. It accomplishes this by launching
a simple netcat server, and two clients with different
attributes. Each test case creates a network policy which should only allow
connections from one of the clients. The test then asserts that the clients
failed or successfully connected as expected.
*/

var _ = SIGDescribe("NetworkPolicy [LinuxOnly]", func() {
	var service *v1.Service
	var podServer *v1.Pod
	var podServerLabelSelector string
	f := framework.NewDefaultFramework("network-policy")

	ginkgo.BeforeEach(func() {
		// Windows does not support network policies.
		e2eskipper.SkipIfNodeOSDistroIs("windows")
	})

	ginkgo.Context("NetworkPolicy between server and client", func() {
		ginkgo.BeforeEach(func() {
			ginkgo.By("Creating a simple server that serves on port 80 and 81.")
			podServer, service = createServerPodAndService(f, f.Namespace, "server", []int{80, 81})

			ginkgo.By("Waiting for pod ready", func() {
				err := e2epod.WaitTimeoutForPodReadyInNamespace(f.ClientSet, podServer.Name, f.Namespace.Name, framework.PodStartTimeout)
				framework.ExpectNoError(err)
			})

			// podServerLabelSelector holds the value for the podServer's label "pod-name".
			podServerLabelSelector = podServer.ObjectMeta.Labels["pod-name"]

			// Create pods, which should be able to communicate with the server on port 80 and 81.
			ginkgo.By("Testing pods can connect to both ports when no policy is present.")
			testCanConnect(f, f.Namespace, "client-can-connect-80", service, 80)
			testCanConnect(f, f.Namespace, "client-can-connect-81", service, 81)
		})

		ginkgo.AfterEach(func() {
			cleanupServerPodAndService(f, podServer, service)
		})

		ginkgo.It("should support a 'default-deny-ingress' policy [Feature:NetworkPolicy]", func() {
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "deny-ingress",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{},
					Ingress:     []networkingv1.NetworkPolicyIngressRule{},
				},
			}

			policy, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer cleanupNetworkPolicy(f, policy)

			// Create a pod with name 'client-cannot-connect', which will attempt to communicate with the server,
			// but should not be able to now that isolation is on.
			testCannotConnect(f, f.Namespace, "client-cannot-connect", service, 80)
		})

		ginkgo.It("should support a 'default-deny-all' policy [Feature:NetworkPolicy]", func() {
			nsA := f.Namespace
			nsBName := f.BaseName + "-b"
			nsB, err := f.CreateNamespace(nsBName, map[string]string{
				"ns-name": nsBName,
			})
			framework.ExpectNoError(err, "Error occurred while creating namespace-b.")

			ginkgo.By("Creating a simple server in another namespace that serves on port 80 and 81.")
			podB, serviceB := createServerPodAndService(f, nsB, "pod-b", []int{80, 81})

			ginkgo.By("Waiting for pod ready", func() {
				err := e2epod.WaitTimeoutForPodReadyInNamespace(f.ClientSet, podB.Name, nsB.Name, framework.PodStartTimeout)
				framework.ExpectNoError(err)
			})

			ginkgo.By("Creating client-a, which should be able to contact the server in another namespace.", func() {
				testCanConnect(f, nsA, "client-a", serviceB, 80)
			})

			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default-deny-all",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress, networkingv1.PolicyTypeIngress},
					Ingress:     []networkingv1.NetworkPolicyIngressRule{},
					Egress:      []networkingv1.NetworkPolicyEgressRule{},
				},
			}

			policy, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer cleanupNetworkPolicy(f, policy)

			ginkgo.By("Creating client-to-a, which should not be able to contact the server in the same namespace, Ingress check.", func() {
				testCannotConnect(f, nsA, "client-to-a", service, 80)
			})

			ginkgo.By("Creating client-to-b, which should not be able to contact the server in another namespace, Egress check.", func() {
				testCannotConnect(f, nsA, "client-to-b", serviceB, 80)
			})
		})

		ginkgo.It("should enforce policy to allow traffic from pods within server namespace based on PodSelector [Feature:NetworkPolicy]", func() {
			nsA := f.Namespace
			nsBName := f.BaseName + "-b"
			nsB, err := f.CreateNamespace(nsBName, map[string]string{
				"ns-name": nsBName,
			})
			framework.ExpectNoError(err, "Error occurred while creating namespace-b.")

			// All communication should be possible before applying the policy.
			ginkgo.By("Creating client-a, in server's namespace, which should be able to contact the server.", func() {
				testCanConnect(f, nsA, "client-a", service, 80)
			})
			ginkgo.By("Creating client-b, in server's namespace, which should be able to contact the server.", func() {
				testCanConnect(f, nsA, "client-b", service, 80)
			})
			ginkgo.By("Creating client-a, not in server's namespace, which should be able to contact the server.", func() {
				testCanConnect(f, nsB, "client-a", service, 80)
			})

			ginkgo.By("Creating a network policy for the server which allows traffic from the pod 'client-a' in same namespace.")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-client-a-via-pod-selector",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to the Server
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					// Allow traffic only from client-a
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"pod-name": "client-a",
								},
							},
						}},
					}},
				},
			}

			policy, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer cleanupNetworkPolicy(f, policy)

			ginkgo.By("Creating client-a, in server's namespace, which should be able to contact the server.", func() {
				testCanConnect(f, nsA, "client-a", service, 80)
			})
			ginkgo.By("Creating client-b, in server's namespace, which should not be able to contact the server.", func() {
				testCannotConnect(f, nsA, "client-b", service, 80)
			})
			ginkgo.By("Creating client-a, not in server's namespace, which should not be able to contact the server.", func() {
				testCannotConnect(f, nsB, "client-a", service, 80)
			})
		})

		ginkgo.It("should enforce policy to allow traffic only from a different namespace, based on NamespaceSelector [Feature:NetworkPolicy]", func() {
			nsA := f.Namespace
			nsBName := f.BaseName + "-b"
			nsB, err := f.CreateNamespace(nsBName, map[string]string{
				"ns-name": nsBName,
			})
			framework.ExpectNoError(err)

			// Create Server with Service in NS-B
			framework.Logf("Waiting for server to come up.")
			err = e2epod.WaitForPodRunningInNamespace(f.ClientSet, podServer)
			framework.ExpectNoError(err)

			// Create Policy for that service that allows traffic only via namespace B
			ginkgo.By("Creating a network policy for the server which allows traffic from namespace-b.")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-ns-b-via-namespace-selector",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply to server
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					// Allow traffic only from NS-B
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"ns-name": nsBName,
								},
							},
						}},
					}},
				},
			}
			policy, err = f.ClientSet.NetworkingV1().NetworkPolicies(nsA.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer cleanupNetworkPolicy(f, policy)

			testCannotConnect(f, nsA, "client-a", service, 80)
			testCanConnect(f, nsB, "client-b", service, 80)
		})

		ginkgo.It("should enforce policy based on PodSelector with MatchExpressions[Feature:NetworkPolicy]", func() {
			ginkgo.By("Creating a network policy for the server which allows traffic from the pod 'client-a'.")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-client-a-via-pod-selector-with-match-expressions",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							PodSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      "pod-name",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"client-a"},
								}},
							},
						}},
					}},
				},
			}

			policy, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error creating Network Policy %v: %v", policy.ObjectMeta.Name, err)
			defer cleanupNetworkPolicy(f, policy)

			ginkgo.By("Creating client-a which should be able to contact the server.", func() {
				testCanConnect(f, f.Namespace, "client-a", service, 80)
			})
			ginkgo.By("Creating client-b which should not be able to contact the server.", func() {
				testCannotConnect(f, f.Namespace, "client-b", service, 80)
			})
		})

		ginkgo.It("should enforce policy based on NamespaceSelector with MatchExpressions[Feature:NetworkPolicy]", func() {
			nsA := f.Namespace
			nsBName := f.BaseName + "-b"
			nsB, err := f.CreateNamespace(nsBName, map[string]string{
				"ns-name": nsBName,
			})
			framework.ExpectNoError(err, "Error creating namespace %v: %v", nsBName, err)

			nsCName := f.BaseName + "-c"
			nsC, err := f.CreateNamespace(nsCName, map[string]string{
				"ns-name": nsCName,
			})
			framework.ExpectNoError(err, "Error creating namespace %v: %v", nsCName, err)

			// Create Policy for the server that allows traffic from namespace different than namespace-a
			ginkgo.By("Creating a network policy for the server which allows traffic from ns different than namespace-a.")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-any-ns-different-than-ns-a-via-ns-selector-with-match-expressions",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							NamespaceSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      "ns-name",
									Operator: metav1.LabelSelectorOpNotIn,
									Values:   []string{nsCName},
								}},
							},
						}},
					}},
				},
			}

			policy, err = f.ClientSet.NetworkingV1().NetworkPolicies(nsA.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error creating Network Policy %v: %v", policy.ObjectMeta.Name, err)
			defer cleanupNetworkPolicy(f, policy)

			testCannotConnect(f, nsC, "client-a", service, 80)
			testCanConnect(f, nsB, "client-a", service, 80)
		})

		ginkgo.It("should enforce policy based on PodSelector or NamespaceSelector [Feature:NetworkPolicy]", func() {
			nsA := f.Namespace
			nsBName := f.BaseName + "-b"
			nsB, err := f.CreateNamespace(nsBName, map[string]string{
				"ns-name": nsBName,
			})
			framework.ExpectNoError(err, "Error creating namespace %v: %v", nsBName, err)

			// Create Policy for the server that allows traffic only via client B or namespace B
			ginkgo.By("Creating a network policy for the server which allows traffic from client-b or namespace-b.")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-ns-b-via-namespace-selector-or-client-b-via-pod-selector",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"pod-name": "client-b",
								},
							},
						}, {
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"ns-name": nsBName,
								},
							},
						}},
					}},
				},
			}

			policy, err = f.ClientSet.NetworkingV1().NetworkPolicies(nsA.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error creating Network Policy %v: %v", policy.ObjectMeta.Name, err)
			defer cleanupNetworkPolicy(f, policy)

			testCanConnect(f, nsB, "client-a", service, 80)
			testCanConnect(f, nsA, "client-b", service, 80)
			testCannotConnect(f, nsA, "client-c", service, 80)
		})

		ginkgo.It("should enforce policy based on PodSelector and NamespaceSelector [Feature:NetworkPolicy]", func() {
			nsA := f.Namespace
			nsBName := f.BaseName + "-b"
			nsB, err := f.CreateNamespace(nsBName, map[string]string{
				"ns-name": nsBName,
			})
			framework.ExpectNoError(err, "Error creating namespace %v: %v", nsBName, err)

			// Create Policy for the server that allows traffic only via client-b in namespace B
			ginkgo.By("Creating a network policy for the server which allows traffic from client-b in namespace-b.")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-client-b-in-ns-b-via-ns-selector-and-pod-selector",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"pod-name": "client-b",
								},
							},
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"ns-name": nsBName,
								},
							},
						}},
					}},
				},
			}

			policy, err = f.ClientSet.NetworkingV1().NetworkPolicies(nsA.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error creating Network Policy %v: %v", policy.ObjectMeta.Name, err)
			defer cleanupNetworkPolicy(f, policy)

			testCannotConnect(f, nsB, "client-a", service, 80)
			testCannotConnect(f, nsA, "client-b", service, 80)
			testCanConnect(f, nsB, "client-b", service, 80)
		})

		ginkgo.It("should enforce policy to allow traffic only from a pod in a different namespace based on PodSelector and NamespaceSelector [Feature:NetworkPolicy]", func() {
			nsA := f.Namespace
			nsBName := f.BaseName + "-b"
			nsB, err := f.CreateNamespace(nsBName, map[string]string{
				"ns-name": nsBName,
			})
			framework.ExpectNoError(err, "Error occurred while creating namespace-b.")

			// Wait for Server in namespaces-a to be ready
			framework.Logf("Waiting for server to come up.")
			err = e2epod.WaitForPodRunningInNamespace(f.ClientSet, podServer)
			framework.ExpectNoError(err, "Error occurred while waiting for pod status in namespace: Running.")

			// Before application of the policy, all communication should be successful.
			ginkgo.By("Creating client-a, in server's namespace, which should be able to contact the server.", func() {
				testCanConnect(f, nsA, "client-a", service, 80)
			})
			ginkgo.By("Creating client-b, in server's namespace, which should be able to contact the server.", func() {
				testCanConnect(f, nsA, "client-b", service, 80)
			})
			ginkgo.By("Creating client-a, not in server's namespace, which should be able to contact the server.", func() {
				testCanConnect(f, nsB, "client-a", service, 80)
			})
			ginkgo.By("Creating client-b, not in server's namespace, which should be able to contact the server.", func() {
				testCanConnect(f, nsB, "client-b", service, 80)
			})

			ginkgo.By("Creating a network policy for the server which allows traffic only from client-a in namespace-b.")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: nsA.Name,
					Name:      "allow-ns-b-client-a-via-namespace-pod-selector",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to the Server
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					// Allow traffic only from client-a in namespace-b
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"ns-name": nsBName,
								},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"pod-name": "client-a",
								},
							},
						}},
					}},
				},
			}

			policy, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policy.")
			defer cleanupNetworkPolicy(f, policy)

			ginkgo.By("Creating client-a, in server's namespace, which should not be able to contact the server.", func() {
				testCannotConnect(f, nsA, "client-a", service, 80)
			})
			ginkgo.By("Creating client-b, in server's namespace, which should not be able to contact the server.", func() {
				testCannotConnect(f, nsA, "client-b", service, 80)
			})
			ginkgo.By("Creating client-a, not in server's namespace, which should be able to contact the server.", func() {
				testCanConnect(f, nsB, "client-a", service, 80)
			})
			ginkgo.By("Creating client-b, not in server's namespace, which should not be able to contact the server.", func() {
				testCannotConnect(f, nsB, "client-b", service, 80)
			})
		})

		ginkgo.It("should enforce policy based on Ports [Feature:NetworkPolicy]", func() {
			ginkgo.By("Creating a network policy for the Service which allows traffic only to one port.")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-ingress-on-port-81",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply to server
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					// Allow traffic only to one port.
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						Ports: []networkingv1.NetworkPolicyPort{{
							Port: &intstr.IntOrString{IntVal: 81},
						}},
					}},
				},
			}
			policy, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer cleanupNetworkPolicy(f, policy)

			ginkgo.By("Testing pods can connect only to the port allowed by the policy.")
			testCannotConnect(f, f.Namespace, "client-a", service, 80)
			testCanConnect(f, f.Namespace, "client-b", service, 81)
		})

		ginkgo.It("should enforce multiple, stacked policies with overlapping podSelectors [Feature:NetworkPolicy]", func() {
			ginkgo.By("Creating a network policy for the Service which allows traffic only to one port.")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-ingress-on-port-80",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply to server
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					// Allow traffic only to one port.
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						Ports: []networkingv1.NetworkPolicyPort{{
							Port: &intstr.IntOrString{IntVal: 80},
						}},
					}},
				},
			}
			policy, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer cleanupNetworkPolicy(f, policy)

			ginkgo.By("Creating a network policy for the Service which allows traffic only to another port.")
			policy2 := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-ingress-on-port-81",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply to server
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					// Allow traffic only to one port.
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						Ports: []networkingv1.NetworkPolicyPort{{
							Port: &intstr.IntOrString{IntVal: 81},
						}},
					}},
				},
			}
			policy2, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy2, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer cleanupNetworkPolicy(f, policy2)

			ginkgo.By("Testing pods can connect to both ports when both policies are present.")
			testCanConnect(f, f.Namespace, "client-a", service, 80)
			testCanConnect(f, f.Namespace, "client-b", service, 81)
		})

		ginkgo.It("should support allow-all policy [Feature:NetworkPolicy]", func() {
			ginkgo.By("Creating a network policy which allows all traffic.")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-all",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Allow all traffic
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{},
					},
					Ingress: []networkingv1.NetworkPolicyIngressRule{{}},
				},
			}
			policy, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer cleanupNetworkPolicy(f, policy)

			ginkgo.By("Testing pods can connect to both ports when an 'allow-all' policy is present.")
			testCanConnect(f, f.Namespace, "client-a", service, 80)
			testCanConnect(f, f.Namespace, "client-b", service, 81)
		})

		ginkgo.It("should allow ingress access on one named port [Feature:NetworkPolicy]", func() {
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-client-a-via-named-port-ingress-rule",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to the Server
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					// Allow traffic to only one named port: "serve-80".
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						Ports: []networkingv1.NetworkPolicyPort{{
							Port: &intstr.IntOrString{Type: intstr.String, StrVal: "serve-80"},
						}},
					}},
				},
			}

			policy, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer cleanupNetworkPolicy(f, policy)

			ginkgo.By("Creating client-a which should be able to contact the server.", func() {
				testCanConnect(f, f.Namespace, "client-a", service, 80)
			})
			ginkgo.By("Creating client-b which should not be able to contact the server on port 81.", func() {
				testCannotConnect(f, f.Namespace, "client-b", service, 81)
			})
		})

		ginkgo.It("should allow ingress access from namespace on one named port [Feature:NetworkPolicy]", func() {
			nsBName := f.BaseName + "-b"
			nsB, err := f.CreateNamespace(nsBName, map[string]string{
				"ns-name": nsBName,
			})
			framework.ExpectNoError(err, "Error creating namespace %v: %v", nsBName, err)

			const allowedPort = 80
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-client-in-ns-b-via-named-port-ingress-rule",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to the Server
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					// Allow traffic to only one named port: "serve-80" from namespace-b.
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"ns-name": nsBName,
								},
							},
						}},
						Ports: []networkingv1.NetworkPolicyPort{{
							Port: &intstr.IntOrString{Type: intstr.String, StrVal: "serve-80"},
						}},
					}},
				},
			}

			policy, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error creating Network Policy %v: %v", policy.ObjectMeta.Name, err)
			defer cleanupNetworkPolicy(f, policy)

			testCannotConnect(f, f.Namespace, "client-a", service, allowedPort)
			testCanConnect(f, nsB, "client-b", service, allowedPort)
		})

		ginkgo.It("should allow egress access on one named port [Feature:NetworkPolicy]", func() {
			clientPodName := "client-a"
			protocolUDP := v1.ProtocolUDP
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-client-a-via-named-port-egress-rule",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to client-a
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": clientPodName,
						},
					},
					// Allow traffic to only one named port: "serve-80".
					Egress: []networkingv1.NetworkPolicyEgressRule{{
						Ports: []networkingv1.NetworkPolicyPort{
							{
								Port: &intstr.IntOrString{Type: intstr.String, StrVal: "serve-80"},
							},
							// Allow DNS look-ups
							{
								Protocol: &protocolUDP,
								Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
							},
						},
					}},
				},
			}

			policy, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer cleanupNetworkPolicy(f, policy)

			ginkgo.By("Creating client-a which should be able to contact the server.", func() {
				testCanConnect(f, f.Namespace, clientPodName, service, 80)
			})
			ginkgo.By("Creating client-a which should not be able to contact the server on port 81.", func() {
				testCannotConnect(f, f.Namespace, clientPodName, service, 81)
			})
		})

		ginkgo.It("should enforce updated policy [Feature:NetworkPolicy]", func() {
			const (
				clientAAllowedPort    = 80
				clientANotAllowedPort = 81
			)
			ginkgo.By("Creating a network policy for the Service which allows traffic from pod at a port")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-ingress",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply to server
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					// Allow traffic only to one port.
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"pod-name": "client-a",
								},
							},
						}},
						Ports: []networkingv1.NetworkPolicyPort{{
							Port: &intstr.IntOrString{IntVal: clientAAllowedPort},
						}},
					}},
				},
			}
			policy, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error creating Network Policy %v: %v", policy.ObjectMeta.Name, err)

			testCanConnect(f, f.Namespace, "client-a", service, clientAAllowedPort)
			e2epod.WaitForPodNotFoundInNamespace(f.ClientSet, "client-a", f.Namespace.Name, framework.PodDeleteTimeout)
			framework.ExpectNoError(err, "Expected pod to be not found.")

			testCannotConnect(f, f.Namespace, "client-b", service, clientAAllowedPort)
			e2epod.WaitForPodNotFoundInNamespace(f.ClientSet, "client-b", f.Namespace.Name, framework.PodDeleteTimeout)
			framework.ExpectNoError(err, "Expected pod to be not found.")

			testCannotConnect(f, f.Namespace, "client-a", service, clientANotAllowedPort)
			e2epod.WaitForPodNotFoundInNamespace(f.ClientSet, "client-a", f.Namespace.Name, framework.PodDeleteTimeout)
			framework.ExpectNoError(err, "Expected pod to be not found.")

			const (
				clientBAllowedPort    = 81
				clientBNotAllowedPort = 80
			)
			ginkgo.By("Updating a network policy for the Service which allows traffic from another pod at another port.")
			policy = &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-ingress",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply to server
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					// Allow traffic only to one port.
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"pod-name": "client-b",
								},
							},
						}},
						Ports: []networkingv1.NetworkPolicyPort{{
							Port: &intstr.IntOrString{IntVal: clientBAllowedPort},
						}},
					}},
				},
			}
			policy, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Update(context.TODO(), policy, metav1.UpdateOptions{})
			framework.ExpectNoError(err, "Error updating Network Policy %v: %v", policy.ObjectMeta.Name, err)
			defer cleanupNetworkPolicy(f, policy)

			testCannotConnect(f, f.Namespace, "client-b", service, clientBNotAllowedPort)
			e2epod.WaitForPodNotFoundInNamespace(f.ClientSet, "client-b", f.Namespace.Name, framework.PodDeleteTimeout)
			framework.ExpectNoError(err, "Expected pod to be not found.")

			testCannotConnect(f, f.Namespace, "client-a", service, clientBNotAllowedPort)
			testCanConnect(f, f.Namespace, "client-b", service, clientBAllowedPort)
		})

		ginkgo.It("should allow ingress access from updated namespace [Feature:NetworkPolicy]", func() {
			nsA := f.Namespace
			nsBName := f.BaseName + "-b"
			newNsBName := nsBName + "-updated"
			nsB, err := f.CreateNamespace(nsBName, map[string]string{
				"ns-name": nsBName,
			})
			framework.ExpectNoError(err, "Error creating namespace %v: %v", nsBName, err)

			const allowedPort = 80
			// Create Policy for that service that allows traffic only via namespace B
			ginkgo.By("Creating a network policy for the server which allows traffic from namespace-b.")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-ns-b-via-namespace-selector",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"ns-name": newNsBName,
								},
							},
						}},
					}},
				},
			}

			policy, err = f.ClientSet.NetworkingV1().NetworkPolicies(nsA.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error creating Network Policy %v: %v", policy.ObjectMeta.Name, err)
			defer cleanupNetworkPolicy(f, policy)

			testCannotConnect(f, nsB, "client-a", service, allowedPort)

			nsB, err = f.ClientSet.CoreV1().Namespaces().Get(context.TODO(), nsB.Name, metav1.GetOptions{})
			framework.ExpectNoError(err, "Error getting Namespace %v: %v", nsB.ObjectMeta.Name, err)

			nsB.ObjectMeta.Labels["ns-name"] = newNsBName
			nsB, err = f.ClientSet.CoreV1().Namespaces().Update(context.TODO(), nsB, metav1.UpdateOptions{})
			framework.ExpectNoError(err, "Error updating Namespace %v: %v", nsB.ObjectMeta.Name, err)

			testCanConnect(f, nsB, "client-b", service, allowedPort)
		})

		ginkgo.It("should allow ingress access from updated pod [Feature:NetworkPolicy]", func() {
			const allowedPort = 80
			ginkgo.By("Creating a network policy for the server which allows traffic from client-a-updated.")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-pod-b-via-pod-selector",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							PodSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      "pod-name",
									Operator: metav1.LabelSelectorOpDoesNotExist,
								}},
							},
						}},
					}},
				},
			}

			policy, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error creating Network Policy %v: %v", policy.ObjectMeta.Name, err)
			defer cleanupNetworkPolicy(f, policy)

			ginkgo.By(fmt.Sprintf("Creating client pod %s that should not be able to connect to %s.", "client-a", service.Name))
			// Specify RestartPolicy to OnFailure so we can check the client pod fails in the beginning and succeeds
			// after updating its label, otherwise it would not restart after the first failure.
			podClient := createNetworkClientPodWithRestartPolicy(f, f.Namespace, "client-a", service, allowedPort, v1.RestartPolicyOnFailure)
			defer func() {
				ginkgo.By(fmt.Sprintf("Cleaning up the pod %s", podClient.Name))
				if err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), podClient.Name, metav1.DeleteOptions{}); err != nil {
					framework.Failf("unable to cleanup pod %v: %v", podClient.Name, err)
				}
			}()
			// Check Container exit code as restartable Pod's Phase will be Running even when container fails.
			checkNoConnectivityByExitCode(f, f.Namespace, podClient, service)

			ginkgo.By(fmt.Sprintf("Updating client pod %s that should successfully connect to %s.", podClient.Name, service.Name))
			podClient = updatePodLabel(f, f.Namespace, podClient.Name, "replace", "/metadata/labels", map[string]string{})
			checkConnectivity(f, f.Namespace, podClient, service)
		})

		ginkgo.It("should deny ingress access to updated pod [Feature:NetworkPolicy]", func() {
			const allowedPort = 80
			ginkgo.By("Creating a network policy for the server which denies all traffic.")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "deny-ingress-via-isolated-label-selector",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "isolated",
							Operator: metav1.LabelSelectorOpExists,
						}},
					},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
					Ingress:     []networkingv1.NetworkPolicyIngressRule{},
				},
			}

			policy, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error creating Network Policy %v: %v", policy.ObjectMeta.Name, err)
			defer cleanupNetworkPolicy(f, policy)

			// Client can connect to service when the network policy doesn't apply to the server pod.
			testCanConnect(f, f.Namespace, "client-a", service, allowedPort)

			// Client cannot connect to service after updating the server pod's labels to match the network policy's selector.
			ginkgo.By(fmt.Sprintf("Updating server pod %s to be selected by network policy %s.", podServer.Name, policy.Name))
			updatePodLabel(f, f.Namespace, podServer.Name, "add", "/metadata/labels/isolated", nil)
			testCannotConnect(f, f.Namespace, "client-a", service, allowedPort)
		})

		ginkgo.It("should work with Ingress,Egress specified together [Feature:NetworkPolicy]", func() {
			const allowedPort = 80
			const notAllowedPort = 81
			protocolUDP := v1.ProtocolUDP

			nsBName := f.BaseName + "-b"
			nsB, err := f.CreateNamespace(nsBName, map[string]string{
				"ns-name": nsBName,
			})
			framework.ExpectNoError(err, "Error occurred while creating namespace-b.")

			podB, serviceB := createServerPodAndService(f, nsB, "pod-b", []int{allowedPort, notAllowedPort})
			defer cleanupServerPodAndService(f, podB, serviceB)

			// Wait for Server with Service in NS-B to be ready
			framework.Logf("Waiting for servers to be ready.")
			err = e2epod.WaitTimeoutForPodReadyInNamespace(f.ClientSet, podB.Name, nsB.Name, framework.PodStartTimeout)
			framework.ExpectNoError(err, "Error occurred while waiting for pod status in namespace: Ready.")

			ginkgo.By("Create a network policy for the server which denies both Ingress and Egress traffic.")
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ingress-egress-rule",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"ns-name": nsBName,
								},
							},
						}},
						Ports: []networkingv1.NetworkPolicyPort{{
							Port: &intstr.IntOrString{IntVal: allowedPort},
						}},
					}},
					Egress: []networkingv1.NetworkPolicyEgressRule{
						{
							Ports: []networkingv1.NetworkPolicyPort{
								// Allow DNS look-ups
								{
									Protocol: &protocolUDP,
									Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
								},
							},
						},
						{
							To: []networkingv1.NetworkPolicyPeer{
								{
									NamespaceSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"ns-name": nsBName,
										},
									},
								},
							},
							Ports: []networkingv1.NetworkPolicyPort{{
								Port: &intstr.IntOrString{IntVal: allowedPort},
							}},
						},
					},
				},
			}

			policy, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error creating Network Policy %v: %v", policy.ObjectMeta.Name, err)
			defer cleanupNetworkPolicy(f, policy)

			ginkgo.By("client-a should be able to communicate with server port 80 in namespace-b", func() {
				testCanConnect(f, f.Namespace, "client-a", serviceB, allowedPort)
			})

			ginkgo.By("client-b should be able to communicate with server port 80 in namespace-a", func() {
				testCanConnect(f, nsB, "client-b", service, allowedPort)
			})

			ginkgo.By("client-a should not be able to communicate with server port 81 in namespace-b", func() {
				testCannotConnect(f, f.Namespace, "client-a", serviceB, notAllowedPort)
			})

			ginkgo.By("client-b should not be able to communicate with server port 81 in namespace-a", func() {
				testCannotConnect(f, nsB, "client-b", service, notAllowedPort)
			})

		})

		ginkgo.It("should enforce egress policy allowing traffic to a server in a different namespace based on PodSelector and NamespaceSelector [Feature:NetworkPolicy]", func() {
			var nsBserviceA, nsBserviceB *v1.Service
			var nsBpodServerA, nsBpodServerB *v1.Pod

			nsA := f.Namespace
			nsBName := f.BaseName + "-b"
			nsB, err := f.CreateNamespace(nsBName, map[string]string{
				"ns-name": nsBName,
			})
			framework.ExpectNoError(err, "Error occurred while creating namespace-b.")

			// Creating pods and services in namespace-b
			nsBpodServerA, nsBserviceA = createServerPodAndService(f, nsB, "ns-b-server-a", []int{80})
			defer cleanupServerPodAndService(f, nsBpodServerA, nsBserviceA)
			nsBpodServerB, nsBserviceB = createServerPodAndService(f, nsB, "ns-b-server-b", []int{80})
			defer cleanupServerPodAndService(f, nsBpodServerB, nsBserviceB)

			// Wait for Server with Service in NS-A to be ready
			framework.Logf("Waiting for servers to be ready.")
			err = e2epod.WaitTimeoutForPodReadyInNamespace(f.ClientSet, podServer.Name, podServer.Namespace, framework.PodStartTimeout)
			framework.ExpectNoError(err, "Error occurred while waiting for pod status in namespace: Ready.")

			// Wait for Servers with Services in NS-B to be ready
			err = e2epod.WaitTimeoutForPodReadyInNamespace(f.ClientSet, nsBpodServerA.Name, nsBpodServerA.Namespace, framework.PodStartTimeout)
			framework.ExpectNoError(err, "Error occurred while waiting for pod status in namespace: Ready.")

			err = e2epod.WaitTimeoutForPodReadyInNamespace(f.ClientSet, nsBpodServerB.Name, nsBpodServerB.Namespace, framework.PodStartTimeout)
			framework.ExpectNoError(err, "Error occurred while waiting for pod status in namespace: Ready.")

			ginkgo.By("Creating a network policy for the server which allows traffic only to a server in different namespace.")
			protocolUDP := v1.ProtocolUDP
			policyAllowToServerInNSB := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: nsA.Name,
					Name:      "allow-to-ns-b-server-a-via-namespace-selector",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to the client
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": "client-a",
						},
					},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					// Allow traffic only to server-a in namespace-b
					Egress: []networkingv1.NetworkPolicyEgressRule{
						{
							Ports: []networkingv1.NetworkPolicyPort{
								// Allow DNS look-ups
								{
									Protocol: &protocolUDP,
									Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
								},
							},
						},
						{
							To: []networkingv1.NetworkPolicyPeer{
								{
									NamespaceSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"ns-name": nsBName,
										},
									},
									PodSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"pod-name": nsBpodServerA.ObjectMeta.Labels["pod-name"],
										},
									},
								},
							},
						},
					},
				},
			}

			policyAllowToServerInNSB, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policyAllowToServerInNSB, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policyAllowToServerInNSB.")
			defer cleanupNetworkPolicy(f, policyAllowToServerInNSB)

			ginkgo.By("Creating client-a, in 'namespace-a', which should be able to contact the server-a in namespace-b.", func() {
				testCanConnect(f, nsA, "client-a", nsBserviceA, 80)
			})
			ginkgo.By("Creating client-a, in 'namespace-a', which should not be able to contact the server-b in namespace-b.", func() {
				testCannotConnect(f, nsA, "client-a", nsBserviceB, 80)
			})
			ginkgo.By("Creating client-a, in 'namespace-a', which should not be able to contact the server in namespace-a.", func() {
				testCannotConnect(f, nsA, "client-a", service, 80)
			})
		})

		ginkgo.It("should enforce multiple ingress policies with ingress allow-all policy taking precedence [Feature:NetworkPolicy]", func() {
			ginkgo.By("Creating a network policy for the server which allows traffic only from client-b.")
			policyAllowOnlyFromClientB := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: f.Namespace.Name,
					Name:      "allow-from-client-b-pod-selector",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to the Server
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
					// Allow traffic only from "client-b"
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"pod-name": "client-b",
								},
							},
						}},
					}},
				},
			}

			policyAllowOnlyFromClientB, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policyAllowOnlyFromClientB, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policyAllowOnlyFromClientB.")
			defer cleanupNetworkPolicy(f, policyAllowOnlyFromClientB)

			ginkgo.By("Creating client-a which should not be able to contact the server.", func() {
				testCannotConnect(f, f.Namespace, "client-a", service, 80)
			})
			ginkgo.By("Creating client-b which should be able to contact the server.", func() {
				testCanConnect(f, f.Namespace, "client-b", service, 80)
			})

			ginkgo.By("Creating a network policy for the server which allows traffic from all clients.")
			policyIngressAllowAll := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					//Namespace: f.Namespace.Name,
					Name: "allow-all",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to all pods
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{},
					},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
					Ingress:     []networkingv1.NetworkPolicyIngressRule{{}},
				},
			}

			policyIngressAllowAll, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policyIngressAllowAll, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policyIngressAllowAll.")
			defer cleanupNetworkPolicy(f, policyIngressAllowAll)

			ginkgo.By("Creating client-a which should be able to contact the server.", func() {
				testCanConnect(f, f.Namespace, "client-a", service, 80)
			})
			ginkgo.By("Creating client-b which should be able to contact the server.", func() {
				testCanConnect(f, f.Namespace, "client-b", service, 80)
			})
		})

		ginkgo.It("should enforce multiple egress policies with egress allow-all policy taking precedence [Feature:NetworkPolicy]", func() {
			podServerB, serviceB := createServerPodAndService(f, f.Namespace, "server-b", []int{80})
			defer cleanupServerPodAndService(f, podServerB, serviceB)

			ginkgo.By("Waiting for pod ready", func() {
				err := e2epod.WaitTimeoutForPodReadyInNamespace(f.ClientSet, podServerB.Name, f.Namespace.Name, framework.PodStartTimeout)
				framework.ExpectNoError(err, "Error occurred while waiting for pod type: Ready.")
			})

			protocolUDP := v1.ProtocolUDP

			ginkgo.By("Creating client-a which should be able to contact the server before applying policy.", func() {
				testCanConnect(f, f.Namespace, "client-a", serviceB, 80)
			})

			ginkgo.By("Creating a network policy for the server which allows traffic only to server-a.")
			policyAllowOnlyToServerA := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: f.Namespace.Name,
					Name:      "allow-to-server-a-pod-selector",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to the "client-a"
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": "client-a",
						},
					},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					// Allow traffic only to "server-a"
					Egress: []networkingv1.NetworkPolicyEgressRule{
						{
							Ports: []networkingv1.NetworkPolicyPort{
								// Allow DNS look-ups
								{
									Protocol: &protocolUDP,
									Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
								},
							},
						},
						{
							To: []networkingv1.NetworkPolicyPeer{
								{
									PodSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"pod-name": podServerLabelSelector,
										},
									},
								},
							},
						},
					},
				},
			}
			policyAllowOnlyToServerA, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policyAllowOnlyToServerA, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policyAllowOnlyToServerA.")
			defer cleanupNetworkPolicy(f, policyAllowOnlyToServerA)

			ginkgo.By("Creating client-a which should not be able to contact the server-b.", func() {
				testCannotConnect(f, f.Namespace, "client-a", serviceB, 80)
			})
			ginkgo.By("Creating client-a which should be able to contact the server.", func() {
				testCanConnect(f, f.Namespace, "client-a", service, 80)
			})

			ginkgo.By("Creating a network policy which allows traffic to all pods.")
			policyEgressAllowAll := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-all",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to all pods
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{},
					},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					Egress:      []networkingv1.NetworkPolicyEgressRule{{}},
				},
			}

			policyEgressAllowAll, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policyEgressAllowAll, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policyEgressAllowAll.")
			defer cleanupNetworkPolicy(f, policyEgressAllowAll)

			ginkgo.By("Creating client-a which should be able to contact the server-b.", func() {
				testCanConnect(f, f.Namespace, "client-a", serviceB, 80)
			})
			ginkgo.By("Creating client-a which should be able to contact the server-a.", func() {
				testCanConnect(f, f.Namespace, "client-a", service, 80)
			})
		})

		ginkgo.It("should stop enforcing policies after they are deleted [Feature:NetworkPolicy]", func() {
			ginkgo.By("Creating a network policy for the server which denies all traffic.")
			policyDenyAll := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: f.Namespace.Name,
					Name:      "deny-all",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Deny all traffic
					PodSelector: metav1.LabelSelector{},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
					Ingress:     []networkingv1.NetworkPolicyIngressRule{},
				},
			}

			policyDenyAll, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policyDenyAll, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policyDenyAll.")

			ginkgo.By("Creating client-a which should not be able to contact the server.", func() {
				testCannotConnect(f, f.Namespace, "client-a", service, 80)
			})

			ginkgo.By("Creating a network policy for the server which allows traffic only from client-a.")
			policyAllowFromClientA := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: f.Namespace.Name,
					Name:      "allow-from-client-a-pod-selector",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to the Server
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
					// Allow traffic from "client-a"
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						From: []networkingv1.NetworkPolicyPeer{{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"pod-name": "client-a",
								},
							},
						}},
					}},
				},
			}

			policyAllowFromClientA, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policyAllowFromClientA, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policyAllowFromClientA.")

			ginkgo.By("Creating client-a which should be able to contact the server.", func() {
				testCanConnect(f, f.Namespace, "client-a", service, 80)
			})

			ginkgo.By("Deleting the network policy allowing traffic from client-a")
			cleanupNetworkPolicy(f, policyAllowFromClientA)

			ginkgo.By("Creating client-a which should not be able to contact the server.", func() {
				testCannotConnect(f, f.Namespace, "client-a", service, 80)
			})

			ginkgo.By("Deleting the network policy denying all traffic.")
			cleanupNetworkPolicy(f, policyDenyAll)

			ginkgo.By("Creating client-a which should be able to contact the server.", func() {
				testCanConnect(f, f.Namespace, "client-a", service, 80)
			})

		})

		ginkgo.It("should allow egress access to server in CIDR block [Feature:NetworkPolicy]", func() {
			var serviceB *v1.Service
			var podServerB *v1.Pod

			protocolUDP := v1.ProtocolUDP

			// Getting podServer's status to get podServer's IP, to create the CIDR
			podServerStatus, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), podServer.Name, metav1.GetOptions{})
			if err != nil {
				framework.ExpectNoError(err, "Error occurred while getting pod status.")
			}

			podServerCIDR := fmt.Sprintf("%s/32", podServerStatus.Status.PodIP)

			// Creating pod-b and service-b
			podServerB, serviceB = createServerPodAndService(f, f.Namespace, "pod-b", []int{80})
			ginkgo.By("Waiting for pod-b to be ready", func() {
				err := e2epod.WaitTimeoutForPodReadyInNamespace(f.ClientSet, podServerB.Name, f.Namespace.Name, framework.PodStartTimeout)
				framework.ExpectNoError(err, "Error occurred while waiting for pod type: Ready.")
			})
			defer cleanupServerPodAndService(f, podServerB, serviceB)

			// Wait for podServerB with serviceB to be ready
			err = e2epod.WaitForPodRunningInNamespace(f.ClientSet, podServerB)
			framework.ExpectNoError(err, "Error occurred while waiting for pod status in namespace: Running.")

			ginkgo.By("Creating client-a which should be able to contact the server-b.", func() {
				testCanConnect(f, f.Namespace, "client-a", serviceB, 80)
			})

			policyAllowCIDR := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: f.Namespace.Name,
					Name:      "allow-client-a-via-cidr-egress-rule",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to the Server
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": "client-a",
						},
					},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					// Allow traffic to only one CIDR block.
					Egress: []networkingv1.NetworkPolicyEgressRule{
						{
							Ports: []networkingv1.NetworkPolicyPort{
								// Allow DNS look-ups
								{
									Protocol: &protocolUDP,
									Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
								},
							},
						},
						{
							To: []networkingv1.NetworkPolicyPeer{
								{
									IPBlock: &networkingv1.IPBlock{
										CIDR: podServerCIDR,
									},
								},
							},
						},
					},
				},
			}

			policyAllowCIDR, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policyAllowCIDR, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policyAllowCIDR.")
			defer cleanupNetworkPolicy(f, policyAllowCIDR)

			ginkgo.By("Creating client-a which should not be able to contact the server-b.", func() {
				testCannotConnect(f, f.Namespace, "client-a", serviceB, 80)
			})
			ginkgo.By("Creating client-a which should be able to contact the server.", func() {
				testCanConnect(f, f.Namespace, "client-a", service, 80)
			})
		})

		ginkgo.It("should enforce except clause while egress access to server in CIDR block [Feature:NetworkPolicy]", func() {
			protocolUDP := v1.ProtocolUDP

			// Getting podServer's status to get podServer's IP, to create the CIDR with except clause
			podServerStatus, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), podServer.Name, metav1.GetOptions{})
			if err != nil {
				framework.ExpectNoError(err, "Error occurred while getting pod status.")
			}

			podServerAllowCIDR := fmt.Sprintf("%s/24", podServerStatus.Status.PodIP)
			// Exclude podServer's IP with an Except clause
			podServerExceptList := []string{fmt.Sprintf("%s/32", podServerStatus.Status.PodIP)}

			// client-a can connect to server prior to applying the NetworkPolicy
			ginkgo.By("Creating client-a which should be able to contact the server.", func() {
				testCanConnect(f, f.Namespace, "client-a", service, 80)
			})

			policyAllowCIDRWithExcept := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: f.Namespace.Name,
					Name:      "deny-client-a-via-except-cidr-egress-rule",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to the client.
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": "client-a",
						},
					},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					// Allow traffic to only one CIDR block except subnet which includes Server.
					Egress: []networkingv1.NetworkPolicyEgressRule{
						{
							Ports: []networkingv1.NetworkPolicyPort{
								// Allow DNS look-ups
								{
									Protocol: &protocolUDP,
									Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
								},
							},
						},
						{
							To: []networkingv1.NetworkPolicyPeer{
								{
									IPBlock: &networkingv1.IPBlock{
										CIDR:   podServerAllowCIDR,
										Except: podServerExceptList,
									},
								},
							},
						},
					},
				},
			}

			policyAllowCIDRWithExcept, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policyAllowCIDRWithExcept, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policyAllowCIDRWithExcept.")
			defer cleanupNetworkPolicy(f, policyAllowCIDRWithExcept)

			ginkgo.By("Creating client-a which should no longer be able to contact the server.", func() {
				testCannotConnect(f, f.Namespace, "client-a", service, 80)
			})
		})

		ginkgo.It("should ensure an IP overlapping both IPBlock.CIDR and IPBlock.Except is allowed [Feature:NetworkPolicy]", func() {
			protocolUDP := v1.ProtocolUDP

			// Getting podServer's status to get podServer's IP, to create the CIDR with except clause
			podServerStatus, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), podServer.Name, metav1.GetOptions{})
			if err != nil {
				framework.ExpectNoError(err, "Error occurred while getting pod status.")
			}

			podServerAllowCIDR := fmt.Sprintf("%s/24", podServerStatus.Status.PodIP)
			podServerIP := fmt.Sprintf("%s/32", podServerStatus.Status.PodIP)
			// Exclude podServer's IP with an Except clause
			podServerExceptList := []string{podServerIP}

			// Create NetworkPolicy which blocks access to podServer with except clause.
			policyAllowCIDRWithExceptServerPod := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: f.Namespace.Name,
					Name:      "deny-client-a-via-except-cidr-egress-rule",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to the client.
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": "client-a",
						},
					},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					// Allow traffic to only one CIDR block except subnet which includes Server.
					Egress: []networkingv1.NetworkPolicyEgressRule{
						{
							Ports: []networkingv1.NetworkPolicyPort{
								// Allow DNS look-ups
								{
									Protocol: &protocolUDP,
									Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
								},
							},
						},
						{
							To: []networkingv1.NetworkPolicyPeer{
								{
									IPBlock: &networkingv1.IPBlock{
										CIDR:   podServerAllowCIDR,
										Except: podServerExceptList,
									},
								},
							},
						},
					},
				},
			}

			policyAllowCIDRWithExceptServerPodObj, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policyAllowCIDRWithExceptServerPod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policyAllowCIDRWithExceptServerPod.")

			ginkgo.By("Creating client-a which should not be able to contact the server.", func() {
				testCannotConnect(f, f.Namespace, "client-a", service, 80)
			})

			// Create NetworkPolicy which allows access to the podServer using podServer's IP in allow CIDR.
			policyAllowCIDRServerPod := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: f.Namespace.Name,
					Name:      "allow-client-a-via-cidr-egress-rule",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy to the client.
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": "client-a",
						},
					},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					// Allow traffic to only one CIDR block which includes Server.
					Egress: []networkingv1.NetworkPolicyEgressRule{
						{
							Ports: []networkingv1.NetworkPolicyPort{
								// Allow DNS look-ups
								{
									Protocol: &protocolUDP,
									Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
								},
							},
						},
						{
							To: []networkingv1.NetworkPolicyPeer{
								{
									IPBlock: &networkingv1.IPBlock{
										CIDR: podServerIP,
									},
								},
							},
						},
					},
				},
			}

			policyAllowCIDRServerPod, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policyAllowCIDRServerPod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policyAllowCIDRServerPod.")
			defer cleanupNetworkPolicy(f, policyAllowCIDRServerPod)

			ginkgo.By("Creating client-a which should now be able to contact the server.", func() {
				testCanConnect(f, f.Namespace, "client-a", service, 80)
			})

			ginkgo.By("Deleting the network policy with except podServer IP which disallows access to podServer.")
			cleanupNetworkPolicy(f, policyAllowCIDRWithExceptServerPodObj)

			ginkgo.By("Creating client-a which should still be able to contact the server after deleting the network policy with except clause.", func() {
				testCanConnect(f, f.Namespace, "client-a", service, 80)
			})

			// Recreate the NetworkPolicy which contains the podServer's IP in the except list.
			policyAllowCIDRWithExceptServerPod, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policyAllowCIDRWithExceptServerPod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policyAllowCIDRWithExceptServerPod.")
			defer cleanupNetworkPolicy(f, policyAllowCIDRWithExceptServerPod)

			ginkgo.By("Creating client-a which should still be able to contact the server after recreating the network policy with except clause.", func() {
				testCanConnect(f, f.Namespace, "client-a", service, 80)
			})

		})

		ginkgo.It("should enforce policies to check ingress and egress policies can be controlled independently based on PodSelector [Feature:NetworkPolicy]", func() {
			var serviceA, serviceB *v1.Service
			var podA, podB *v1.Pod
			var err error

			protocolUDP := v1.ProtocolUDP

			// Before applying policy, communication should be successful between pod-a and pod-b
			podA, serviceA = createServerPodAndService(f, f.Namespace, "pod-a", []int{80})
			ginkgo.By("Waiting for pod-a to be ready", func() {
				err := e2epod.WaitTimeoutForPodReadyInNamespace(f.ClientSet, podA.Name, f.Namespace.Name, framework.PodStartTimeout)
				framework.ExpectNoError(err, "Error occurred while waiting for pod type: Ready.")
			})
			ginkgo.By("Creating client pod-b which should be able to contact the server pod-a.", func() {
				testCanConnect(f, f.Namespace, "pod-b", serviceA, 80)
			})
			cleanupServerPodAndService(f, podA, serviceA)

			podB, serviceB = createServerPodAndService(f, f.Namespace, "pod-b", []int{80})
			ginkgo.By("Waiting for pod-b to be ready", func() {
				err := e2epod.WaitTimeoutForPodReadyInNamespace(f.ClientSet, podB.Name, f.Namespace.Name, framework.PodStartTimeout)
				framework.ExpectNoError(err, "Error occurred while waiting for pod type: Ready.")
			})
			ginkgo.By("Creating client pod-a which should be able to contact the server pod-b.", func() {
				testCanConnect(f, f.Namespace, "pod-a", serviceB, 80)
			})

			ginkgo.By("Creating a network policy for pod-a which allows Egress traffic to pod-b.")
			policyAllowToPodB := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: f.Namespace.Name,
					Name:      "allow-pod-a-to-pod-b-using-pod-selector",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy on pod-a
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": "pod-a",
						},
					},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					// Allow traffic to server on pod-b
					Egress: []networkingv1.NetworkPolicyEgressRule{
						{
							Ports: []networkingv1.NetworkPolicyPort{
								// Allow DNS look-ups
								{
									Protocol: &protocolUDP,
									Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
								},
							},
						},
						{
							To: []networkingv1.NetworkPolicyPeer{
								{
									PodSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"pod-name": "pod-b",
										},
									},
								},
							},
						},
					},
				},
			}

			policyAllowToPodB, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policyAllowToPodB, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policyAllowToPodB.")
			defer cleanupNetworkPolicy(f, policyAllowToPodB)

			ginkgo.By("Creating a network policy for pod-a that denies traffic from pod-b.")
			policyDenyFromPodB := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: f.Namespace.Name,
					Name:      "deny-pod-b-to-pod-a-pod-selector",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply this policy on the server on pod-a
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": "pod-a",
						},
					},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
					// Deny traffic from all pods, including pod-b
					Ingress: []networkingv1.NetworkPolicyIngressRule{},
				},
			}

			policyDenyFromPodB, err = f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policyDenyFromPodB, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Error occurred while creating policy: policyDenyFromPodB.")
			defer cleanupNetworkPolicy(f, policyDenyFromPodB)

			ginkgo.By("Creating client pod-a which should be able to contact the server pod-b.", func() {
				testCanConnect(f, f.Namespace, "pod-a", serviceB, 80)
			})
			cleanupServerPodAndService(f, podB, serviceB)

			// Creating server pod with label "pod-name": "pod-a" to deny traffic from client pod with label "pod-name": "pod-b"
			podA, serviceA = createServerPodAndService(f, f.Namespace, "pod-a", []int{80})
			ginkgo.By("Waiting for pod-a to be ready", func() {
				err := e2epod.WaitTimeoutForPodReadyInNamespace(f.ClientSet, podA.Name, f.Namespace.Name, framework.PodStartTimeout)
				framework.ExpectNoError(err, "Error occurred while waiting for pod type: Ready.")
			})

			ginkgo.By("Creating client pod-b which should be able to contact the server pod-a.", func() {
				testCannotConnect(f, f.Namespace, "pod-b", serviceA, 80)
			})
			cleanupServerPodAndService(f, podA, serviceA)
		})
		ginkgo.It("should not allow access by TCP when a policy specifies only SCTP [Feature:NetworkPolicy] [Feature:SCTP]", func() {
			ginkgo.By("getting the state of the sctp module on nodes")
			nodes, err := e2enode.GetReadySchedulableNodes(f.ClientSet)
			framework.ExpectNoError(err)
			sctpLoadedAtStart := CheckSCTPModuleLoadedOnNodes(f, nodes)

			ginkgo.By("Creating a network policy for the server which allows traffic only via SCTP on port 80.")
			protocolSCTP := v1.ProtocolSCTP
			policy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-only-sctp-ingress-on-port-80",
				},
				Spec: networkingv1.NetworkPolicySpec{
					// Apply to server
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pod-name": podServerLabelSelector,
						},
					},
					// Allow traffic only via SCTP on port 80 .
					Ingress: []networkingv1.NetworkPolicyIngressRule{{
						Ports: []networkingv1.NetworkPolicyPort{{
							Port:     &intstr.IntOrString{IntVal: 80},
							Protocol: &protocolSCTP,
						}},
					}},
				},
			}
			appliedPolicy, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).Create(context.TODO(), policy, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer cleanupNetworkPolicy(f, appliedPolicy)

			ginkgo.By("Testing pods cannot connect on port 80 anymore when not using SCTP as protocol.")
			testCannotConnect(f, f.Namespace, "client-a", service, 80)

			ginkgo.By("validating sctp module is still not loaded")
			sctpLoadedAtEnd := CheckSCTPModuleLoadedOnNodes(f, nodes)
			if !sctpLoadedAtStart && sctpLoadedAtEnd {
				framework.Failf("The state of the sctp module has changed due to the test case")
			}
		})
	})
})

func testCanConnect(f *framework.Framework, ns *v1.Namespace, podName string, service *v1.Service, targetPort int) {
	ginkgo.By(fmt.Sprintf("Creating client pod %s that should successfully connect to %s.", podName, service.Name))
	podClient := createNetworkClientPod(f, ns, podName, service, targetPort)
	defer func() {
		ginkgo.By(fmt.Sprintf("Cleaning up the pod %s", podClient.Name))
		if err := f.ClientSet.CoreV1().Pods(ns.Name).Delete(context.TODO(), podClient.Name, metav1.DeleteOptions{}); err != nil {
			framework.Failf("unable to cleanup pod %v: %v", podClient.Name, err)
		}
	}()
	checkConnectivity(f, ns, podClient, service)
}

func testCannotConnect(f *framework.Framework, ns *v1.Namespace, podName string, service *v1.Service, targetPort int) {
	ginkgo.By(fmt.Sprintf("Creating client pod %s that should not be able to connect to %s.", podName, service.Name))
	podClient := createNetworkClientPod(f, ns, podName, service, targetPort)
	defer func() {
		ginkgo.By(fmt.Sprintf("Cleaning up the pod %s", podClient.Name))
		if err := f.ClientSet.CoreV1().Pods(ns.Name).Delete(context.TODO(), podClient.Name, metav1.DeleteOptions{}); err != nil {
			framework.Failf("unable to cleanup pod %v: %v", podClient.Name, err)
		}
	}()
	checkNoConnectivity(f, ns, podClient, service)
}

func checkConnectivity(f *framework.Framework, ns *v1.Namespace, podClient *v1.Pod, service *v1.Service) {
	framework.Logf("Waiting for %s to complete.", podClient.Name)
	err := e2epod.WaitForPodNoLongerRunningInNamespace(f.ClientSet, podClient.Name, ns.Name)
	framework.ExpectNoError(err, "Pod did not finish as expected.")

	framework.Logf("Waiting for %s to complete.", podClient.Name)
	err = e2epod.WaitForPodSuccessInNamespace(f.ClientSet, podClient.Name, ns.Name)
	if err != nil {
		pods, policies, logs := collectPodsAndNetworkPolicies(f, podClient)
		framework.Failf("Pod %s should be able to connect to service %s, but was not able to connect.\nPod logs:\n%s\n\n Current NetworkPolicies:\n\t%v\n\n Pods:\n\t%v\n\n", podClient.Name, service.Name, logs, policies.Items, pods)

		// Dump debug information for the test namespace.
		framework.DumpDebugInfo(f.ClientSet, f.Namespace.Name)
	}
}

func checkNoConnectivity(f *framework.Framework, ns *v1.Namespace, podClient *v1.Pod, service *v1.Service) {
	framework.Logf("Waiting for %s to complete.", podClient.Name)
	err := e2epod.WaitForPodSuccessInNamespace(f.ClientSet, podClient.Name, ns.Name)

	// We expect an error here since it's a cannot connect test.
	// Dump debug information if the error was nil.
	if err == nil {
		pods, policies, logs := collectPodsAndNetworkPolicies(f, podClient)
		framework.Failf("Pod %s should not be able to connect to service %s, but was able to connect.\nPod logs:\n%s\n\n Current NetworkPolicies:\n\t%v\n\n Pods:\n\t %v\n\n", podClient.Name, service.Name, logs, policies.Items, pods)

		// Dump debug information for the test namespace.
		framework.DumpDebugInfo(f.ClientSet, f.Namespace.Name)
	}
}

func checkNoConnectivityByExitCode(f *framework.Framework, ns *v1.Namespace, podClient *v1.Pod, service *v1.Service) {
	err := e2epod.WaitForPodCondition(f.ClientSet, ns.Name, podClient.Name, "terminated", framework.PodStartTimeout, func(pod *v1.Pod) (bool, error) {
		statuses := pod.Status.ContainerStatuses
		if len(statuses) == 0 || statuses[0].State.Terminated == nil {
			return false, nil
		}
		if statuses[0].State.Terminated.ExitCode != 0 {
			return true, fmt.Errorf("pod %q container exited with code: %d", podClient.Name, statuses[0].State.Terminated.ExitCode)
		}
		return true, nil
	})
	// We expect an error here since it's a cannot connect test.
	// Dump debug information if the error was nil.
	if err == nil {
		pods, policies, logs := collectPodsAndNetworkPolicies(f, podClient)
		framework.Failf("Pod %s should not be able to connect to service %s, but was able to connect.\nPod logs:\n%s\n\n Current NetworkPolicies:\n\t%v\n\n Pods:\n\t%v\n\n", podClient.Name, service.Name, logs, policies.Items, pods)

		// Dump debug information for the test namespace.
		framework.DumpDebugInfo(f.ClientSet, f.Namespace.Name)
	}
}

func collectPodsAndNetworkPolicies(f *framework.Framework, podClient *v1.Pod) ([]string, *networkingv1.NetworkPolicyList, string) {
	// Collect pod logs when we see a failure.
	logs, logErr := e2epod.GetPodLogs(f.ClientSet, f.Namespace.Name, podClient.Name, "client")
	if logErr != nil && apierrors.IsNotFound(logErr) {
		// Pod may have already been removed; try to get previous pod logs
		logs, logErr = e2epod.GetPreviousPodLogs(f.ClientSet, f.Namespace.Name, podClient.Name, fmt.Sprintf("%s-container", podClient.Name))
	}
	if logErr != nil {
		framework.Logf("Error getting container logs: %s", logErr)
	}

	// Collect current NetworkPolicies applied in the test namespace.
	policies, err := f.ClientSet.NetworkingV1().NetworkPolicies(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		framework.Logf("error getting current NetworkPolicies for %s namespace: %s", f.Namespace.Name, err)
	}
	// Collect the list of pods running in the test namespace.
	podsInNS, err := e2epod.GetPodsInNamespace(f.ClientSet, f.Namespace.Name, map[string]string{})
	if err != nil {
		framework.Logf("error getting pods for %s namespace: %s", f.Namespace.Name, err)
	}
	pods := []string{}
	for _, p := range podsInNS {
		pods = append(pods, fmt.Sprintf("Pod: %s, Status: %s\n", p.Name, p.Status.String()))
	}
	return pods, policies, logs
}

// Create a server pod with a listening container for each port in ports[].
// Will also assign a pod label with key: "pod-name" and label set to the given podName for later use by the network
// policy.
func createServerPodAndService(f *framework.Framework, namespace *v1.Namespace, podName string, ports []int) (*v1.Pod, *v1.Service) {
	// Because we have a variable amount of ports, we'll first loop through and generate our Containers for our pod,
	// and ServicePorts.for our Service.
	containers := []v1.Container{}
	servicePorts := []v1.ServicePort{}
	for _, port := range ports {
		// Build the containers for the server pod.
		containers = append(containers, v1.Container{
			Name:  fmt.Sprintf("%s-container-%d", podName, port),
			Image: imageutils.GetE2EImage(imageutils.Agnhost),
			Args:  []string{"porter"},
			Env: []v1.EnvVar{
				{
					Name:  fmt.Sprintf("SERVE_PORT_%d", port),
					Value: "foo",
				},
			},
			Ports: []v1.ContainerPort{
				{
					ContainerPort: int32(port),
					Name:          fmt.Sprintf("serve-%d", port),
				},
			},
			ReadinessProbe: &v1.Probe{
				Handler: v1.Handler{
					HTTPGet: &v1.HTTPGetAction{
						Path: "/",
						Port: intstr.IntOrString{
							IntVal: int32(port),
						},
						Scheme: v1.URISchemeHTTP,
					},
				},
			},
		})

		// Build the Service Ports for the service.
		servicePorts = append(servicePorts, v1.ServicePort{
			Name:       fmt.Sprintf("%s-%d", podName, port),
			Port:       int32(port),
			TargetPort: intstr.FromInt(port),
		})
	}

	ginkgo.By(fmt.Sprintf("Creating a server pod %s in namespace %s", podName, namespace.Name))
	pod, err := f.ClientSet.CoreV1().Pods(namespace.Name).Create(context.TODO(), &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: podName + "-",
			Labels: map[string]string{
				"pod-name": podName,
			},
		},
		Spec: v1.PodSpec{
			Containers:    containers,
			RestartPolicy: v1.RestartPolicyNever,
		},
	}, metav1.CreateOptions{})
	framework.ExpectNoError(err)
	framework.Logf("Created pod %v", pod.ObjectMeta.Name)

	svcName := fmt.Sprintf("svc-%s", podName)
	ginkgo.By(fmt.Sprintf("Creating a service %s for pod %s in namespace %s", svcName, podName, namespace.Name))
	svc, err := f.ClientSet.CoreV1().Services(namespace.Name).Create(context.TODO(), &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: svcName,
		},
		Spec: v1.ServiceSpec{
			Ports: servicePorts,
			Selector: map[string]string{
				"pod-name": podName,
			},
		},
	}, metav1.CreateOptions{})
	framework.ExpectNoError(err)
	framework.Logf("Created service %s", svc.Name)

	return pod, svc
}

func cleanupServerPodAndService(f *framework.Framework, pod *v1.Pod, service *v1.Service) {
	ginkgo.By("Cleaning up the server.")
	if err := f.ClientSet.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{}); err != nil {
		framework.Failf("unable to cleanup pod %v: %v", pod.Name, err)
	}
	ginkgo.By("Cleaning up the server's service.")
	if err := f.ClientSet.CoreV1().Services(service.Namespace).Delete(context.TODO(), service.Name, metav1.DeleteOptions{}); err != nil {
		framework.Failf("unable to cleanup svc %v: %v", service.Name, err)
	}
}

// Create a client pod which will attempt a netcat to the provided service, on the specified port.
// This client will attempt a one-shot connection, then die, without restarting the pod.
// Test can then be asserted based on whether the pod quit with an error or not.
func createNetworkClientPod(f *framework.Framework, namespace *v1.Namespace, podName string, targetService *v1.Service, targetPort int) *v1.Pod {
	return createNetworkClientPodWithRestartPolicy(f, namespace, podName, targetService, targetPort, v1.RestartPolicyNever)
}

// Create a client pod which will attempt a netcat to the provided service, on the specified port.
// It is similar to createNetworkClientPod but supports specifying RestartPolicy.
func createNetworkClientPodWithRestartPolicy(f *framework.Framework, namespace *v1.Namespace, podName string, targetService *v1.Service, targetPort int, restartPolicy v1.RestartPolicy) *v1.Pod {
	pod, err := f.ClientSet.CoreV1().Pods(namespace.Name).Create(context.TODO(), &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: podName + "-",
			Labels: map[string]string{
				"pod-name": podName,
			},
		},
		Spec: v1.PodSpec{
			RestartPolicy: restartPolicy,
			Containers: []v1.Container{
				{
					Name:  "client",
					Image: imageutils.GetE2EImage(imageutils.BusyBox),
					Args: []string{
						"/bin/sh",
						"-c",
						fmt.Sprintf("for i in $(seq 1 5); do nc -vz -w 8 %s.%s %d && exit 0 || sleep 1; done; exit 1",
							targetService.Name, targetService.Namespace, targetPort),
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	framework.ExpectNoError(err)

	return pod
}

// Patch pod with a map value
func updatePodLabel(f *framework.Framework, namespace *v1.Namespace, podName string, patchOperation string, patchPath string, patchValue map[string]string) *v1.Pod {
	type patchMapValue struct {
		Op    string            `json:"op"`
		Path  string            `json:"path"`
		Value map[string]string `json:"value,omitempty"`
	}
	payload := []patchMapValue{{
		Op:    patchOperation,
		Path:  patchPath,
		Value: patchValue,
	}}
	payloadBytes, err := json.Marshal(payload)
	framework.ExpectNoError(err)

	pod, err := f.ClientSet.CoreV1().Pods(namespace.Name).Patch(context.TODO(), podName, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
	framework.ExpectNoError(err)

	return pod
}

func cleanupNetworkPolicy(f *framework.Framework, policy *networkingv1.NetworkPolicy) {
	ginkgo.By("Cleaning up the policy.")
	if err := f.ClientSet.NetworkingV1().NetworkPolicies(policy.Namespace).Delete(context.TODO(), policy.Name, metav1.DeleteOptions{}); err != nil {
		framework.Failf("unable to cleanup policy %v: %v", policy.Name, err)
	}
}

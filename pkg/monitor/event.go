package monitor

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

func startEventMonitoring(ctx context.Context, m Recorder, client kubernetes.Interface) {
	reMatchFirstQuote := regexp.MustCompile(`"([^"]+)"( in (\d+(\.\d+)?(s|ms)$))?`)

	go func() {
		// Track our last observed resource version from each event, used to re-establish the watch
		// when it's requested rv gets too old for the server. (which may happen when apiservers cycle)
		var rv string
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// filter out events written "now" but with significantly older start times (events
			// created in test jobs are the most common)
			significantlyBeforeNow := time.Now().UTC().Add(-15 * time.Minute)

			// Doing our own List + Watch here, cannot use an Informer as it will group similar events, when we need
			// each individual.

			if rv == "" {
				events, err := client.CoreV1().Events("").List(ctx, metav1.ListOptions{Limit: 1})
				if err != nil {
					continue
				}
				rv = events.ResourceVersion
				fmt.Printf("Using initial resource version from event list: %s\n", rv)
				for i := range events.Items {
					m.RecordResource("events", &events.Items[i])
				}
			} else {
				// Re-use the last resource version we observed.
				fmt.Printf("Using last observed resource version: %s\n", rv)
			}

			for expired := false; !expired; {
				w, err := client.CoreV1().Events("").Watch(ctx, metav1.ListOptions{ResourceVersion: rv})
				if err != nil {
					if errors.IsResourceExpired(err) {
						break
					}
					continue
				}
				w = watch.Filter(w, func(in watch.Event) (watch.Event, bool) {
					// TODO: gathering all events results in a 4x increase in e2e.log size, but is is
					//       valuable enough to gather that the cost is worth it
					// return in, filterToSystemNamespaces(in.Object)
					return in, true
				})
				func() {
					defer w.Stop()
					for event := range w.ResultChan() {
						switch event.Type {
						case watch.Added, watch.Modified:
							obj, ok := event.Object.(*corev1.Event)
							if !ok {
								continue
							}
							// Record the observed rv version, re-used if we lose our watch connection and re-establish
							// to prevent missed events.
							rv = obj.ResourceVersion
							m.RecordResource("events", obj)

							t := obj.LastTimestamp.Time
							if t.IsZero() {
								t = obj.EventTime.Time
							}
							if t.IsZero() {
								t = obj.CreationTimestamp.Time
							}
							if t.Before(significantlyBeforeNow) {
								break
							}

							message := obj.Message
							if obj.Count > 1 {
								message += fmt.Sprintf(" (%d times)", obj.Count)
							}

							if obj.InvolvedObject.Kind == "Node" {
								if node, err := client.CoreV1().Nodes().Get(ctx, obj.InvolvedObject.Name, metav1.GetOptions{}); err == nil {
									message = fmt.Sprintf("roles/%s %s", nodeRoles(node), message)
								}
							}

							// special case some very common events
							switch obj.Reason {
							case "":
							case "Scheduled":
								if obj.InvolvedObject.Kind == "Pod" {
									if strings.HasPrefix(message, "Successfully assigned ") {
										if i := strings.Index(message, " to "); i != -1 {
											node := message[i+4:]
											message = fmt.Sprintf("node/%s reason/%s", node, obj.Reason)
											break
										}
									}
								}
								message = fmt.Sprintf("reason/%s %s", obj.Reason, message)
							case "Started", "Created", "Killing":
								if obj.InvolvedObject.Kind == "Pod" {
									if containerName, ok := eventForContainer(obj.InvolvedObject.FieldPath); ok {
										message = fmt.Sprintf("container/%s reason/%s", containerName, obj.Reason)
										break
									}
								}
								message = fmt.Sprintf("reason/%s %s", obj.Reason, message)
							case "Pulling", "Pulled":
								if obj.InvolvedObject.Kind == "Pod" {
									if containerName, ok := eventForContainer(obj.InvolvedObject.FieldPath); ok {
										if m := reMatchFirstQuote.FindStringSubmatch(obj.Message); m != nil {
											if len(m) > 3 {
												if d, err := time.ParseDuration(m[3]); err == nil {
													message = fmt.Sprintf("container/%s reason/%s duration/%.3fs image/%s", containerName, obj.Reason, d.Seconds(), m[1])
													break
												}
											}
											message = fmt.Sprintf("container/%s reason/%s image/%s", containerName, obj.Reason, m[1])
											break
										}
									}
								}
								message = fmt.Sprintf("reason/%s %s", obj.Reason, message)
							default:
								message = fmt.Sprintf("reason/%s %s", obj.Reason, message)
							}
							condition := monitorapi.Condition{
								Level:   monitorapi.Info,
								Locator: locateEvent(obj),
								Message: message,
							}
							if obj.Type == corev1.EventTypeWarning {
								condition.Level = monitorapi.Warning
							}
							m.RecordAt(t, condition)
						case watch.Error:
							var message string
							if status, ok := event.Object.(*metav1.Status); ok {
								if err := errors.FromObject(status); err != nil && errors.IsResourceExpired(err) {
									expired = true
									return
								}
								message = status.Message
							} else {
								message = fmt.Sprintf("event object was not a Status: %T", event.Object)
							}
							m.Record(monitorapi.Condition{
								Level:   monitorapi.Info,
								Locator: "kube-apiserver",
								Message: fmt.Sprintf("received an error while watching events: %s", message),
							})
							return
						default:
						}
					}
				}()
			}
		}
	}()
}

func eventForContainer(fieldPath string) (string, bool) {
	if !strings.HasSuffix(fieldPath, "}") {
		return "", false
	}
	fieldPath = strings.TrimSuffix(fieldPath, "}")
	switch {
	case strings.HasPrefix(fieldPath, "spec.containers{"):
		return strings.TrimPrefix(fieldPath, "spec.containers{"), true
	case strings.HasPrefix(fieldPath, "spec.initContainers{"):
		return strings.TrimPrefix(fieldPath, "spec.initContainers{"), true
	default:
		return "", false
	}
}

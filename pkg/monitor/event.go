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

// trivial change to trigger CI

func startEventMonitoring(ctx context.Context, m Recorder, client kubernetes.Interface) {
	reMatchFirstQuote := regexp.MustCompile(`"([^"]+)"( in (\d+(\.\d+)?(s|ms)$))?`)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// filter out events written "now" but with significantly older start times (events
			// created in test jobs are the most common)
			significantlyBeforeNow := time.Now().UTC().Add(-15 * time.Minute)

			events, err := client.CoreV1().Events("").List(ctx, metav1.ListOptions{Limit: 1})
			if err != nil {
				continue
			}
			rv := events.ResourceVersion

			for i := range events.Items {
				m.RecordResource("events", &events.Items[i])
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
							m.RecordResource("events", obj)

							// Temporary hack by dgoodwin, we're missing events here that show up later in
							// gather-extra/events.json. Adding some output to see if we can isolate what we saw
							// and where it might have been filtered out.
							osEvent := false
							if obj.Reason == "OSUpdateStaged" || obj.Reason == "OSUpdateStarted" {
								osEvent = true
								fmt.Printf("Watch received OS update event: %s - %s - %s\n",
									obj.Reason, obj.InvolvedObject.Name, obj.LastTimestamp.Format(time.RFC3339))

							}
							t := obj.LastTimestamp.Time
							if t.IsZero() {
								t = obj.EventTime.Time
							}
							if t.IsZero() {
								t = obj.CreationTimestamp.Time
							}
							if t.Before(significantlyBeforeNow) {
								if osEvent {
									fmt.Printf("OS update event filtered for being too old: %s - %s - %s (now: %s)\n",
										obj.Reason, obj.InvolvedObject.Name, obj.LastTimestamp.Format(time.RFC3339),
										time.Now().Format(time.RFC3339))
								}
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

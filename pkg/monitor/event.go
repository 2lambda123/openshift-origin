package monitor

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

func startEventMonitoring(ctx context.Context, m Recorder, client kubernetes.Interface) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			events, err := client.Core().Events("").List(metav1.ListOptions{Limit: 1})
			if err != nil {
				continue
			}
			rv := events.ResourceVersion
			for {
				w, err := client.Core().Events("").Watch(metav1.ListOptions{ResourceVersion: rv})
				if err != nil {
					if errors.IsResourceExpired(err) {
						break
					}
					continue
				}
				w = watch.Filter(w, func(in watch.Event) (watch.Event, bool) {
					return in, filterToSystemNamespaces(in.Object)
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
							condition := Condition{
								Level:   Info,
								Locator: locateEvent(obj),
								// make sure the message ends up on a single line.  Something about the way we collect logs wants this.
								Message: strings.Replace(obj.Message, "\n", "\\n", -1) + fmt.Sprintf(" count(%d)", +obj.Count),
							}
							if obj.Type == corev1.EventTypeWarning {
								condition.Level = Warning
							}
							m.Record(condition)
						case watch.Error:
							var message string
							if status, ok := event.Object.(*metav1.Status); ok {
								message = status.Message
							} else {
								message = fmt.Sprintf("event object was not a Status: %T", event.Object)
							}
							m.Record(Condition{
								Level:   Info,
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

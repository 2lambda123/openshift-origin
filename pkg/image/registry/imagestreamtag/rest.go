package imagestreamtag

import (
	"fmt"
	"net/http"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
)

// REST implements the RESTStorage interface for ImageStreamTag
// It only supports the Get method and is used to simplify retrieving an Image by tag from an ImageStream
type REST struct {
	imageRegistry       image.Registry
	imageStreamRegistry imagestream.Registry
}

// NewREST returns a new REST.
func NewREST(imageRegistry image.Registry, imageStreamRegistry imagestream.Registry) *REST {
	return &REST{imageRegistry, imageStreamRegistry}
}

// New is only implemented to make REST implement RESTStorage
func (r *REST) New() runtime.Object {
	return &api.ImageStreamTag{}
}

// nameAndTag splits a string into its name component and tag component, and returns an error
// if the string is not in the right form.
func nameAndTag(id string) (name string, tag string, err error) {
	segments := strings.Split(id, ":")
	switch len(segments) {
	case 2:
		name = segments[0]
		tag = segments[1]
		if len(name) == 0 || len(tag) == 0 {
			err = errors.NewBadRequest("imageStreamTags must be retrieved with <name>:<tag>")
		}
	default:
		err = errors.NewBadRequest("imageStreamTags must be retrieved with <name>:<tag>")
	}
	return
}

// Get retrieves an image that has been tagged by stream and tag. `id` is of the format
// <stream name>:<tag>.
func (r *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	name, tag, err := nameAndTag(id)
	if err != nil {
		return nil, err
	}

	stream, err := r.imageStreamRegistry.GetImageStream(ctx, name)
	if err != nil {
		return nil, err
	}

	event := api.LatestTaggedImage(stream, tag)
	if event == nil || len(event.Image) == 0 {
		return nil, errors.NewNotFound("imageStreamTag", id)
	}

	image, err := r.imageRegistry.GetImage(ctx, event.Image)
	if err != nil {
		return nil, err
	}

	// if the stream has Spec.Tags[tag].Annotations[k] = v, copy it to the image's annotations
	if stream.Spec.Tags != nil {
		if tagRef, ok := stream.Spec.Tags[tag]; ok {
			if image.Annotations == nil {
				image.Annotations = make(map[string]string)
			}
			for k, v := range tagRef.Annotations {
				image.Annotations[k] = v
			}
		}
	}

	imageWithMetadata, err := api.ImageWithMetadata(*image)
	if err != nil {
		return nil, err
	}

	ist := api.ImageStreamTag{
		Image:     *imageWithMetadata,
		ImageName: imageWithMetadata.Name,
	}
	ist.Namespace = kapi.NamespaceValue(ctx)
	ist.Name = id
	return &ist, nil
}

// watcher provides support for watching image stream tags.
type watcher struct {
	imageStreamWatcher watch.Interface
	rest               *REST
	tag                string
	resultChan         chan watch.Event
}

// newWatcher returns a new watcher.
func newWatcher(imageStreamWatcher watch.Interface, rest *REST, tag string) watch.Interface {
	w := &watcher{imageStreamWatcher, rest, tag, make(chan watch.Event)}
	go w.run()
	return w
}

// Stop stops the watcher.
func (w *watcher) Stop() {
	w.imageStreamWatcher.Stop()
}

// run processes ImageStream events and emits ImageStreamTag events.
func (w *watcher) run() {
	for {
		event, ok := <-w.imageStreamWatcher.ResultChan()
		if !ok {
			break
		}

		switch event.Type {
		case watch.Added, watch.Modified:
			stream, ok := event.Object.(*api.ImageStream)
			if !ok {
				w.resultChan <- watch.Event{
					Type: watch.Error,
					Object: &kapi.Status{
						Status:  "Failure",
						Message: "event object was not an ImageStream",
						Code:    http.StatusInternalServerError,
					},
				}
				continue
			}

			if _, ok := stream.Status.Tags[w.tag]; !ok {
				continue
			}

			ist, err := w.rest.Get(kapi.WithNamespace(kapi.NewContext(), stream.Namespace), fmt.Sprintf("%s:%s", stream.Name, w.tag))
			if err != nil {
				w.resultChan <- watch.Event{
					Type: watch.Error,
					Object: &kapi.Status{
						Status:  "Failure",
						Message: fmt.Sprintf("error retrieving image stream tag: %v", err),
						Code:    http.StatusInternalServerError,
					},
				}
				continue
			}

			w.resultChan <- watch.Event{
				Type:   event.Type,
				Object: ist,
			}
		case watch.Deleted:
			stream, ok := event.Object.(*api.ImageStream)
			if !ok {
				w.resultChan <- watch.Event{
					Type: watch.Error,
					Object: &kapi.Status{
						Status:  "Failure",
						Message: "event object was not an ImageStream",
						Code:    http.StatusInternalServerError,
					},
				}
				continue
			}

			w.resultChan <- watch.Event{
				Type: watch.Deleted,
				Object: &api.ImageStreamTag{
					Image: api.Image{
						ObjectMeta: kapi.ObjectMeta{
							Namespace: stream.Namespace,
							Name:      fmt.Sprintf("%s:%s", stream.Name, w.tag),
						},
					},
				},
			}
		}
	}
}

// ResultChan returns the watcher's result channel.
func (w *watcher) ResultChan() <-chan watch.Event {
	return w.resultChan
}

func tagAndSelector(field fields.Selector) (tag string, selector fields.Selector, err error) {
	var stream string
	_, err = field.Transform(func(field, value string) (newField, newValue string, err error) {
		if field == "name" {
			parts := strings.Split(value, ":")
			if len(parts) != 2 {
				return "", "", fmt.Errorf("name must be of the form <stream>:<tag>")
			}

			stream = parts[0]
			if len(stream) == 0 {
				return "", "", fmt.Errorf("name must be of the form <stream>:<tag>")
			}

			tag = parts[1]
			if len(tag) == 0 {
				return "", "", fmt.Errorf("name must be of the form <stream>:<tag>")
			}
		}
		return field, value, nil
	})
	if err != nil {
		return
	}
	selector = fields.SelectorFromSet(fields.Set{"name": stream})
	return
}

func (r *REST) Watch(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	tag, imageStreamField, err := tagAndSelector(field)
	if err != nil {
		return nil, err
	}
	w, err := r.imageStreamRegistry.WatchImageStreams(ctx, label, imageStreamField, resourceVersion)
	if err != nil {
		return nil, fmt.Errorf("Error watching image streams: %v", err)
	}
	return newWatcher(w, r, tag), nil
}

// Delete removes a tag from a stream. `id` is of the format <stream name>:<tag>.
// The associated image that the tag points to is *not* deleted.
// The tag history remains intact and is not deleted.
func (r *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	name, tag, err := nameAndTag(id)
	if err != nil {
		return nil, err
	}

	stream, err := r.imageStreamRegistry.GetImageStream(ctx, name)
	if err != nil {
		return nil, err
	}

	if stream.Spec.Tags == nil {
		return nil, errors.NewNotFound("imageStreamTag", tag)
	}

	_, ok := stream.Spec.Tags[tag]
	if !ok {
		return nil, errors.NewNotFound("imageStreamTag", tag)
	}

	delete(stream.Spec.Tags, tag)

	_, err = r.imageStreamRegistry.UpdateImageStream(ctx, stream)
	if err != nil {
		return nil, fmt.Errorf("Error removing tag from image stream: %s", err)
	}

	return &kapi.Status{Status: kapi.StatusSuccess}, nil
}

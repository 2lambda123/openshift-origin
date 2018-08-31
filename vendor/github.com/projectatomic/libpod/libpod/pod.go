package libpod

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/storage"
	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/ulule/deepcopier"
)

// Pod represents a group of containers that are managed together.
// Any operations on a Pod that access state must begin with a call to
// updatePod().
// There is no guarantee that state exists in a readable state before this call,
// and even if it does its contents will be out of date and must be refreshed
// from the database.
// Generally, this requirement applies only to top-level functions; helpers can
// assume their callers handled this requirement. Generally speaking, if a
// function takes the pod lock and accesses any part of state, it should
// updatePod() immediately after locking.
// ffjson: skip
type Pod struct {
	config *PodConfig
	state  *podState

	valid   bool
	runtime *Runtime
	lock    storage.Locker
}

// PodConfig represents a pod's static configuration
type PodConfig struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Namespace the pod is in
	Namespace string `json:"namespace,omitempty"`

	// Labels contains labels applied to the pod
	Labels map[string]string `json:"labels"`
	// CgroupParent contains the pod's CGroup parent
	CgroupParent string `json:"cgroupParent"`
	// UsePodCgroup indicates whether the pod will create its own CGroup and
	// join containers to it.
	// If true, all containers joined to the pod will use the pod cgroup as
	// their cgroup parent, and cannot set a different cgroup parent
	UsePodCgroup bool

	// Time pod was created
	CreatedTime time.Time `json:"created"`
}

// podState represents a pod's state
type podState struct {
	// CgroupPath is the path to the pod's CGroup
	CgroupPath string
}

// PodInspect represents the data we want to display for
// podman pod inspect
type PodInspect struct {
	Config     *PodConfig
	State      *podState
	Containers []PodContainerInfo
}

// PodContainerInfo keeps information on a container in a pod
type PodContainerInfo struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// ID retrieves the pod's ID
func (p *Pod) ID() string {
	return p.config.ID
}

// Name retrieves the pod's name
func (p *Pod) Name() string {
	return p.config.Name
}

// Namespace returns the pod's libpod namespace.
// Namespaces are used to logically separate containers and pods in the state.
func (p *Pod) Namespace() string {
	return p.config.Namespace
}

// Labels returns the pod's labels
func (p *Pod) Labels() map[string]string {
	labels := make(map[string]string)
	for key, value := range p.config.Labels {
		labels[key] = value
	}

	return labels
}

// CreatedTime gets the time when the pod was created
func (p *Pod) CreatedTime() time.Time {
	return p.config.CreatedTime
}

// CgroupParent returns the pod's CGroup parent
func (p *Pod) CgroupParent() string {
	return p.config.CgroupParent
}

// UsePodCgroup returns whether containers in the pod will default to this pod's
// cgroup instead of the default libpod parent
func (p *Pod) UsePodCgroup() bool {
	return p.config.UsePodCgroup
}

// CgroupPath returns the path to the pod's CGroup
func (p *Pod) CgroupPath() (string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if err := p.updatePod(); err != nil {
		return "", err
	}

	return p.state.CgroupPath, nil
}

// Creates a new, empty pod
func newPod(lockDir string, runtime *Runtime) (*Pod, error) {
	pod := new(Pod)
	pod.config = new(PodConfig)
	pod.config.ID = stringid.GenerateNonCryptoID()
	pod.config.Labels = make(map[string]string)
	pod.config.CreatedTime = time.Now()
	pod.state = new(podState)
	pod.runtime = runtime

	// Path our lock file will reside at
	lockPath := filepath.Join(lockDir, pod.config.ID)
	// Grab a lockfile at the given path
	lock, err := storage.GetLockfile(lockPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating lockfile for new pod")
	}
	pod.lock = lock

	return pod, nil
}

// Update pod state from database
func (p *Pod) updatePod() error {
	if err := p.runtime.state.UpdatePod(p); err != nil {
		return err
	}

	return nil
}

// Save pod state to database
func (p *Pod) save() error {
	if err := p.runtime.state.SavePod(p); err != nil {
		return errors.Wrapf(err, "error saving pod %s state")
	}

	return nil
}

// Refresh a pod's state after restart
func (p *Pod) refresh() error {
	// Need to to an update from the DB to pull potentially-missing state
	if err := p.runtime.state.UpdatePod(p); err != nil {
		return err
	}

	if !p.valid {
		return ErrPodRemoved
	}

	// We need to recreate the pod's cgroup
	if p.config.UsePodCgroup {
		switch p.runtime.config.CgroupManager {
		case SystemdCgroupsManager:
			// NOOP for now, until proper systemd cgroup management
			// is implemented
		case CgroupfsCgroupsManager:
			p.state.CgroupPath = filepath.Join(p.config.CgroupParent, p.ID())

			logrus.Debugf("setting pod cgroup to %s", p.state.CgroupPath)
		default:
			return errors.Wrapf(ErrInvalidArg, "unknown cgroups manager %s specified", p.runtime.config.CgroupManager)
		}
	}

	// Save changes
	return p.save()
}

// Start starts all containers within a pod
// It combines the effects of Init() and Start() on a container
// If a container has already been initialized it will be started,
// otherwise it will be initialized then started.
// Containers that are already running or have been paused are ignored
// All containers are started independently, in order dictated by their
// dependencies.
// An error and a map[string]error are returned
// If the error is not nil and the map is nil, an error was encountered before
// any containers were started
// If map is not nil, an error was encountered when starting one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrCtrExists
// If both error and the map are nil, all containers were started successfully
func (p *Pod) Start(ctx context.Context) (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	// Build a dependency graph of containers in the pod
	graph, err := buildContainerGraph(allCtrs)
	if err != nil {
		return nil, errors.Wrapf(err, "error generating dependency graph for pod %s", p.ID())
	}

	ctrErrors := make(map[string]error)
	ctrsVisited := make(map[string]bool)

	// If there are no containers without dependencies, we can't start
	// Error out
	if len(graph.noDepNodes) == 0 {
		return nil, errors.Wrapf(ErrNoSuchCtr, "no containers in pod %s have no dependencies, cannot start pod", p.ID())
	}

	// Traverse the graph beginning at nodes with no dependencies
	for _, node := range graph.noDepNodes {
		startNode(ctx, node, false, ctrErrors, ctrsVisited, false)
	}

	return ctrErrors, nil
}

// Visit a node on a container graph and start the container, or set an error if
// a dependency failed to start. if restart is true, startNode will restart the node instead of starting it.
func startNode(ctx context.Context, node *containerNode, setError bool, ctrErrors map[string]error, ctrsVisited map[string]bool, restart bool) {
	// First, check if we have already visited the node
	if ctrsVisited[node.id] {
		return
	}

	// If setError is true, a dependency of us failed
	// Mark us as failed and recurse
	if setError {
		// Mark us as visited, and set an error
		ctrsVisited[node.id] = true
		ctrErrors[node.id] = errors.Wrapf(ErrCtrStateInvalid, "a dependency of container %s failed to start", node.id)

		// Hit anyone who depends on us, and set errors on them too
		for _, successor := range node.dependedOn {
			startNode(ctx, successor, true, ctrErrors, ctrsVisited, restart)
		}

		return
	}

	// Have all our dependencies started?
	// If not, don't visit the node yet
	depsVisited := true
	for _, dep := range node.dependsOn {
		depsVisited = depsVisited && ctrsVisited[dep.id]
	}
	if !depsVisited {
		// Don't visit us yet, all dependencies are not up
		// We'll hit the dependencies eventually, and when we do it will
		// recurse here
		return
	}

	// Going to try to start the container, mark us as visited
	ctrsVisited[node.id] = true

	ctrErrored := false

	// Check if dependencies are running
	// Graph traversal means we should have started them
	// But they could have died before we got here
	// Does not require that the container be locked, we only need to lock
	// the dependencies
	depsStopped, err := node.container.checkDependenciesRunning()
	if err != nil {
		ctrErrors[node.id] = err
		ctrErrored = true
	} else if len(depsStopped) > 0 {
		// Our dependencies are not running
		depsList := strings.Join(depsStopped, ",")
		ctrErrors[node.id] = errors.Wrapf(ErrCtrStateInvalid, "the following dependencies of container %s are not running: %s", node.id, depsList)
		ctrErrored = true
	}

	// Lock before we start
	node.container.lock.Lock()

	// Sync the container to pick up current state
	if !ctrErrored {
		if err := node.container.syncContainer(); err != nil {
			ctrErrored = true
			ctrErrors[node.id] = err
		}
	}

	// Start the container (only if it is not running)
	if !ctrErrored {
		if !restart && node.container.state.State != ContainerStateRunning {
			if err := node.container.initAndStart(ctx); err != nil {
				ctrErrored = true
				ctrErrors[node.id] = err
			}
		}
		if restart && node.container.state.State != ContainerStatePaused && node.container.state.State != ContainerStateUnknown {
			if err := node.container.restartWithTimeout(ctx, node.container.config.StopTimeout); err != nil {
				ctrErrored = true
				ctrErrors[node.id] = err
			}
		}
	}

	node.container.lock.Unlock()

	// Recurse to anyone who depends on us and start them
	for _, successor := range node.dependedOn {
		startNode(ctx, successor, ctrErrored, ctrErrors, ctrsVisited, restart)
	}

	return
}

// Stop stops all containers within a pod that are not already stopped
// Each container will use its own stop timeout
// Only running containers will be stopped. Paused, stopped, or created
// containers will be ignored.
// If cleanup is true, mounts and network namespaces will be cleaned up after
// the container is stopped.
// All containers are stopped independently. An error stopping one container
// will not prevent other containers being stopped.
// An error and a map[string]error are returned
// If the error is not nil and the map is nil, an error was encountered before
// any containers were stopped
// If map is not nil, an error was encountered when stopping one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrCtrExists
// If both error and the map are nil, all containers were stopped without error
func (p *Pod) Stop(cleanup bool) (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	ctrErrors := make(map[string]error)

	// TODO: There may be cases where it makes sense to order stops based on
	// dependencies. Should we bother with this?

	// Stop to all containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()

		if err := ctr.syncContainer(); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		// Ignore containers that are not running
		if ctr.state.State != ContainerStateRunning {
			ctr.lock.Unlock()
			continue
		}

		if err := ctr.stop(ctr.config.StopTimeout); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		if cleanup {
			if err := ctr.cleanup(); err != nil {
				ctrErrors[ctr.ID()] = err
			}
		}

		ctr.lock.Unlock()
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(ErrCtrExists, "error stopping some containers")
	}

	return nil, nil
}

// Pause pauses all containers within a pod that are running.
// Only running containers will be paused. Paused, stopped, or created
// containers will be ignored.
// All containers are paused independently. An error pausing one container
// will not prevent other containers being paused.
// An error and a map[string]error are returned
// If the error is not nil and the map is nil, an error was encountered before
// any containers were paused
// If map is not nil, an error was encountered when pausing one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrCtrExists
// If both error and the map are nil, all containers were paused without error
func (p *Pod) Pause() (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	ctrErrors := make(map[string]error)

	// Pause to all containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()

		if err := ctr.syncContainer(); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		// Ignore containers that are not running
		if ctr.state.State != ContainerStateRunning {
			ctr.lock.Unlock()
			continue
		}

		if err := ctr.pause(); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		ctr.lock.Unlock()
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(ErrCtrExists, "error pausing some containers")
	}

	return nil, nil
}

// Unpause unpauses all containers within a pod that are running.
// Only paused containers will be unpaused. Running, stopped, or created
// containers will be ignored.
// All containers are unpaused independently. An error unpausing one container
// will not prevent other containers being unpaused.
// An error and a map[string]error are returned
// If the error is not nil and the map is nil, an error was encountered before
// any containers were unpaused
// If map is not nil, an error was encountered when unpausing one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrCtrExists
// If both error and the map are nil, all containers were unpaused without error
func (p *Pod) Unpause() (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	ctrErrors := make(map[string]error)

	// Pause to all containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()

		if err := ctr.syncContainer(); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		// Ignore containers that are not paused
		if ctr.state.State != ContainerStatePaused {
			ctr.lock.Unlock()
			continue
		}

		if err := ctr.unpause(); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		ctr.lock.Unlock()
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(ErrCtrExists, "error unpausing some containers")
	}

	return nil, nil
}

// Restart restarts all containers within a pod that are not paused or in an error state.
// It combines the effects of Stop() and Start() on a container
// Each container will use its own stop timeout.
// All containers are started independently, in order dictated by their
// dependencies. An error restarting one container
// will not prevent other containers being restarted.
// An error and a map[string]error are returned
// If the error is not nil and the map is nil, an error was encountered before
// any containers were restarted
// If map is not nil, an error was encountered when restarting one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrCtrExists
// If both error and the map are nil, all containers were restarted without error
func (p *Pod) Restart(ctx context.Context) (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	// Build a dependency graph of containers in the pod
	graph, err := buildContainerGraph(allCtrs)
	if err != nil {
		return nil, errors.Wrapf(err, "error generating dependency graph for pod %s", p.ID())
	}

	ctrErrors := make(map[string]error)
	ctrsVisited := make(map[string]bool)

	// If there are no containers without dependencies, we can't start
	// Error out
	if len(graph.noDepNodes) == 0 {
		return nil, errors.Wrapf(ErrNoSuchCtr, "no containers in pod %s have no dependencies, cannot start pod", p.ID())
	}

	// Traverse the graph beginning at nodes with no dependencies
	for _, node := range graph.noDepNodes {
		startNode(ctx, node, false, ctrErrors, ctrsVisited, true)
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(ErrCtrExists, "error stopping some containers")
	}

	return nil, nil
}

// Kill sends a signal to all running containers within a pod
// Signals will only be sent to running containers. Containers that are not
// running will be ignored. All signals are sent independently, and sending will
// continue even if some containers encounter errors.
// An error and a map[string]error are returned
// If the error is not nil and the map is nil, an error was encountered before
// any containers were signalled
// If map is not nil, an error was encountered when signalling one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrCtrExists
// If both error and the map are nil, all containers were signalled successfully
func (p *Pod) Kill(signal uint) (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	ctrErrors := make(map[string]error)

	// Send a signal to all containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()

		if err := ctr.syncContainer(); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		// Ignore containers that are not running
		if ctr.state.State != ContainerStateRunning {
			ctr.lock.Unlock()
			continue
		}

		if err := ctr.runtime.ociRuntime.killContainer(ctr, signal); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		logrus.Debugf("Killed container %s with signal %d", ctr.ID(), signal)
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, nil
	}

	return nil, nil
}

// HasContainer checks if a container is present in the pod
func (p *Pod) HasContainer(id string) (bool, error) {
	if !p.valid {
		return false, ErrPodRemoved
	}

	return p.runtime.state.PodHasContainer(p, id)
}

// AllContainersByID returns the container IDs of all the containers in the pod
func (p *Pod) AllContainersByID() ([]string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, ErrPodRemoved
	}

	return p.runtime.state.PodContainersByID(p)
}

// AllContainers retrieves the containers in the pod
func (p *Pod) AllContainers() ([]*Container, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, ErrPodRemoved
	}

	return p.runtime.state.PodContainers(p)
}

// Status gets the status of all containers in the pod
// Returns a map of Container ID to Container Status
func (p *Pod) Status() (map[string]ContainerStatus, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	// We need to lock all the containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()
		defer ctr.lock.Unlock()
	}

	// Now that all containers are locked, get their status
	status := make(map[string]ContainerStatus, len(allCtrs))
	for _, ctr := range allCtrs {
		if err := ctr.syncContainer(); err != nil {
			return nil, err
		}

		status[ctr.ID()] = ctr.state.State
	}

	return status, nil
}

// TODO add pod batching
// Lock pod to avoid lock contention
// Store and lock all containers (no RemoveContainer in batch guarantees cache will not become stale)

// Inspect returns a PodInspect struct to describe the pod
func (p *Pod) Inspect() (*PodInspect, error) {
	var (
		podContainers []PodContainerInfo
	)

	containers, err := p.AllContainers()
	if err != nil {
		return &PodInspect{}, err
	}
	for _, c := range containers {
		containerStatus := "unknown"
		// Ignoring possible errors here because we dont want this to be
		// catastrophic in nature
		containerState, err := c.State()
		if err == nil {
			containerStatus = containerState.String()
		}
		pc := PodContainerInfo{
			ID:    c.ID(),
			State: containerStatus,
		}
		podContainers = append(podContainers, pc)
	}

	config := new(PodConfig)
	deepcopier.Copy(p.config).To(config)
	inspectData := PodInspect{
		Config:     config,
		State:      p.state,
		Containers: podContainers,
	}
	return &inspectData, nil
}

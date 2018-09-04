package libpod

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/BurntSushi/toml"
	is "github.com/containers/image/storage"
	"github.com/containers/image/types"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/hooks"
	sysreg "github.com/containers/libpod/pkg/registries"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/ulule/deepcopier"
)

// RuntimeStateStore is a constant indicating which state store implementation
// should be used by libpod
type RuntimeStateStore int

const (
	// InvalidStateStore is an invalid state store
	InvalidStateStore RuntimeStateStore = iota
	// InMemoryStateStore is an in-memory state that will not persist data
	// on containers and pods between libpod instances or after system
	// reboot
	InMemoryStateStore RuntimeStateStore = iota
	// SQLiteStateStore is a state backed by a SQLite database
	// It is presently disabled
	SQLiteStateStore RuntimeStateStore = iota
	// BoltDBStateStore is a state backed by a BoltDB database
	BoltDBStateStore RuntimeStateStore = iota

	// SeccompDefaultPath defines the default seccomp path
	SeccompDefaultPath = "/usr/share/containers/seccomp.json"
	// SeccompOverridePath if this exists it overrides the default seccomp path
	SeccompOverridePath = "/etc/crio/seccomp.json"

	// ConfigPath is the path to the libpod configuration file
	// This file is loaded to replace the builtin default config before
	// runtime options (e.g. WithStorageConfig) are applied.
	// If it is not present, the builtin default config is used instead
	// This path can be overridden when the runtime is created by using
	// NewRuntimeFromConfig() instead of NewRuntime()
	ConfigPath = "/usr/share/containers/libpod.conf"
	// OverrideConfigPath is the path to an override for the default libpod
	// configuration file. If OverrideConfigPath exists, it will be used in
	// place of the configuration file pointed to by ConfigPath.
	OverrideConfigPath = "/etc/containers/libpod.conf"

	// DefaultInfraImage to use for infra container
	DefaultInfraImage = "k8s.gcr.io/pause:3.1"
	// DefaultInfraCommand to be run in an infra container
	DefaultInfraCommand = "/pause"
)

// A RuntimeOption is a functional option which alters the Runtime created by
// NewRuntime
type RuntimeOption func(*Runtime) error

// Runtime is the core libpod runtime
type Runtime struct {
	config         *RuntimeConfig
	state          State
	store          storage.Store
	storageService *storageService
	imageContext   *types.SystemContext
	ociRuntime     *OCIRuntime
	lockDir        string
	netPlugin      ocicni.CNIPlugin
	ociRuntimePath string
	conmonPath     string
	valid          bool
	lock           sync.RWMutex
	imageRuntime   *image.Runtime
}

// RuntimeConfig contains configuration options used to set up the runtime
type RuntimeConfig struct {
	// StorageConfig is the configuration used by containers/storage
	// Not included in on-disk config, use the dedicated containers/storage
	// configuration file instead
	StorageConfig storage.StoreOptions `toml:"-"`
	// ImageDefaultTransport is the default transport method used to fetch
	// images
	ImageDefaultTransport string `toml:"image_default_transport"`
	// SignaturePolicyPath is the path to a signature policy to use for
	// validating images
	// If left empty, the containers/image default signature policy will
	// be used
	SignaturePolicyPath string `toml:"signature_policy_path,omitempty"`
	// StateType is the type of the backing state store.
	// Avoid using multiple values for this with the same containers/storage
	// configuration on the same system. Different state types do not
	// interact, and each will see a separate set of containers, which may
	// cause conflicts in containers/storage
	// As such this is not exposed via the config file
	StateType RuntimeStateStore `toml:"-"`
	// RuntimePath is the path to OCI runtime binary for launching
	// containers
	// The first path pointing to a valid file will be used
	RuntimePath []string `toml:"runtime_path"`
	// ConmonPath is the path to the Conmon binary used for managing
	// containers
	// The first path pointing to a valid file will be used
	ConmonPath []string `toml:"conmon_path"`
	// ConmonEnvVars are environment variables to pass to the Conmon binary
	// when it is launched
	ConmonEnvVars []string `toml:"conmon_env_vars"`
	// CGroupManager is the CGroup Manager to use
	// Valid values are "cgroupfs" and "systemd"
	CgroupManager string `toml:"cgroup_manager"`
	// StaticDir is the path to a persistent directory to store container
	// files
	StaticDir string `toml:"static_dir"`
	// TmpDir is the path to a temporary directory to store per-boot
	// container files
	// Must be stored in a tmpfs
	TmpDir string `toml:"tmp_dir"`
	// MaxLogSize is the maximum size of container logfiles
	MaxLogSize int64 `toml:"max_log_size,omitempty"`
	// NoPivotRoot sets whether to set no-pivot-root in the OCI runtime
	NoPivotRoot bool `toml:"no_pivot_root"`
	// CNIConfigDir sets the directory where CNI configuration files are
	// stored
	CNIConfigDir string `toml:"cni_config_dir"`
	// CNIPluginDir sets a number of directories where the CNI network
	// plugins can be located
	CNIPluginDir []string `toml:"cni_plugin_dir"`
	// HooksDir Path to the directory containing hooks configuration files
	HooksDir string `toml:"hooks_dir"`
	// CNIDefaultNetwork is the network name of the default CNI network
	// to attach pods to
	CNIDefaultNetwork string `toml:"cni_default_network,omitempty"`
	// HooksDirNotExistFatal switches between fatal errors and non-fatal
	// warnings if the configured HooksDir does not exist.
	HooksDirNotExistFatal bool `toml:"hooks_dir_not_exist_fatal"`
	// DefaultMountsFile is the path to the default mounts file for testing
	// purposes only
	DefaultMountsFile string `toml:"-"`
	// Namespace is the libpod namespace to use.
	// Namespaces are used to create scopes to separate containers and pods
	// in the state.
	// When namespace is set, libpod will only view containers and pods in
	// the same namespace. All containers and pods created will default to
	// the namespace set here.
	// A namespace of "", the empty string, is equivalent to no namespace,
	// and all containers and pods will be visible.
	// The default namespace is "".
	Namespace string `toml:"namespace,omitempty"`
	// InfraImage is the image a pod infra container will use to manage namespaces
	InfraImage string `toml:"infra_image"`
	// InfraCommand is the command run to start up a pod infra container
	InfraCommand string `toml:"infra_command"`
}

var (
	defaultRuntimeConfig = RuntimeConfig{
		// Leave this empty so containers/storage will use its defaults
		StorageConfig:         storage.StoreOptions{},
		ImageDefaultTransport: DefaultTransport,
		StateType:             BoltDBStateStore,
		RuntimePath: []string{
			"/usr/bin/runc",
			"/usr/sbin/runc",
			"/sbin/runc",
			"/bin/runc",
			"/usr/lib/cri-o-runc/sbin/runc",
		},
		ConmonPath: []string{
			"/usr/libexec/podman/conmon",
			"/usr/libexec/crio/conmon",
			"/usr/local/libexec/crio/conmon",
			"/usr/bin/conmon",
			"/usr/sbin/conmon",
			"/usr/lib/crio/bin/conmon",
		},
		ConmonEnvVars: []string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		},
		CgroupManager: SystemdCgroupsManager,
		HooksDir:      hooks.DefaultDir,
		StaticDir:     filepath.Join(storage.DefaultStoreOptions.GraphRoot, "libpod"),
		TmpDir:        "",
		MaxLogSize:    -1,
		NoPivotRoot:   false,
		CNIConfigDir:  "/etc/cni/net.d/",
		CNIPluginDir:  []string{"/usr/libexec/cni", "/usr/lib/cni", "/opt/cni/bin"},
		InfraCommand:  DefaultInfraCommand,
		InfraImage:    DefaultInfraImage,
	}
)

// GetRootlessRuntimeDir returns the runtime directory when running as non root
func GetRootlessRuntimeDir() (string, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	uid := fmt.Sprintf("%d", rootless.GetRootlessUID())
	if runtimeDir == "" {
		tmpDir := filepath.Join("/run", "user", uid)
		os.MkdirAll(tmpDir, 0700)
		st, err := os.Stat(tmpDir)
		if err == nil && int(st.Sys().(*syscall.Stat_t).Uid) == os.Getuid() && st.Mode().Perm() == 0700 {
			runtimeDir = tmpDir
		}
	}
	if runtimeDir == "" {
		tmpDir := filepath.Join(os.TempDir(), "user", uid)
		os.MkdirAll(tmpDir, 0700)
		st, err := os.Stat(tmpDir)
		if err == nil && int(st.Sys().(*syscall.Stat_t).Uid) == os.Getuid() && st.Mode().Perm() == 0700 {
			runtimeDir = tmpDir
		}
	}
	if runtimeDir == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return "", fmt.Errorf("neither XDG_RUNTIME_DIR nor HOME was set non-empty")
		}
		resolvedHome, err := filepath.EvalSymlinks(home)
		if err != nil {
			return "", errors.Wrapf(err, "cannot resolve %s", home)
		}
		runtimeDir = filepath.Join(resolvedHome, "rundir")
	}
	return runtimeDir, nil
}

func getDefaultTmpDir() (string, error) {
	if !rootless.IsRootless() {
		return "/var/run/libpod", nil
	}

	rootlessRuntimeDir, err := GetRootlessRuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(rootlessRuntimeDir, "libpod", "tmp"), nil
}

// NewRuntime creates a new container runtime
// Options can be passed to override the default configuration for the runtime
func NewRuntime(options ...RuntimeOption) (runtime *Runtime, err error) {
	runtime = new(Runtime)
	runtime.config = new(RuntimeConfig)

	// Copy the default configuration
	tmpDir, err := getDefaultTmpDir()
	if err != nil {
		return nil, err
	}
	deepcopier.Copy(defaultRuntimeConfig).To(runtime.config)
	runtime.config.TmpDir = tmpDir

	configPath := ConfigPath
	foundConfig := true
	if rootless.IsRootless() {
		home := os.Getenv("HOME")
		if runtime.config.SignaturePolicyPath == "" {
			newPath := filepath.Join(home, ".config/containers/policy.json")
			if _, err := os.Stat(newPath); err == nil {
				runtime.config.SignaturePolicyPath = newPath
			}
		}
		configPath = filepath.Join(home, ".config/containers/libpod.conf")
		if _, err := os.Stat(configPath); err != nil {
			foundConfig = false
		}

		runtimeDir, err := GetRootlessRuntimeDir()
		if err != nil {
			return nil, err
		}

		// containers/image uses XDG_RUNTIME_DIR to locate the auth file.
		// So make sure the env variable is set.
		err = os.Setenv("XDG_RUNTIME_DIR", runtimeDir)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot set XDG_RUNTIME_DIR")
		}

	} else if _, err := os.Stat(OverrideConfigPath); err == nil {
		// Use the override configuration path
		configPath = OverrideConfigPath
	} else if _, err := os.Stat(ConfigPath); err != nil {
		// Both stat checks failed, no config found
		foundConfig = false
	}

	// If we have a valid configuration file, load it in
	if foundConfig {
		contents, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading configuration file %s", configPath)
		}
		if _, err := toml.Decode(string(contents), runtime.config); err != nil {
			return nil, errors.Wrapf(err, "error decoding configuration file %s", configPath)
		}
	}

	// Overwrite config with user-given configuration options
	for _, opt := range options {
		if err := opt(runtime); err != nil {
			return nil, errors.Wrapf(err, "error configuring runtime")
		}
	}
	if err := makeRuntime(runtime); err != nil {
		return nil, err
	}
	return runtime, nil
}

// NewRuntimeFromConfig creates a new container runtime using the given
// configuration file for its default configuration. Passed RuntimeOption
// functions can be used to mutate this configuration further.
// An error will be returned if the configuration file at the given path does
// not exist or cannot be loaded
func NewRuntimeFromConfig(configPath string, options ...RuntimeOption) (runtime *Runtime, err error) {
	runtime = new(Runtime)
	runtime.config = new(RuntimeConfig)

	// Set two fields not in the TOML config
	runtime.config.StateType = defaultRuntimeConfig.StateType
	runtime.config.StorageConfig = storage.StoreOptions{}

	// Check to see if the given configuration file exists
	if _, err := os.Stat(configPath); err != nil {
		return nil, errors.Wrapf(err, "error checking existence of configuration file %s", configPath)
	}

	// Read contents of the config file
	contents, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading configuration file %s", configPath)
	}

	// Decode configuration file
	if _, err := toml.Decode(string(contents), runtime.config); err != nil {
		return nil, errors.Wrapf(err, "error decoding configuration from file %s", configPath)
	}

	// Overwrite the config with user-given configuration options
	for _, opt := range options {
		if err := opt(runtime); err != nil {
			return nil, errors.Wrapf(err, "error configuring runtime")
		}
	}

	if err := makeRuntime(runtime); err != nil {
		return nil, err
	}

	return runtime, nil
}

// Make a new runtime based on the given configuration
// Sets up containers/storage, state store, OCI runtime
func makeRuntime(runtime *Runtime) (err error) {
	// Find a working OCI runtime binary
	foundRuntime := false
	for _, path := range runtime.config.RuntimePath {
		stat, err := os.Stat(path)
		if err != nil {
			continue
		}
		if stat.IsDir() {
			continue
		}
		foundRuntime = true
		runtime.ociRuntimePath = path
		break
	}
	if !foundRuntime {
		return errors.Wrapf(ErrInvalidArg,
			"could not find a working runc binary (configured options: %v)",
			runtime.config.RuntimePath)
	}

	// Find a working conmon binary
	foundConmon := false
	for _, path := range runtime.config.ConmonPath {
		stat, err := os.Stat(path)
		if err != nil {
			continue
		}
		if stat.IsDir() {
			continue
		}
		foundConmon = true
		runtime.conmonPath = path
		break
	}
	if !foundConmon {
		return errors.Wrapf(ErrInvalidArg,
			"could not find a working conmon binary (configured options: %v)",
			runtime.config.ConmonPath)
	}

	// Set up containers/storage
	var store storage.Store
	if rootless.SkipStorageSetup() {
		logrus.Debug("Not configuring container store")
	} else {
		store, err = storage.GetStore(runtime.config.StorageConfig)
		if err != nil {
			return err
		}
	}

	runtime.store = store
	is.Transport.SetStore(store)

	// Set up image runtime and store in runtime
	ir := image.NewImageRuntimeFromStore(runtime.store)
	if err != nil {
		return err
	}

	runtime.imageRuntime = ir

	// Setting signaturepolicypath
	ir.SignaturePolicyPath = runtime.config.SignaturePolicyPath
	defer func() {
		if err != nil && store != nil {
			// Don't forcibly shut down
			// We could be opening a store in use by another libpod
			_, err2 := store.Shutdown(false)
			if err2 != nil {
				logrus.Errorf("Error removing store for partially-created runtime: %s", err2)
			}
		}
	}()

	// Set up a storage service for creating container root filesystems from
	// images
	storageService, err := getStorageService(runtime.store)
	if err != nil {
		return err
	}
	runtime.storageService = storageService

	// Set up containers/image
	runtime.imageContext = &types.SystemContext{
		SignaturePolicyPath: runtime.config.SignaturePolicyPath,
	}

	// Create the tmpDir
	if err := os.MkdirAll(runtime.config.TmpDir, 0751); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrapf(err, "error creating tmpdir %s", runtime.config.TmpDir)
		}
	}

	// Make an OCI runtime to perform container operations
	ociRuntime, err := newOCIRuntime("runc", runtime.ociRuntimePath,
		runtime.conmonPath, runtime.config.ConmonEnvVars,
		runtime.config.CgroupManager, runtime.config.TmpDir,
		runtime.config.MaxLogSize, runtime.config.NoPivotRoot)
	if err != nil {
		return err
	}
	runtime.ociRuntime = ociRuntime

	// Make the static files directory if it does not exist
	if err := os.MkdirAll(runtime.config.StaticDir, 0755); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrapf(err, "error creating runtime static files directory %s",
				runtime.config.StaticDir)
		}
	}

	// Make a directory to hold container lockfiles
	lockDir := filepath.Join(runtime.config.TmpDir, "lock")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrapf(err, "error creating runtime lockfiles directory %s",
				lockDir)
		}
	}
	runtime.lockDir = lockDir

	// Make the per-boot files directory if it does not exist
	if err := os.MkdirAll(runtime.config.TmpDir, 0755); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrapf(err, "error creating runtime temporary files directory %s",
				runtime.config.TmpDir)
		}
	}

	// Set up the CNI net plugin
	netPlugin, err := ocicni.InitCNI(runtime.config.CNIDefaultNetwork, runtime.config.CNIConfigDir, runtime.config.CNIPluginDir...)
	if err != nil {
		return errors.Wrapf(err, "error configuring CNI network plugin")
	}
	runtime.netPlugin = netPlugin

	// Set up the state
	switch runtime.config.StateType {
	case InMemoryStateStore:
		state, err := NewInMemoryState()
		if err != nil {
			return err
		}
		runtime.state = state
	case SQLiteStateStore:
		return errors.Wrapf(ErrInvalidArg, "SQLite state is currently disabled")
	case BoltDBStateStore:
		dbPath := filepath.Join(runtime.config.StaticDir, "bolt_state.db")

		state, err := NewBoltState(dbPath, runtime.lockDir, runtime)
		if err != nil {
			return err
		}
		runtime.state = state
	default:
		return errors.Wrapf(ErrInvalidArg, "unrecognized state type passed")
	}

	if err := runtime.state.SetNamespace(runtime.config.Namespace); err != nil {
		return errors.Wrapf(err, "error setting libpod namespace in state")
	}
	logrus.Debugf("Set libpod namespace to %q", runtime.config.Namespace)

	// We now need to see if the system has restarted
	// We check for the presence of a file in our tmp directory to verify this
	// This check must be locked to prevent races
	runtimeAliveLock := filepath.Join(runtime.config.TmpDir, "alive.lck")
	runtimeAliveFile := filepath.Join(runtime.config.TmpDir, "alive")
	aliveLock, err := storage.GetLockfile(runtimeAliveLock)
	if err != nil {
		return errors.Wrapf(err, "error acquiring runtime init lock")
	}
	// Acquire the lock and hold it until we return
	// This ensures that no two processes will be in runtime.refresh at once
	// TODO: we can't close the FD in this lock, so we should keep it around
	// and use it to lock important operations
	aliveLock.Lock()
	defer aliveLock.Unlock()
	_, err = os.Stat(runtimeAliveFile)
	if err != nil {
		// If the file doesn't exist, we need to refresh the state
		// This will trigger on first use as well, but refreshing an
		// empty state only creates a single file
		// As such, it's not really a performance concern
		if os.IsNotExist(err) {
			if err2 := runtime.refresh(runtimeAliveFile); err2 != nil {
				return err2
			}
		} else {
			return errors.Wrapf(err, "error reading runtime status file %s", runtimeAliveFile)
		}
	}

	// Mark the runtime as valid - ready to be used, cannot be modified
	// further
	runtime.valid = true

	return nil
}

// GetConfig returns a copy of the configuration used by the runtime
func (r *Runtime) GetConfig() *RuntimeConfig {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil
	}

	config := new(RuntimeConfig)

	// Copy so the caller won't be able to modify the actual config
	deepcopier.Copy(r.config).To(config)

	return config
}

// Shutdown shuts down the runtime and associated containers and storage
// If force is true, containers and mounted storage will be shut down before
// cleaning up; if force is false, an error will be returned if there are
// still containers running or mounted
func (r *Runtime) Shutdown(force bool) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return ErrRuntimeStopped
	}

	r.valid = false

	// Shutdown all containers if --force is given
	if force {
		ctrs, err := r.state.AllContainers()
		if err != nil {
			logrus.Errorf("Error retrieving containers from database: %v", err)
		} else {
			for _, ctr := range ctrs {
				if err := ctr.StopWithTimeout(CtrRemoveTimeout); err != nil {
					logrus.Errorf("Error stopping container %s: %v", ctr.ID(), err)
				}
			}
		}
	}

	var lastError error
	if r.store != nil {
		if _, err := r.store.Shutdown(force); err != nil {
			lastError = errors.Wrapf(err, "Error shutting down container storage")
		}
	}
	if err := r.state.Close(); err != nil {
		if lastError != nil {
			logrus.Errorf("%v", lastError)
		}
		lastError = err
	}

	return lastError
}

// Reconfigures the runtime after a reboot
// Refreshes the state, recreating temporary files
// Does not check validity as the runtime is not valid until after this has run
func (r *Runtime) refresh(alivePath string) error {
	// First clear the state in the database
	if err := r.state.Refresh(); err != nil {
		return err
	}

	// Next refresh the state of all containers to recreate dirs and
	// namespaces, and all the pods to recreate cgroups
	ctrs, err := r.state.AllContainers()
	if err != nil {
		return errors.Wrapf(err, "error retrieving all containers from state")
	}
	pods, err := r.state.AllPods()
	if err != nil {
		return errors.Wrapf(err, "error retrieving all pods from state")
	}
	for _, ctr := range ctrs {
		ctr.lock.Lock()
		if err := ctr.refresh(); err != nil {
			logrus.Errorf("Error refreshing container %s: %v", ctr.ID(), err)
		}
		ctr.lock.Unlock()
	}
	for _, pod := range pods {
		pod.lock.Lock()
		if err := pod.refresh(); err != nil {
			logrus.Errorf("Error refreshing pod %s: %v", pod.ID(), err)
		}
		pod.lock.Unlock()
	}

	// Create a file indicating the runtime is alive and ready
	file, err := os.OpenFile(alivePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return errors.Wrapf(err, "error creating runtime status file %s", alivePath)
	}
	defer file.Close()

	return nil
}

// Info returns the store and host information
func (r *Runtime) Info() ([]InfoData, error) {
	info := []InfoData{}
	// get host information
	hostInfo, err := r.hostInfo()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting host info")
	}
	info = append(info, InfoData{Type: "host", Data: hostInfo})

	// get store information
	storeInfo, err := r.storeInfo()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting store info")
	}
	info = append(info, InfoData{Type: "store", Data: storeInfo})

	reg, err := sysreg.GetRegistries()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting registries")
	}
	registries := make(map[string]interface{})
	registries["registries"] = reg
	info = append(info, InfoData{Type: "registries", Data: registries})

	i, err := sysreg.GetInsecureRegistries()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting registries")
	}
	insecureRegistries := make(map[string]interface{})
	insecureRegistries["registries"] = i
	info = append(info, InfoData{Type: "insecure registries", Data: insecureRegistries})
	return info, nil
}

// generateName generates a unique name for a container or pod.
func (r *Runtime) generateName() (string, error) {
	for {
		name := namesgenerator.GetRandomName(0)
		// Make sure container with this name does not exist
		if _, err := r.state.LookupContainer(name); err == nil {
			continue
		} else {
			if errors.Cause(err) != ErrNoSuchCtr {
				return "", err
			}
		}
		// Make sure pod with this name does not exist
		if _, err := r.state.LookupPod(name); err == nil {
			continue
		} else {
			if errors.Cause(err) != ErrNoSuchPod {
				return "", err
			}
		}
		return name, nil
	}
	// The code should never reach here.
}

// ImageRuntime returns the imageruntime for image resolution
func (r *Runtime) ImageRuntime() *image.Runtime {
	return r.imageRuntime
}

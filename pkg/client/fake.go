package client

type FakeAction struct {
	Action string
	Value  interface{}
}

// Fake implements Interface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the method you want to test easier.
type Fake struct {
	// Fake by default keeps a simple list of the methods that have been called.
	Actions []FakeAction
}

func (c *Fake) Builds(namespace string) BuildInterface {
	return &FakeBuilds{Fake: c, Namespace: namespace}
}

func (c *Fake) BuildConfigs(namespace string) BuildConfigInterface {
	return &FakeBuildConfigs{Fake: c, Namespace: namespace}
}

func (c *Fake) Images(namespace string) ImageInterface {
	return &FakeImages{Fake: c, Namespace: namespace}
}

func (c *Fake) ImageRepositories(namespace string) ImageRepositoryInterface {
	return &FakeImageRepositories{Fake: c, Namespace: namespace}
}

func (c *Fake) ImageRepositoryMappings(namespace string) ImageRepositoryMappingInterface {
	return &FakeImageRepositoryMappings{Fake: c, Namespace: namespace}
}

func (c *Fake) ImageRepositoryTags(namespace string) ImageRepositoryTagInterface {
	return &FakeImageRepositoryTags{Fake: c, Namespace: namespace}
}

func (c *Fake) Deployments(namespace string) DeploymentInterface {
	return &FakeDeployments{Fake: c, Namespace: namespace}
}

func (c *Fake) DeploymentConfigs(namespace string) DeploymentConfigInterface {
	return &FakeDeploymentConfigs{Fake: c, Namespace: namespace}
}

func (c *Fake) Routes(namespace string) RouteInterface {
	return &FakeRoutes{Fake: c, Namespace: namespace}
}

func (c *Fake) Users() UserInterface {
	return &FakeUsers{Fake: c}
}

func (c *Fake) UserIdentityMappings() UserIdentityMappingInterface {
	return &FakeUserIdentityMappings{Fake: c}
}

func (c *Fake) Projects() ProjectInterface {
	return &FakeProjects{Fake: c}
}

func (c *Fake) Policies(namespace string) PolicyInterface {
	return &FakePolicies{Fake: c}
}

func (c *Fake) Roles(namespace string) RoleInterface {
	return &FakeRoles{Fake: c}
}

func (c *Fake) RoleBindings(namespace string) RoleBindingInterface {
	return &FakeRoleBindings{Fake: c}
}

func (c *Fake) PolicyBindings(namespace string) PolicyBindingInterface {
	return &FakePolicyBindings{Fake: c}
}

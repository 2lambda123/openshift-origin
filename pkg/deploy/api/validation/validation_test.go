package validation

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/api/test"
)

// Convenience methods

func manualTrigger() []api.DeploymentTriggerPolicy {
	return []api.DeploymentTriggerPolicy{
		{
			Type: api.DeploymentTriggerManual,
		},
	}
}

// TODO: test validation errors for ReplicationControllerTemplates

func TestValidateDeploymentOK(t *testing.T) {
	errs := ValidateDeployment(&api.Deployment{
		ObjectMeta:         kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
		Strategy:           test.OkStrategy(),
		ControllerTemplate: test.OkControllerTemplate(),
	})
	if len(errs) > 0 {
		t.Errorf("Unxpected non-empty error list: %#v", errs)
	}
}

func TestValidateDeploymentMissingFields(t *testing.T) {
	errorCases := map[string]struct {
		D api.Deployment
		T fielderrors.ValidationErrorType
		F string
	}{
		"missing strategy.type": {
			api.Deployment{
				ObjectMeta:         kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Strategy:           api.DeploymentStrategy{},
				ControllerTemplate: test.OkControllerTemplate(),
			},
			fielderrors.ValidationErrorTypeRequired,
			"strategy.type",
		},
	}

	for k, v := range errorCases {
		errs := ValidateDeployment(&v.D)
		if len(errs) == 0 {
			t.Errorf("Expected failure for scenario %s", k)
		}
		for i := range errs {
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateDeploymentConfigOK(t *testing.T) {
	errs := ValidateDeploymentConfig(&api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
		Triggers:   manualTrigger(),
		Template:   test.OkDeploymentTemplate(),
	})

	if len(errs) > 0 {
		t.Errorf("Unxpected non-empty error list: %#v", errs)
	}
}

func TestValidateDeploymentConfigMissingFields(t *testing.T) {
	errorCases := map[string]struct {
		D api.DeploymentConfig
		T fielderrors.ValidationErrorType
		F string
	}{
		"missing name": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "", Namespace: "bar"},
				Template:   test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeRequired,
			"name",
		},
		"missing namespace": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: ""},
				Template:   test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeRequired,
			"namespace",
		},
		"invalid name": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "-foo", Namespace: "bar"},
				Template:   test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeInvalid,
			"name",
		},
		"invalid namespace": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "-bar"},
				Template:   test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeInvalid,
			"namespace",
		},

		"missing trigger.type": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers: []api.DeploymentTriggerPolicy{
					{
						ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
							ContainerNames: []string{"foo"},
						},
					},
				},
				Template: test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeRequired,
			"triggers[0].type",
		},
		"missing Trigger imageChangeParams.from": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers: []api.DeploymentTriggerPolicy{
					{
						Type: api.DeploymentTriggerOnImageChange,
						ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
							ContainerNames: []string{"foo"},
						},
					},
				},
				Template: test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeRequired,
			"triggers[0].imageChangeParams.from",
		},
		"both fields illegal Trigger imageChangeParams.repositoryName": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers: []api.DeploymentTriggerPolicy{
					{
						Type: api.DeploymentTriggerOnImageChange,
						ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
							ContainerNames: []string{"foo"},
							RepositoryName: "name",
							From: kapi.ObjectReference{
								Name: "other",
							},
						},
					},
				},
				Template: test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeInvalid,
			"triggers[0].imageChangeParams.repositoryName",
		},
		"missing Trigger imageChangeParams.containerNames": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers: []api.DeploymentTriggerPolicy{
					{
						Type: api.DeploymentTriggerOnImageChange,
						ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
							RepositoryName: "foo",
						},
					},
				},
				Template: test.OkDeploymentTemplate(),
			},
			fielderrors.ValidationErrorTypeRequired,
			"triggers[0].imageChangeParams.containerNames",
		},
		"missing strategy.type": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers:   manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						CustomParams: test.OkCustomParams(),
					},
					ControllerTemplate: test.OkControllerTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"template.strategy.type",
		},
		"missing strategy.customParams": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers:   manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeCustom,
					},
					ControllerTemplate: test.OkControllerTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"template.strategy.customParams",
		},
		"missing template.strategy.customParams.image": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Triggers:   manualTrigger(),
				Template: api.DeploymentTemplate{
					Strategy: api.DeploymentStrategy{
						Type:         api.DeploymentStrategyTypeCustom,
						CustomParams: &api.CustomDeploymentStrategyParams{},
					},
					ControllerTemplate: test.OkControllerTemplate(),
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"template.strategy.customParams.image",
		},
	}

	for k, v := range errorCases {
		errs := ValidateDeploymentConfig(&v.D)
		if len(errs) == 0 {
			t.Errorf("Expected failure for scenario %s", k)
		}
		for i := range errs {
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateDeploymentConfigRollbackOK(t *testing.T) {
	rollback := &api.DeploymentConfigRollback{
		Spec: api.DeploymentConfigRollbackSpec{
			From: kapi.ObjectReference{
				Name: "deployment",
			},
		},
	}

	errs := ValidateDeploymentConfigRollback(rollback)
	if len(errs) > 0 {
		t.Errorf("Unxpected non-empty error list: %v", errs)
	}

	if e, a := "ReplicationController", rollback.Spec.From.Kind; e != a {
		t.Errorf("expected kind %s, got %s", e, a)
	}
}

func TestValidateDeploymentConfigRollbackInvalidFields(t *testing.T) {
	errorCases := map[string]struct {
		D api.DeploymentConfigRollback
		T fielderrors.ValidationErrorType
		F string
	}{
		"missing spec.from.name": {
			api.DeploymentConfigRollback{
				Spec: api.DeploymentConfigRollbackSpec{
					From: kapi.ObjectReference{},
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"spec.from.name",
		},
		"wrong spec.from.kind": {
			api.DeploymentConfigRollback{
				Spec: api.DeploymentConfigRollbackSpec{
					From: kapi.ObjectReference{
						Kind: "unknown",
						Name: "deployment",
					},
				},
			},
			fielderrors.ValidationErrorTypeInvalid,
			"spec.from.kind",
		},
	}

	for k, v := range errorCases {
		errs := ValidateDeploymentConfigRollback(&v.D)
		if len(errs) == 0 {
			t.Errorf("Expected failure for scenario %s", k)
		}
		for i := range errs {
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

package validation

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/intstr"
	"k8s.io/kubernetes/pkg/util/validation/field"

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

func rollingConfig(interval, updatePeriod, timeout int) api.DeploymentConfig {
	return api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
		Spec: api.DeploymentConfigSpec{
			Triggers: manualTrigger(),
			Strategy: api.DeploymentStrategy{
				Type: api.DeploymentStrategyTypeRolling,
				RollingParams: &api.RollingDeploymentStrategyParams{
					IntervalSeconds:     mkint64p(interval),
					UpdatePeriodSeconds: mkint64p(updatePeriod),
					TimeoutSeconds:      mkint64p(timeout),
					MaxSurge:            intstr.FromInt(1),
				},
			},
			Template: test.OkPodTemplate(),
			Selector: test.OkSelector(),
		},
	}
}

func rollingConfigMax(maxSurge, maxUnavailable intstr.IntOrString) api.DeploymentConfig {
	return api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
		Spec: api.DeploymentConfigSpec{
			Triggers: manualTrigger(),
			Strategy: api.DeploymentStrategy{
				Type: api.DeploymentStrategyTypeRolling,
				RollingParams: &api.RollingDeploymentStrategyParams{
					IntervalSeconds:     mkint64p(1),
					UpdatePeriodSeconds: mkint64p(1),
					TimeoutSeconds:      mkint64p(1),
					MaxSurge:            maxSurge,
					MaxUnavailable:      maxUnavailable,
				},
			},
			Template: test.OkPodTemplate(),
			Selector: test.OkSelector(),
		},
	}
}

func TestValidateDeploymentConfigOK(t *testing.T) {
	errs := ValidateDeploymentConfig(&api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
		Spec: api.DeploymentConfigSpec{
			Replicas: 1,
			Triggers: manualTrigger(),
			Selector: test.OkSelector(),
			Strategy: test.OkStrategy(),
			Template: test.OkPodTemplate(),
		},
	})

	if len(errs) > 0 {
		t.Errorf("Unxpected non-empty error list: %#v", errs)
	}
}

func TestValidateDeploymentConfigICTMissingImage(t *testing.T) {
	dc := &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
		Spec: api.DeploymentConfigSpec{
			Replicas: 1,
			Triggers: []api.DeploymentTriggerPolicy{test.OkImageChangeTrigger()},
			Selector: test.OkSelector(),
			Strategy: test.OkStrategy(),
			Template: test.OkPodTemplateMissingImage("container1"),
		},
	}
	errs := ValidateDeploymentConfig(dc)

	if len(errs) > 0 {
		t.Errorf("Unexpected non-empty error list: %+v", errs)
	}

	for _, c := range dc.Spec.Template.Spec.Containers {
		if c.Image == "unset" {
			t.Errorf("%s image field still has validation fake out value of %s", c.Name, c.Image)
		}
	}
}

func TestValidateDeploymentConfigMissingFields(t *testing.T) {
	errorCases := map[string]struct {
		DeploymentConfig api.DeploymentConfig
		ErrorType        field.ErrorType
		Field            string
	}{
		"empty container field": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: []api.DeploymentTriggerPolicy{test.OkConfigChangeTrigger()},
					Selector: test.OkSelector(),
					Strategy: test.OkStrategy(),
					Template: test.OkPodTemplateMissingImage("container1"),
				},
			},
			field.ErrorTypeRequired,
			"spec.template.spec.containers[0].image",
		},
		"missing name": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "", Namespace: "bar"},
				Spec:       test.OkDeploymentConfigSpec(),
			},
			field.ErrorTypeRequired,
			"metadata.name",
		},
		"missing namespace": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: ""},
				Spec:       test.OkDeploymentConfigSpec(),
			},
			field.ErrorTypeRequired,
			"metadata.namespace",
		},
		"invalid name": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "-foo", Namespace: "bar"},
				Spec:       test.OkDeploymentConfigSpec(),
			},
			field.ErrorTypeInvalid,
			"metadata.name",
		},
		"invalid namespace": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "-bar"},
				Spec:       test.OkDeploymentConfigSpec(),
			},
			field.ErrorTypeInvalid,
			"metadata.namespace",
		},

		"missing trigger.type": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: []api.DeploymentTriggerPolicy{
						{
							ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
								ContainerNames: []string{"foo"},
							},
						},
					},
					Selector: test.OkSelector(),
					Strategy: test.OkStrategy(),
					Template: test.OkPodTemplate(),
				},
			},
			field.ErrorTypeRequired,
			"spec.triggers[0].type",
		},
		"missing Trigger imageChangeParams.from": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: []api.DeploymentTriggerPolicy{
						{
							Type: api.DeploymentTriggerOnImageChange,
							ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
								ContainerNames: []string{"foo"},
							},
						},
					},
					Selector: test.OkSelector(),
					Strategy: test.OkStrategy(),
					Template: test.OkPodTemplate(),
				},
			},
			field.ErrorTypeRequired,
			"spec.triggers[0].imageChangeParams.from",
		},
		"invalid Trigger imageChangeParams.from.kind": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: []api.DeploymentTriggerPolicy{
						{
							Type: api.DeploymentTriggerOnImageChange,
							ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
								From: kapi.ObjectReference{
									Kind: "Invalid",
									Name: "name:tag",
								},
								ContainerNames: []string{"foo"},
							},
						},
					},
					Selector: test.OkSelector(),
					Strategy: test.OkStrategy(),
					Template: test.OkPodTemplate(),
				},
			},
			field.ErrorTypeInvalid,
			"spec.triggers[0].imageChangeParams.from.kind",
		},
		"missing Trigger imageChangeParams.containerNames": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: []api.DeploymentTriggerPolicy{
						{
							Type: api.DeploymentTriggerOnImageChange,
							ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
								From: kapi.ObjectReference{
									Kind: "ImageStreamTag",
									Name: "foo:v1",
								},
							},
						},
					},
					Selector: test.OkSelector(),
					Strategy: test.OkStrategy(),
					Template: test.OkPodTemplate(),
				},
			},
			field.ErrorTypeRequired,
			"spec.triggers[0].imageChangeParams.containerNames",
		},
		"missing strategy.type": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: manualTrigger(),
					Selector: test.OkSelector(),
					Strategy: api.DeploymentStrategy{
						CustomParams: test.OkCustomParams(),
					},
					Template: test.OkPodTemplate(),
				},
			},
			field.ErrorTypeRequired,
			"spec.strategy.type",
		},
		"missing strategy.customParams": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: manualTrigger(),
					Selector: test.OkSelector(),
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeCustom,
					},
					Template: test.OkPodTemplate(),
				},
			},
			field.ErrorTypeRequired,
			"spec.strategy.customParams",
		},
		"invalid spec.strategy.customParams.environment": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Triggers: manualTrigger(),
					Selector: test.OkSelector(),
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeCustom,
						CustomParams: &api.CustomDeploymentStrategyParams{
							Environment: []kapi.EnvVar{
								{Name: "A=B"},
							},
						},
					},
					Template: test.OkPodTemplate(),
				},
			},
			field.ErrorTypeInvalid,
			"spec.strategy.customParams.environment[0].name",
		},
		"missing spec.strategy.recreateParams.pre.failurePolicy": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Pre: &api.LifecycleHook{
								ExecNewPod: &api.ExecNewPodHook{
									Command:       []string{"cmd"},
									ContainerName: "container",
								},
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			field.ErrorTypeRequired,
			"spec.strategy.recreateParams.pre.failurePolicy",
		},
		"missing spec.strategy.recreateParams.pre.execNewPod": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Pre: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			field.ErrorTypeInvalid,
			"spec.strategy.recreateParams.pre",
		},
		"missing spec.strategy.recreateParams.pre.execNewPod.command": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Pre: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
								ExecNewPod: &api.ExecNewPodHook{
									ContainerName: "container",
								},
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			field.ErrorTypeRequired,
			"spec.strategy.recreateParams.pre.execNewPod.command",
		},
		"missing spec.strategy.recreateParams.pre.execNewPod.containerName": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Pre: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
								ExecNewPod: &api.ExecNewPodHook{
									Command: []string{"cmd"},
								},
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			field.ErrorTypeRequired,
			"spec.strategy.recreateParams.pre.execNewPod.containerName",
		},
		"invalid spec.strategy.recreateParams.pre.execNewPod.volumes": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Pre: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
								ExecNewPod: &api.ExecNewPodHook{
									ContainerName: "container",
									Command:       []string{"cmd"},
									Volumes:       []string{"good", ""},
								},
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			field.ErrorTypeInvalid,
			"spec.strategy.recreateParams.pre.execNewPod.volumes[1]",
		},
		"missing spec.strategy.recreateParams.mid.execNewPod": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Mid: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			field.ErrorTypeInvalid,
			"spec.strategy.recreateParams.mid",
		},
		"missing spec.strategy.recreateParams.post.execNewPod": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Post: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			field.ErrorTypeInvalid,
			"spec.strategy.recreateParams.post",
		},
		"missing spec.strategy.after.tagImages": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Post: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
								TagImages: []api.TagImageHook{
									{
										ContainerName: "missing",
										To:            kapi.ObjectReference{Kind: "ImageStreamTag", Name: "stream:tag"},
									},
								},
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			field.ErrorTypeInvalid,
			"spec.strategy.recreateParams.post.tagImages[0].containerName",
		},
		"missing spec.strategy.after.tagImages.to.kind": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Post: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
								TagImages: []api.TagImageHook{
									{
										ContainerName: "container1",
										To:            kapi.ObjectReference{Name: "stream:tag"},
									},
								},
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			field.ErrorTypeInvalid,
			"spec.strategy.recreateParams.post.tagImages[0].to.kind",
		},
		"missing spec.strategy.after.tagImages.to.name": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Post: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
								TagImages: []api.TagImageHook{
									{
										ContainerName: "container1",
										To:            kapi.ObjectReference{Kind: "ImageStreamTag"},
									},
								},
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			field.ErrorTypeRequired,
			"spec.strategy.recreateParams.post.tagImages[0].to.name",
		},
		"can't have both tag and execNewPod": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRecreate,
						RecreateParams: &api.RecreateDeploymentStrategyParams{
							Post: &api.LifecycleHook{
								FailurePolicy: api.LifecycleHookFailurePolicyRetry,
								ExecNewPod:    &api.ExecNewPodHook{},
								TagImages:     []api.TagImageHook{{}},
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			field.ErrorTypeInvalid,
			"spec.strategy.recreateParams.post",
		},
		"invalid spec.strategy.rollingParams.intervalSeconds": {
			rollingConfig(-20, 1, 1),
			field.ErrorTypeInvalid,
			"spec.strategy.rollingParams.intervalSeconds",
		},
		"invalid spec.strategy.rollingParams.updatePeriodSeconds": {
			rollingConfig(1, -20, 1),
			field.ErrorTypeInvalid,
			"spec.strategy.rollingParams.updatePeriodSeconds",
		},
		"invalid spec.strategy.rollingParams.timeoutSeconds": {
			rollingConfig(1, 1, -20),
			field.ErrorTypeInvalid,
			"spec.strategy.rollingParams.timeoutSeconds",
		},
		"missing spec.strategy.rollingParams.pre.failurePolicy": {
			api.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.DeploymentConfigSpec{
					Replicas: 1,
					Strategy: api.DeploymentStrategy{
						Type: api.DeploymentStrategyTypeRolling,
						RollingParams: &api.RollingDeploymentStrategyParams{
							IntervalSeconds:     mkint64p(1),
							UpdatePeriodSeconds: mkint64p(1),
							TimeoutSeconds:      mkint64p(20),
							MaxSurge:            intstr.FromInt(1),
							Pre: &api.LifecycleHook{
								ExecNewPod: &api.ExecNewPodHook{
									Command:       []string{"cmd"},
									ContainerName: "container",
								},
							},
						},
					},
					Template: test.OkPodTemplate(),
					Selector: test.OkSelector(),
				},
			},
			field.ErrorTypeRequired,
			"spec.strategy.rollingParams.pre.failurePolicy",
		},
		"both maxSurge and maxUnavailable 0 spec.strategy.rollingParams.maxUnavailable": {
			rollingConfigMax(intstr.FromInt(0), intstr.FromInt(0)),
			field.ErrorTypeInvalid,
			"spec.strategy.rollingParams.maxUnavailable",
		},
		"invalid lower bound spec.strategy.rollingParams.maxUnavailable": {
			rollingConfigMax(intstr.FromInt(0), intstr.FromInt(-100)),
			field.ErrorTypeInvalid,
			"spec.strategy.rollingParams.maxUnavailable",
		},
		"invalid lower bound spec.strategy.rollingParams.maxSurge": {
			rollingConfigMax(intstr.FromInt(-1), intstr.FromInt(0)),
			field.ErrorTypeInvalid,
			"spec.strategy.rollingParams.maxSurge",
		},
		"both maxSurge and maxUnavailable 0 percent spec.strategy.rollingParams.maxUnavailable": {
			rollingConfigMax(intstr.FromString("0%"), intstr.FromString("0%")),
			field.ErrorTypeInvalid,
			"spec.strategy.rollingParams.maxUnavailable",
		},
		"invalid lower bound percent spec.strategy.rollingParams.maxUnavailable": {
			rollingConfigMax(intstr.FromInt(0), intstr.FromString("-1%")),
			field.ErrorTypeInvalid,
			"spec.strategy.rollingParams.maxUnavailable",
		},
		"invalid upper bound percent spec.strategy.rollingParams.maxUnavailable": {
			rollingConfigMax(intstr.FromInt(0), intstr.FromString("101%")),
			field.ErrorTypeInvalid,
			"spec.strategy.rollingParams.maxUnavailable",
		},
		"invalid percent spec.strategy.rollingParams.maxUnavailable": {
			rollingConfigMax(intstr.FromInt(0), intstr.FromString("foo")),
			field.ErrorTypeInvalid,
			"spec.strategy.rollingParams.maxUnavailable",
		},
		"invalid percent spec.strategy.rollingParams.maxSurge": {
			rollingConfigMax(intstr.FromString("foo"), intstr.FromString("100%")),
			field.ErrorTypeInvalid,
			"spec.strategy.rollingParams.maxSurge",
		},
	}

	for testName, v := range errorCases {
		errs := ValidateDeploymentConfig(&v.DeploymentConfig)
		if len(v.ErrorType) == 0 {
			if len(errs) > 0 {
				for _, e := range errs {
					t.Errorf("%s: unexpected error: %s", testName, e)
				}
			}
			continue
		}
		if len(errs) == 0 {
			t.Errorf("%s: expected test failure, got success", testName)
		}
		for i := range errs {
			if got, exp := errs[i].Type, v.ErrorType; got != exp {
				t.Errorf("%s: expected error \"%v\" to have type %q, but got %q", testName, errs[i], exp, got)
			}
			if got, exp := errs[i].Field, v.Field; got != exp {
				t.Errorf("%s: expected error \"%v\" to have field %q, but got %q", testName, errs[i], exp, got)
			}
		}
	}
}

func TestValidateDeploymentConfigUpdate(t *testing.T) {
	oldConfig := &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar", ResourceVersion: "1"},
		Spec: api.DeploymentConfigSpec{
			Replicas: 1,
			Triggers: manualTrigger(),
			Selector: test.OkSelector(),
			Strategy: test.OkStrategy(),
			Template: test.OkPodTemplate(),
		},
		Status: api.DeploymentConfigStatus{
			LatestVersion: 5,
		},
	}
	newConfig := &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar", ResourceVersion: "1"},
		Spec: api.DeploymentConfigSpec{
			Replicas: 1,
			Triggers: manualTrigger(),
			Selector: test.OkSelector(),
			Strategy: test.OkStrategy(),
			Template: test.OkPodTemplate(),
		},
		Status: api.DeploymentConfigStatus{
			LatestVersion: 3,
		},
	}

	scenarios := []struct {
		oldLatestVersion int64
		newLatestVersion int64
	}{
		{5, 3},
		{5, 7},
		{0, -1},
	}

	for _, values := range scenarios {
		oldConfig.Status.LatestVersion = values.oldLatestVersion
		newConfig.Status.LatestVersion = values.newLatestVersion
		errs := ValidateDeploymentConfigUpdate(newConfig, oldConfig)
		if len(errs) == 0 {
			t.Errorf("Expected update failure")
		}
		for i := range errs {
			if errs[i].Type != field.ErrorTypeInvalid {
				t.Errorf("expected update error to have type %s: %v", field.ErrorTypeInvalid, errs[i])
			}
			if errs[i].Field != "status.latestVersion" {
				t.Errorf("expected update error to have field %s: %v", "latestVersion", errs[i])
			}
		}
	}

	// testing for a successful update
	oldConfig.Status.LatestVersion = 5
	newConfig.Status.LatestVersion = 6
	errs := ValidateDeploymentConfigUpdate(newConfig, oldConfig)
	if len(errs) > 0 {
		t.Errorf("Unexpected update failure")
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
		T field.ErrorType
		F string
	}{
		"missing spec.from.name": {
			api.DeploymentConfigRollback{
				Spec: api.DeploymentConfigRollbackSpec{
					From: kapi.ObjectReference{},
				},
			},
			field.ErrorTypeRequired,
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
			field.ErrorTypeInvalid,
			"spec.from.kind",
		},
	}

	for k, v := range errorCases {
		errs := ValidateDeploymentConfigRollback(&v.D)
		if len(errs) == 0 {
			t.Errorf("Expected failure for scenario %s", k)
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateDeploymentConfigDefaultImageStreamKind(t *testing.T) {
	config := &api.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "bar"},
		Spec: api.DeploymentConfigSpec{
			Replicas: 1,
			Triggers: []api.DeploymentTriggerPolicy{
				{
					Type: api.DeploymentTriggerOnImageChange,
					ImageChangeParams: &api.DeploymentTriggerImageChangeParams{
						From: kapi.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "name:v1",
						},
						ContainerNames: []string{"foo"},
					},
				},
			},
			Selector: test.OkSelector(),
			Template: test.OkPodTemplate(),
			Strategy: test.OkStrategy(),
		},
	}

	if errs := ValidateDeploymentConfig(config); len(errs) > 0 {
		t.Errorf("Unxpected non-empty error list: %v", errs)
	}
}

func mkint64p(i int) *int64 {
	v := int64(i)
	return &v
}

func mkintp(i int) *int {
	return &i
}

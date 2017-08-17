// +build !ignore_autogenerated_openshift

// This file was autogenerated by deepcopy-gen. Do not edit it manually!

package project

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	api "k8s.io/kubernetes/pkg/api"
	reflect "reflect"
)

func init() {
	SchemeBuilder.Register(RegisterDeepCopies)
}

// RegisterDeepCopies adds deep-copy functions to the given scheme. Public
// to allow building arbitrary schemes.
func RegisterDeepCopies(scheme *runtime.Scheme) error {
	return scheme.AddGeneratedDeepCopyFuncs(
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_project_Project, InType: reflect.TypeOf(&Project{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_project_ProjectList, InType: reflect.TypeOf(&ProjectList{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_project_ProjectRequest, InType: reflect.TypeOf(&ProjectRequest{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_project_ProjectReservation, InType: reflect.TypeOf(&ProjectReservation{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_project_ProjectReservationList, InType: reflect.TypeOf(&ProjectReservationList{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_project_ProjectReservationSpec, InType: reflect.TypeOf(&ProjectReservationSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_project_ProjectSpec, InType: reflect.TypeOf(&ProjectSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_project_ProjectStatus, InType: reflect.TypeOf(&ProjectStatus{})},
	)
}

// DeepCopy_project_Project is an autogenerated deepcopy function.
func DeepCopy_project_Project(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*Project)
		out := out.(*Project)
		*out = *in
		if newVal, err := c.DeepCopy(&in.ObjectMeta); err != nil {
			return err
		} else {
			out.ObjectMeta = *newVal.(*v1.ObjectMeta)
		}
		if err := DeepCopy_project_ProjectSpec(&in.Spec, &out.Spec, c); err != nil {
			return err
		}
		return nil
	}
}

// DeepCopy_project_ProjectList is an autogenerated deepcopy function.
func DeepCopy_project_ProjectList(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ProjectList)
		out := out.(*ProjectList)
		*out = *in
		if in.Items != nil {
			in, out := &in.Items, &out.Items
			*out = make([]Project, len(*in))
			for i := range *in {
				if err := DeepCopy_project_Project(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

// DeepCopy_project_ProjectRequest is an autogenerated deepcopy function.
func DeepCopy_project_ProjectRequest(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ProjectRequest)
		out := out.(*ProjectRequest)
		*out = *in
		if newVal, err := c.DeepCopy(&in.ObjectMeta); err != nil {
			return err
		} else {
			out.ObjectMeta = *newVal.(*v1.ObjectMeta)
		}
		return nil
	}
}

// DeepCopy_project_ProjectReservation is an autogenerated deepcopy function.
func DeepCopy_project_ProjectReservation(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ProjectReservation)
		out := out.(*ProjectReservation)
		*out = *in
		if newVal, err := c.DeepCopy(&in.ObjectMeta); err != nil {
			return err
		} else {
			out.ObjectMeta = *newVal.(*v1.ObjectMeta)
		}
		return nil
	}
}

// DeepCopy_project_ProjectReservationList is an autogenerated deepcopy function.
func DeepCopy_project_ProjectReservationList(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ProjectReservationList)
		out := out.(*ProjectReservationList)
		*out = *in
		if in.Items != nil {
			in, out := &in.Items, &out.Items
			*out = make([]ProjectReservation, len(*in))
			for i := range *in {
				if err := DeepCopy_project_ProjectReservation(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

// DeepCopy_project_ProjectReservationSpec is an autogenerated deepcopy function.
func DeepCopy_project_ProjectReservationSpec(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ProjectReservationSpec)
		out := out.(*ProjectReservationSpec)
		*out = *in
		return nil
	}
}

// DeepCopy_project_ProjectSpec is an autogenerated deepcopy function.
func DeepCopy_project_ProjectSpec(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ProjectSpec)
		out := out.(*ProjectSpec)
		*out = *in
		if in.Finalizers != nil {
			in, out := &in.Finalizers, &out.Finalizers
			*out = make([]api.FinalizerName, len(*in))
			copy(*out, *in)
		}
		return nil
	}
}

// DeepCopy_project_ProjectStatus is an autogenerated deepcopy function.
func DeepCopy_project_ProjectStatus(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ProjectStatus)
		out := out.(*ProjectStatus)
		*out = *in
		return nil
	}
}

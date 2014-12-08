/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"io"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/spf13/cobra"
)

func (f *Factory) NewCmdGet(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [(-o|--output=)json|yaml|...] <resource> [<id>]",
		Short: "Display one or many resources",
		Long: `Display one or many resources.

Possible resources include pods (po), replication controllers (rc), services
(se), minions (mi), or events (ev).

If you specify a Go template, you can use any fields defined for the API version
you are connecting to the server with.

Examples:
  $ kubectl get pods
  <list all pods in ps output format>

  $ kubectl get replicationController 1234-56-7890-234234-456456
  <list single replication controller in ps output format>

  $ kubectl get -o json pod 1234-56-7890-234234-456456
  <list single pod in json output format>`,
		Run: func(cmd *cobra.Command, args []string) {
			mapping, namespace, name := ResourceOrTypeFromArgs(cmd, args, f.Mapper)

			selector := GetFlagString(cmd, "selector")
			labelSelector, err := labels.ParseSelector(selector)
			checkErr(err)

			client, err := f.Client(cmd, mapping)
			checkErr(err)

			outputFormat := GetFlagString(cmd, "output")
			templateFile := GetFlagString(cmd, "template")
			defaultPrinter, err := f.Printer(cmd, mapping, GetFlagBool(cmd, "no-headers"))
			checkErr(err)

			outputVersion := GetFlagString(cmd, "output-version")
			if len(outputVersion) == 0 {
				outputVersion = mapping.APIVersion
			}

			printer, err := kubectl.GetPrinter(outputFormat, templateFile, outputVersion, mapping.ObjectConvertor, defaultPrinter)
			checkErr(err)

			restHelper := kubectl.NewRESTHelper(client, mapping)
			obj, err := restHelper.Get(namespace, name, labelSelector)
			checkErr(err)

			isWatch, isWatchOnly := GetFlagBool(cmd, "watch"), GetFlagBool(cmd, "watch-only")

			// print the current object
			if !isWatchOnly {
				if err := printer.PrintObj(obj, out); err != nil {
					checkErr(fmt.Errorf("unable to output the provided object: %v", err))
				}
			}

			// print watched changes
			if isWatch || isWatchOnly {
				rv, err := mapping.MetadataAccessor.ResourceVersion(obj)
				checkErr(err)

				w, err := restHelper.Watch(namespace, rv, labelSelector, labels.Everything())
				checkErr(err)

				kubectl.WatchLoop(w, printer, out)
			}
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output format: json|yaml|template|templatefile")
	cmd.Flags().String("output-version", "", "Output the formatted object with the given version (default api-version)")
	cmd.Flags().Bool("no-headers", false, "When using the default output, don't print headers")
	cmd.Flags().StringP("template", "t", "", "Template string or path to template file to use when --output=template or --output=templatefile")
	cmd.Flags().StringP("selector", "l", "", "Selector (label query) to filter on")
	cmd.Flags().BoolP("watch", "w", false, "After listing/getting the requested object, watch for changes.")
	cmd.Flags().Bool("watch-only", false, "Watch for changes to the requseted object(s), without listing/getting first.")
	return cmd
}

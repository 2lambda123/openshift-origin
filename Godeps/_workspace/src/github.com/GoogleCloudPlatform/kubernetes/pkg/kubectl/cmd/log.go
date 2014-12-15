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
	"io"

	"github.com/spf13/cobra"
)

func (f *Factory) NewCmdLog(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log <pod> <container>",
		Short: "Print the logs for a container in a pod",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 2 {
				usageError(cmd, "<pod> and <container> are required for log")
			}

			namespace := GetKubeNamespace(cmd)

			client, err := f.ClientBuilder.Client()
			checkErr(err)
			pod, err := client.Pods(namespace).Get(args[0])
			checkErr(err)

			data, err := client.RESTClient.Get().
				Path("proxy/minions").
				Path(pod.Status.Host).
				Path("containerLogs").
				Path(namespace).
				Path(args[0]).
				Path(args[1]).
				Do().
				Raw()
			checkErr(err)
			out.Write(data)

		},
	}
	return cmd
}

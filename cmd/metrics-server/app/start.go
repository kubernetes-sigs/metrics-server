// Copyright 2018 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"k8s.io/client-go/pkg/version"

	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"

	"sigs.k8s.io/metrics-server/cmd/metrics-server/app/options"
)

// NewMetricsServerCommand provides a CLI handler for the metrics server entrypoint
func NewMetricsServerCommand(stopCh <-chan struct{}) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Short: "Launch metrics-server",
		Long:  "Launch metrics-server",
		RunE: func(c *cobra.Command, args []string) error {
			if err := runCommand(opts, stopCh); err != nil {
				return err
			}
			return nil
		},
	}
	fs := cmd.Flags()
	nfs := opts.Flags()
	for _, f := range nfs.FlagSets {
		fs.AddFlagSet(f)
	}
	local := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(local)
	nfs.FlagSet("logging").AddGoFlagSet(local)

	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), nfs, cols)
		return nil
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), nfs, cols)
	})
	return cmd
}

func runCommand(o *options.Options, stopCh <-chan struct{}) error {
	if o.ShowVersion {
		fmt.Println(version.Get().GitVersion)
		os.Exit(0)
	}

	errors := o.Validate()
	if len(errors) > 0 {
		return errors[0]
	}

	config, err := o.ServerConfig()

	if err != nil {
		return err
	}

	s, err := config.Complete()

	if err != nil {
		return err
	}

	return s.RunUntil(stopCh)
}

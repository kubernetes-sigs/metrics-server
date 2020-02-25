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
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"k8s.io/apiserver/pkg/server/healthz"

	"sigs.k8s.io/metrics-server/cmd/metrics-server/app/options"
	"sigs.k8s.io/metrics-server/pkg/version"
)

// NewMetricsServerCommand provides a CLI handler for the metrics server entrypoint
func NewMetricsServerCommand(out, errOut io.Writer, stopCh <-chan struct{}) *cobra.Command {
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
	opts.Flags(cmd)
	return cmd
}

func runCommand(o *options.Options, stopCh <-chan struct{}) error {
	if o.ShowVersion {
		fmt.Println(version.VersionInfo())
		os.Exit(0)
	}
	config, err := o.MetricsServerConfig()
	if err != nil {
		return err
	}
	config.Apiserver.EnableMetrics = true
	// Use protobufs for communication with apiserver
	config.Rest.ContentType = "application/vnd.kubernetes.protobuf"

	ms, err := config.Complete()
	if err != nil {
		return err
	}

	err = ms.AddHealthChecks(healthz.NamedCheck("healthz", ms.CheckHealth))
	if err != nil {
		return err
	}

	return ms.RunUntil(stopCh)
}

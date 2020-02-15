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

	"sigs.k8s.io/metrics-server/pkg/version"
)

// NewCommandStartMetricsServer provides a CLI handler for the metrics server entrypoint
func NewCommandStartMetricsServer(out, errOut io.Writer, stopCh <-chan struct{}) *cobra.Command {
	o := NewMetricsServerOptions()

	cmd := &cobra.Command{
		Short: "Launch metrics-server",
		Long:  "Launch metrics-server",
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Run(stopCh); err != nil {
				return err
			}
			return nil
		},
	}
}

func (o MetricsServerOptions) Run(stopCh <-chan struct{}) error {
	if o.ShowVersion {
		fmt.Println(version.VersionInfo())
		os.Exit(0)
	}

	// grab the config for the API server
	config, err := o.Config()
	if err != nil {
		return err
	}
	config.GenericConfig.EnableMetrics = true

	// set up the client config

	// complete the config to get an API server
	server, err := config.Complete(informerFactory).New()
	if err != nil {
		return err
	}

	// add health checks
	err = server.AddHealthChecks(healthz.NamedCheck("healthz", mgr.CheckHealth))
	if err != nil {
		return err
	}

	// run everything (the apiserver runs the shared informer factory for us)
	mgr.RunUntil(stopCh)
	return server.GenericAPIServer.PrepareRun().Run(stopCh)
}

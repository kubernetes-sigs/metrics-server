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

package main

import (
	"os"
	"runtime"
	"runtime/debug"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"sigs.k8s.io/metrics-server/cmd/metrics-server/app"
)

const DefaultMemLimitRatio = 0.9

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	if len(os.Getenv("GOMEMLIMIT")) == 0 {
		if _, err := memlimit.SetGoMemLimitWithOpts(
			memlimit.WithRatio(DefaultMemLimitRatio),
			memlimit.WithProvider(
				memlimit.ApplyFallback(
					memlimit.FromCgroup,
					memlimit.FromSystem,
				),
			),
		); err != nil {
			klog.Warningf("Failed to set GOMEMLIMIT automatically. GOMAXPROCS set to %d", debug.SetMemoryLimit(-1))
		}
	}

	cmd := app.NewMetricsServerCommand(genericapiserver.SetupSignalHandler())
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}

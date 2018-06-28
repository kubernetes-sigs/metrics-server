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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"github.com/kubernetes-incubator/metrics-server/metrics/cmd/heapster-apiserver/app"
	"github.com/kubernetes-incubator/metrics-server/metrics/manager"
	"github.com/kubernetes-incubator/metrics-server/metrics/options"
	"github.com/kubernetes-incubator/metrics-server/metrics/provider"
	"github.com/kubernetes-incubator/metrics-server/metrics/sources"
	"github.com/kubernetes-incubator/metrics-server/metrics/sources/summary"
	"github.com/kubernetes-incubator/metrics-server/version"
	"k8s.io/apimachinery/pkg/util/wait"
	//"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/apiserver/pkg/util/flag"
	"k8s.io/apiserver/pkg/util/logs"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// initialize options
	opt := options.NewHeapsterRunOptions()
	opt.AddFlags(pflag.CommandLine)

	flag.InitFlags()

	if opt.Version {
		fmt.Println(version.VersionInfo())
		os.Exit(0)
	}

	logs.InitLogs()
	defer logs.FlushLogs()

	glog.Infof(strings.Join(os.Args, " "))
	glog.Infof("Metrics Server version %v", version.MetricsServerVersion)
	if err := validateFlags(opt); err != nil {
		glog.Fatal(err)
	}

	// setup kubeconfig
	kubeConfig, err := getClientConfig(opt.Kubeconfig)
	if err != nil {
		glog.Fatalf("unable to construct main client config: %v", err)
	}

	// make an informer factory
	kubeClient := kubernetes.NewForConfigOrDie(kubeConfig)
	// we should never need to resync, since we're not worried about missing events, and resync is actually for
	// regular interval-based reconciliation these days, so set the default resync interval to 0
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	podLister := informerFactory.Core().V1().Pods().Lister()
	nodeLister := informerFactory.Core().V1().Nodes().Lister()

	// set up the source manager and the in-memory sink
	sourceManager := createSourceManagerOrDie(nodeLister, kubeConfig, opt.KubeletPort, opt.InsecureKubelet)
	metricSink, metricsProvider := provider.NewSinkProvider()

	mgr := manager.NewManager(sourceManager, metricSink,
		opt.MetricResolution, manager.DefaultScrapeOffset, manager.DefaultMaxParallelism)
	if err != nil {
		glog.Fatalf("Failed to create main manager: %v", err)
	}

	// Set up the API server
	server, err := app.NewHeapsterApiServer(opt, metricsProvider, metricsProvider, nodeLister, podLister)
	if err != nil {
		glog.Fatalf("Could not create the API server: %v", err)
	}

	// TODO: fix this
	//server.AddHealthzChecks(healthzChecker(metricsProvider))

	// Run the informers, and wait till they're synced
	go informerFactory.Start(wait.NeverStop)
	for informerType, started := range informerFactory.WaitForCacheSync(wait.NeverStop) {
		if !started {
			glog.Fatalf("Unable to start informer (start listing objects) for %s", informerType.String())
		}
	}

	// Run everything else
	glog.Infof("Starting Heapster API server...")
	mgr.RunUntil(wait.NeverStop)
	glog.Fatal(server.RunServer())
}

func getClientConfig(kubeConfigPath string) (*rest.Config, error) {
	if kubeConfigPath == "" {
		authConf, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("unable to create in-cluster client config: %v", err)
		}
		return authConf, err
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	authConf, err := loader.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to create client config from file %q: %v", kubeConfigPath, err)
	}

	return authConf, nil
}

func createSourceManagerOrDie(nodeLister v1listers.NodeLister, kubeConfig *rest.Config, kubeletPort int, insecureKubelet bool) sources.MetricSource {
	kubeletConfig := summary.GetKubeletConfig(kubeConfig, kubeletPort, insecureKubelet)
	kubeletClient, err := summary.KubeletClientFor(kubeletConfig)
	if err != nil {
		glog.Fatalf("Failed to create kubelet client: %v", err)
	}
	sourceProvider := summary.NewSummaryProvider(nodeLister, kubeletClient)
	sourceManager, err := sources.NewSourceManager(sourceProvider, sources.DefaultMetricsScrapeTimeout)
	if err != nil {
		glog.Fatalf("Failed to create source manager: %v", err)
	}
	return sourceManager
}

const (
	minMetricsCount = 1
	maxMetricsDelay = 3 * time.Minute
)

// TODO: fix this
/*
func healthzChecker(metricProv provider.MetricsProvider) healthz.HealthzChecker {
	return healthz.NamedCheck("healthz", func(r *http.Request) error {
		batch := metricSink.GetLatestDataBatch()
		if batch == nil {
			return errors.New("could not get the latest data batch")
		}
		if time.Since(batch.Timestamp) > maxMetricsDelay {
			message := fmt.Sprintf("No current data batch available (latest: %s).", batch.Timestamp.String())
			glog.Warningf(message)
			return errors.New(message)
		}
		if len(batch.MetricSets) < minMetricsCount {
			message := fmt.Sprintf("Not enough metrics found in the latest data batch: %d (expected min. %d) %s", len(batch.MetricSets), minMetricsCount, batch.Timestamp.String())
			glog.Warningf(message)
			return errors.New(message)
		}
		return nil
	})
}*/

func validateFlags(opt *options.HeapsterRunOptions) error {
	if opt.MetricResolution < 5*time.Second {
		return fmt.Errorf("metric resolution needs to be greater than 5 seconds - %d", opt.MetricResolution)
	}
	return nil
}

// Copyright 2014 Google Inc. All Rights Reserved.
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

//go:generate ./hooks/run_extpoints.sh

package main

import (
	"fmt"
	"os"
	"runtime"
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
	"github.com/kubernetes-incubator/metrics-server/metrics/util"
	"github.com/kubernetes-incubator/metrics-server/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	//"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/apiserver/pkg/util/flag"
	"k8s.io/apiserver/pkg/util/logs"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	opt := options.NewHeapsterRunOptions()
	opt.AddFlags(pflag.CommandLine)

	flag.InitFlags()

	if opt.Version {
		fmt.Println(version.VersionInfo())
		os.Exit(0)
	}

	logs.InitLogs()
	defer logs.FlushLogs()

	setLabelSeperator(opt)
	setMaxProcs(opt)
	glog.Infof(strings.Join(os.Args, " "))
	glog.Infof("Metrics Server version %v", version.MetricsServerVersion)
	if err := validateFlags(opt); err != nil {
		glog.Fatal(err)
	}

	kubeConfig, err := getClientConfig(opt.Kubeconfig)
	if err != nil {
		glog.Fatalf("unable to construct main client config: %v", err)
	}
	restClient, err := rest.RESTClientFor(kubeConfig)
	if err != nil {
		glog.Fatalf("unable to construct main REST client: %v", err)
	}

	sourceManager := createSourceManagerOrDie(restClient, kubeConfig, opt.KubeletPort, opt.InsecureKubelet)
	metricSink, metricsProvider := provider.NewSinkProvider()

	mgr := manager.NewManager(sourceManager, metricSink,
		opt.MetricResolution, manager.DefaultScrapeOffset, manager.DefaultMaxParallelism)
	if err != nil {
		glog.Fatalf("Failed to create main manager: %v", err)
	}
	mgr.RunUntil(wait.NeverStop)

	// Run API server
	podLister, nodeLister := getListersOrDie(restClient)
	server, err := app.NewHeapsterApiServer(opt, metricsProvider, metricsProvider, nodeLister, podLister)
	if err != nil {
		glog.Fatalf("Could not create the API server: %v", err)
	}

	// TODO: fix this
	//server.AddHealthzChecks(healthzChecker(metricsProvider))

	glog.Infof("Starting Heapster API server...")
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

func createSourceManagerOrDie(restClient rest.Interface, kubeConfig *rest.Config, kubeletPort int, insecureKubelet bool) sources.MetricSource {
	kubeletConfig := summary.GetKubeletConfig(kubeConfig, kubeletPort, insecureKubelet)
	kubeletClient, err := summary.KubeletClientFor(kubeletConfig)
	if err != nil {
		glog.Fatalf("Failed to create kubelet client: %v", err)
	}
	sourceProvider, err := summary.NewSummaryProvider(restClient, kubeletClient)
	if err != nil {
		glog.Fatalf("Failed to create source provide: %v", err)
	}
	sourceManager, err := sources.NewSourceManager(sourceProvider, sources.DefaultMetricsScrapeTimeout)
	if err != nil {
		glog.Fatalf("Failed to create source manager: %v", err)
	}
	return sourceManager
}

func getListersOrDie(client rest.Interface) (v1listers.PodLister, v1listers.NodeLister) {
	podLister, err := getPodLister(client)
	if err != nil {
		glog.Fatalf("Failed to create podLister: %v", err)
	}
	nodeLister, _, err := util.GetNodeLister(client)
	if err != nil {
		glog.Fatalf("Failed to create nodeLister: %v", err)
	}
	return podLister, nodeLister
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

func getPodLister(restClient rest.Interface) (v1listers.PodLister, error) {
	lw := cache.NewListWatchFromClient(restClient, "pods", corev1.NamespaceAll, fields.Everything())
	store := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	podLister := v1listers.NewPodLister(store)
	reflector := cache.NewReflector(lw, &corev1.Pod{}, store, time.Hour)
	go reflector.Run(wait.NeverStop)
	return podLister, nil
}

func validateFlags(opt *options.HeapsterRunOptions) error {
	if opt.MetricResolution < 5*time.Second {
		return fmt.Errorf("metric resolution needs to be greater than 5 seconds - %d", opt.MetricResolution)
	}
	return nil
}

func setMaxProcs(opt *options.HeapsterRunOptions) {
	// Allow as many threads as we have cores unless the user specified a value.
	var numProcs int
	if opt.MaxProcs < 1 {
		numProcs = runtime.NumCPU()
	} else {
		numProcs = opt.MaxProcs
	}
	runtime.GOMAXPROCS(numProcs)

	// Check if the setting was successful.
	actualNumProcs := runtime.GOMAXPROCS(0)
	if actualNumProcs != numProcs {
		glog.Warningf("Specified max procs of %d but using %d", numProcs, actualNumProcs)
	}
}

func setLabelSeperator(opt *options.HeapsterRunOptions) {
	util.SetLabelSeperator(opt.LabelSeperator)
}

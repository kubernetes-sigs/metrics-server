module sigs.k8s.io/metrics-server

go 1.13.8

require (
	github.com/NYTimes/gziphandler v1.0.1 // indirect
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/go-openapi/spec v0.19.3
	github.com/gogo/protobuf v1.3.0 // indirect
	github.com/golang/groupcache v0.0.0-20191027212112-611e8accdfc9 // indirect
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.11.3 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	golang.org/x/crypto v0.0.0-20191029031824-8986dd9e96cf // indirect
	golang.org/x/net v0.0.0-20191028085509-fe3aa8a45271 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	google.golang.org/appengine v1.6.5 // indirect
	google.golang.org/genproto v0.0.0-20191028173616-919d9bdd9fe6 // indirect
	google.golang.org/grpc v1.24.0 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/apiserver v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/component-base v0.0.0
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	k8s.io/kubernetes v1.17.0-rc.1
	k8s.io/metrics v0.0.0
	sigs.k8s.io/kind v0.6.0 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.0.0-20191122220107-b5267f2975e0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20191122222427-64482ea217ff
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20191121175448-79c2a76c473a
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20191122221311-9d521947b1e1
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20191122222818-9150eb3ded31
	k8s.io/client-go => k8s.io/client-go v0.0.0-20191122220542-ed16ecbdf3a0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20191122224014-59d76ad41876
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20191122223827-289de4a64c1c
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20191121175249-e95606b614f0
	k8s.io/component-base => k8s.io/component-base v0.0.0-20191122220729-2684fb322cb9
	k8s.io/cri-api => k8s.io/cri-api v0.0.0-20191121183020-775aa3c1cf73
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20191122224157-518651485193
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20191122221605-1e8d331e4dcc
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20191122223648-5cfd5067047c
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20191122223145-16f2c0c680a0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20191122223507-ac656df73673
	k8s.io/kubectl => k8s.io/kubectl v0.0.0-20191122225023-1e3c8b70f494
	k8s.io/kubelet => k8s.io/kubelet v0.0.0-20191122223325-9316382755ad
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20191122224431-860df69ff5cc
	k8s.io/metrics => k8s.io/metrics v0.0.0-20191122222628-19ed227de2b6
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20191122221846-294c70c3d5d4
)

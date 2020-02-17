module sigs.k8s.io/metrics-server

go 1.13.8

require (
	github.com/go-openapi/spec v0.19.3
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/prometheus/client_golang v1.0.0
	github.com/spf13/cobra v0.0.5
	k8s.io/api v0.17.3
	k8s.io/apimachinery v0.17.3
	k8s.io/apiserver v0.17.3
	k8s.io/client-go v0.17.3
	k8s.io/component-base v0.17.3
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1 v0.0.0
	k8s.io/metrics v0.17.3
	sigs.k8s.io/kind v0.7.0 // indirect
)

replace k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1 => ./vendor/k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1

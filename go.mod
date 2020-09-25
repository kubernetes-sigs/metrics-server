module sigs.k8s.io/metrics-server

go 1.14

require (
	github.com/go-openapi/spec v0.19.8
	github.com/go-openapi/swag v0.19.9 // indirect
	github.com/google/addlicense v0.0.0-20200906110928-a0294312aa76
	github.com/google/go-cmp v0.4.0
	github.com/mailru/easyjson v0.7.1
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.7.0
	github.com/prometheus/common v0.10.0
	github.com/spf13/cobra v1.0.0
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	golang.org/x/text v0.3.3 // indirect
	k8s.io/api v0.18.5
	k8s.io/apimachinery v0.18.5
	k8s.io/apiserver v0.18.5
	k8s.io/client-go v0.18.5
	k8s.io/component-base v0.18.5
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200410145947-61e04a5be9a6
	k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1 v0.0.0
	k8s.io/metrics v0.18.5
)

replace k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1 => ./vendor/k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1

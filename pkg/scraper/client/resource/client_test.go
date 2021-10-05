// Copyright 2021 The Kubernetes Authors.
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

package resource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkKubeletClient_GetMetrics(b *testing.B) {
	s := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(resourceResponse))
	}))
	defer s.Close()

	c := newClient(s.Client(), nil, 0, "http", false)
	b.ResetTimer()
	b.ReportAllocs()

	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		_, err := c.getMetrics(ctx, s.URL, "node1")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestGetMetrics(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(resourceResponse))
	}))
	defer s.Close()

	c := newClient(s.Client(), nil, 0, "http", false)

	ctx := context.Background()

	ms, err := c.getMetrics(ctx, s.URL, "node1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms.Nodes) != 1 {
		t.Fatalf("No node metrics")
	}
	if len(ms.Pods) != 70 {
		t.Fatalf("Unexpected number of pods, want: %d, got %d", 70, len(ms.Pods))
	}
}

const resourceResponse = `
# HELP container_cpu_usage_seconds_total [ALPHA] Cumulative cpu time consumed by the container in core-seconds
# TYPE container_cpu_usage_seconds_total counter
container_cpu_usage_seconds_total{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 4.710169 1633253812125
container_cpu_usage_seconds_total{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-kjc9p"} 4.612431 1633253814708
container_cpu_usage_seconds_total{container="etcd",namespace="kube-system",pod="etcd-e2e-control-plane"} 31.349402 1633253814387
container_cpu_usage_seconds_total{container="kindnet-cni",namespace="kube-system",pod="kindnet-blbtj"} 0.960551 1633253806022
container_cpu_usage_seconds_total{container="kube-apiserver",namespace="kube-system",pod="kube-apiserver-e2e-control-plane"} 86.500663 1633253814382
container_cpu_usage_seconds_total{container="kube-controller-manager",namespace="kube-system",pod="kube-controller-manager-e2e-control-plane"} 22.161117 1633253800551
container_cpu_usage_seconds_total{container="kube-proxy",namespace="kube-system",pod="kube-proxy-7vcfn"} 1.24067 1633253816654
container_cpu_usage_seconds_total{container="kube-scheduler",namespace="kube-system",pod="kube-scheduler-e2e-control-plane"} 5.518247 1633253814691
container_cpu_usage_seconds_total{container="local-path-provisioner",namespace="local-path-storage",pod="local-path-provisioner-547f784dff-dxdq4"} 1.565106 1633253813495
container_cpu_usage_seconds_total{container="metrics-server",namespace="kube-system",pod="metrics-server-66547b68cb-z9lkj"} 5.102405 1633253811469
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-1-77f658b69-2msf9"} 1.421659 1633253814014
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-1-77f658b69-8pggd"} 1.460639 1633253814085
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-1-77f658b69-jxb4z"} 1.445969 1633253811661
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-1-77f658b69-mmrsl"} 1.37527 1633253811647
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-1-77f658b69-nll2q"} 1.396854 1633253812960
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-1-77f658b69-vkwpw"} 1.32051 1633253810478
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-10-d7fbc765-6xfst"} 0.754433 1633253800367
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-10-d7fbc765-7jn5x"} 0.830579 1633253813719
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-10-d7fbc765-bw4q9"} 0.732721 1633253805453
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-10-d7fbc765-fl9z6"} 0.823433 1633253814663
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-10-d7fbc765-pqtkf"} 0.895272 1633253808738
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-10-d7fbc765-tbglr"} 1.030333 1633253815547
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-2-674945c58b-d7hx7"} 1.204399 1633253800660
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-2-674945c58b-dtm8k"} 1.087 1633253810574
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-2-674945c58b-flsjc"} 1.353367 1633253810212
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-2-674945c58b-lkx6j"} 1.447713 1633253813955
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-2-674945c58b-lndj5"} 1.430566 1633253807924
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-2-674945c58b-td4wj"} 1.325785 1633253809493
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-2bxmb"} 1.31906 1633253812047
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-5bd7k"} 1.284566 1633253814603
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-6nt7q"} 1.120459 1633253806084
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-hl2cj"} 1.18968 1633253803489
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-n4q5l"} 1.050332 1633253809893
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-ws5vx"} 1.505446 1633253814612
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-4-5476d44584-7sh8g"} 1.124649 1633253811819
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-4-5476d44584-gkjfq"} 1.421147 1633253812081
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-4-5476d44584-l47mp"} 1.149942 1633253805614
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-4-5476d44584-nc4rz"} 0.735222 1633253800957
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-4-5476d44584-nvdb4"} 0.761841 1633253815555
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-4-5476d44584-vm68w"} 0.871017 1633253808732
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-dgljs"} 1.23978 1633253813600
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-fbtbt"} 1.306255 1633253816986
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-ftvv5"} 0.949653 1633253812706
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-kbmqg"} 0.904106 1633253815639
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-kzltf"} 0.669691 1633253807507
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-wjrxw"} 0.985654 1633253803848
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-6-776d465d58-26h78"} 1.181718 1633253807152
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-6-776d465d58-4m6rl"} 0.749545 1633253808336
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-6-776d465d58-4tqxg"} 1.321834 1633253813908
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-6-776d465d58-cwbbq"} 0.969905 1633253805682
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-6-776d465d58-kt78n"} 1.169758 1633253808638
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-6-776d465d58-lmpb2"} 1.282218 1633253813030
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-89sv8"} 1.214348 1633253817851
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-cqv9q"} 0.996442 1633253814808
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-jqxss"} 0.968919 1633253809324
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-mv8q5"} 0.894731 1633253811147
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-qxwbp"} 1.128528 1633253810467
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-xk856"} 1.357432 1633253814131
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-2fzbb"} 1.122017 1633253811802
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-8vzvk"} 1.121966 1633253809578
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-hqfr6"} 0.855502 1633253812852
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-jwb7f"} 1.134202 1633253804939
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-l8c58"} 0.685545 1633253808208
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-q8cv5"} 0.745713 1633253812479
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-9-5744dbc77c-2wlvs"} 0.910859 1633253812336
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-9-5744dbc77c-5hd96"} 0.983159 1633253798973
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-9-5744dbc77c-b6bfc"} 0.75489 1633253811224
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-9-5744dbc77c-bqtbg"} 0.907737 1633253805268
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-9-5744dbc77c-lm28m"} 1.033233 1633253811391
container_cpu_usage_seconds_total{container="sleeper",namespace="sleeper-1",pod="sleeper-9-5744dbc77c-q6mg9"} 1.055063 1633253813998
# HELP container_memory_working_set_bytes [ALPHA] Current working set of the container in bytes
# TYPE container_memory_working_set_bytes gauge
container_memory_working_set_bytes{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 1.253376e+07 1633253812125
container_memory_working_set_bytes{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-kjc9p"} 1.1841536e+07 1633253814708
container_memory_working_set_bytes{container="etcd",namespace="kube-system",pod="etcd-e2e-control-plane"} 4.0783872e+07 1633253814387
container_memory_working_set_bytes{container="kindnet-cni",namespace="kube-system",pod="kindnet-blbtj"} 8.495104e+06 1633253806022
container_memory_working_set_bytes{container="kube-apiserver",namespace="kube-system",pod="kube-apiserver-e2e-control-plane"} 3.53316864e+08 1633253814382
container_memory_working_set_bytes{container="kube-controller-manager",namespace="kube-system",pod="kube-controller-manager-e2e-control-plane"} 6.5728512e+07 1633253800551
container_memory_working_set_bytes{container="kube-proxy",namespace="kube-system",pod="kube-proxy-7vcfn"} 1.3123584e+07 1633253816654
container_memory_working_set_bytes{container="kube-scheduler",namespace="kube-system",pod="kube-scheduler-e2e-control-plane"} 2.4244224e+07 1633253814691
container_memory_working_set_bytes{container="local-path-provisioner",namespace="local-path-storage",pod="local-path-provisioner-547f784dff-dxdq4"} 7.114752e+06 1633253813495
container_memory_working_set_bytes{container="metrics-server",namespace="kube-system",pod="metrics-server-66547b68cb-z9lkj"} 1.8845696e+07 1633253811469
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-1-77f658b69-2msf9"} 495616 1633253814014
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-1-77f658b69-8pggd"} 503808 1633253814085
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-1-77f658b69-jxb4z"} 520192 1633253811661
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-1-77f658b69-mmrsl"} 487424 1633253811647
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-1-77f658b69-nll2q"} 503808 1633253812960
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-1-77f658b69-vkwpw"} 315392 1633253810478
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-10-d7fbc765-6xfst"} 495616 1633253800367
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-10-d7fbc765-7jn5x"} 499712 1633253813719
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-10-d7fbc765-bw4q9"} 380928 1633253805453
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-10-d7fbc765-fl9z6"} 528384 1633253814663
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-10-d7fbc765-pqtkf"} 495616 1633253808738
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-10-d7fbc765-tbglr"} 364544 1633253815547
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-2-674945c58b-d7hx7"} 491520 1633253800660
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-2-674945c58b-dtm8k"} 512000 1633253810574
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-2-674945c58b-flsjc"} 503808 1633253810212
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-2-674945c58b-lkx6j"} 253952 1633253813955
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-2-674945c58b-lndj5"} 376832 1633253807924
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-2-674945c58b-td4wj"} 360448 1633253809493
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-2bxmb"} 397312 1633253812047
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-5bd7k"} 311296 1633253814603
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-6nt7q"} 368640 1633253806084
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-hl2cj"} 356352 1633253803489
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-n4q5l"} 503808 1633253809893
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-ws5vx"} 417792 1633253814612
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-4-5476d44584-7sh8g"} 499712 1633253811819
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-4-5476d44584-gkjfq"} 360448 1633253812081
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-4-5476d44584-l47mp"} 360448 1633253805614
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-4-5476d44584-nc4rz"} 507904 1633253800957
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-4-5476d44584-nvdb4"} 356352 1633253815555
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-4-5476d44584-vm68w"} 491520 1633253808732
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-dgljs"} 495616 1633253813600
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-fbtbt"} 487424 1633253816986
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-ftvv5"} 491520 1633253812706
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-kbmqg"} 364544 1633253815639
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-kzltf"} 507904 1633253807507
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-wjrxw"} 507904 1633253803848
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-6-776d465d58-26h78"} 487424 1633253807152
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-6-776d465d58-4m6rl"} 487424 1633253808336
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-6-776d465d58-4tqxg"} 241664 1633253813908
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-6-776d465d58-cwbbq"} 503808 1633253805682
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-6-776d465d58-kt78n"} 339968 1633253808638
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-6-776d465d58-lmpb2"} 360448 1633253813030
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-89sv8"} 397312 1633253817851
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-cqv9q"} 483328 1633253814808
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-jqxss"} 499712 1633253809324
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-mv8q5"} 495616 1633253811147
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-qxwbp"} 462848 1633253810467
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-xk856"} 348160 1633253814131
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-2fzbb"} 507904 1633253811802
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-8vzvk"} 507904 1633253809578
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-hqfr6"} 507904 1633253812852
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-jwb7f"} 503808 1633253804939
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-l8c58"} 512000 1633253808208
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-q8cv5"} 245760 1633253812479
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-9-5744dbc77c-2wlvs"} 479232 1633253812336
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-9-5744dbc77c-5hd96"} 503808 1633253798973
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-9-5744dbc77c-b6bfc"} 372736 1633253811224
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-9-5744dbc77c-bqtbg"} 516096 1633253805268
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-9-5744dbc77c-lm28m"} 245760 1633253811391
container_memory_working_set_bytes{container="sleeper",namespace="sleeper-1",pod="sleeper-9-5744dbc77c-q6mg9"} 495616 1633253813998
# HELP node_cpu_usage_seconds_total [ALPHA] Cumulative cpu time consumed by the node in core-seconds
# TYPE node_cpu_usage_seconds_total counter
node_cpu_usage_seconds_total 357.35491 1633253809720
# HELP node_memory_working_set_bytes [ALPHA] Current working set of the node in bytes
# TYPE node_memory_working_set_bytes gauge
node_memory_working_set_bytes 1.616273408e+09 1633253809720
# HELP pod_cpu_usage_seconds_total [ALPHA] Cumulative cpu time consumed by the pod in core-seconds
# TYPE pod_cpu_usage_seconds_total counter
pod_cpu_usage_seconds_total{namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 4.67812 1633253803935
pod_cpu_usage_seconds_total{namespace="kube-system",pod="coredns-558bd4d5db-kjc9p"} 4.665972 1633253817681
pod_cpu_usage_seconds_total{namespace="kube-system",pod="etcd-e2e-control-plane"} 31.126403 1633253810138
pod_cpu_usage_seconds_total{namespace="kube-system",pod="kindnet-blbtj"} 1.027602 1633253817933
pod_cpu_usage_seconds_total{namespace="kube-system",pod="kube-apiserver-e2e-control-plane"} 85.267128 1633253804369
pod_cpu_usage_seconds_total{namespace="kube-system",pod="kube-controller-manager-e2e-control-plane"} 22.441626 1633253809849
pod_cpu_usage_seconds_total{namespace="kube-system",pod="kube-proxy-7vcfn"} 1.265738 1633253810760
pod_cpu_usage_seconds_total{namespace="kube-system",pod="kube-scheduler-e2e-control-plane"} 5.555036 1633253817325
pod_cpu_usage_seconds_total{namespace="kube-system",pod="metrics-server-66547b68cb-z9lkj"} 5.054846 1633253801239
pod_cpu_usage_seconds_total{namespace="local-path-storage",pod="local-path-provisioner-547f784dff-dxdq4"} 1.597405 1633253815042
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-1-77f658b69-2msf9"} 1.561556 1633253812737
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-1-77f658b69-8pggd"} 1.569844 1633253809753
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-1-77f658b69-jxb4z"} 1.526575 1633253804005
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-1-77f658b69-mmrsl"} 1.548705 1633253815036
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-1-77f658b69-nll2q"} 1.483691 1633253806500
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-1-77f658b69-vkwpw"} 1.507218 1633253811839
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-10-d7fbc765-6xfst"} 1.018288 1633253816329
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-10-d7fbc765-7jn5x"} 0.998706 1633253817028
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-10-d7fbc765-bw4q9"} 0.891149 1633253808302
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-10-d7fbc765-fl9z6"} 0.940191 1633253810241
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-10-d7fbc765-pqtkf"} 1.040135 1633253808802
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-10-d7fbc765-tbglr"} 1.051168 1633253803108
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-2-674945c58b-d7hx7"} 1.457525 1633253814517
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-2-674945c58b-dtm8k"} 1.258563 1633253812716
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-2-674945c58b-flsjc"} 1.536589 1633253816461
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-2-674945c58b-lkx6j"} 1.467734 1633253802353
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-2-674945c58b-lndj5"} 1.592954 1633253811121
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-2-674945c58b-td4wj"} 1.492984 1633253810876
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-2bxmb"} 1.485834 1633253813338
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-5bd7k"} 1.387102 1633253810836
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-6nt7q"} 1.328426 1633253812098
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-hl2cj"} 1.411231 1633253816256
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-n4q5l"} 1.205433 1633253809484
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-ws5vx"} 1.600207 1633253810601
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-4-5476d44584-7sh8g"} 1.241606 1633253809958
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-4-5476d44584-gkjfq"} 1.574685 1633253815105
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-4-5476d44584-l47mp"} 1.352205 1633253814108
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-4-5476d44584-nc4rz"} 0.970371 1633253809267
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-4-5476d44584-nvdb4"} 0.855228 1633253806701
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-4-5476d44584-vm68w"} 1.094652 1633253816933
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-dgljs"} 1.290654 1633253804056
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-fbtbt"} 1.430815 1633253812960
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-ftvv5"} 1.060868 1633253807769
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-kbmqg"} 1.014714 1633253809149
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-kzltf"} 0.788778 1633253800470
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-wjrxw"} 1.138528 1633253804706
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-6-776d465d58-26h78"} 1.387431 1633253813524
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-6-776d465d58-4m6rl"} 0.941748 1633253811712
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-6-776d465d58-4tqxg"} 1.462956 1633253817431
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-6-776d465d58-cwbbq"} 1.177253 1633253814088
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-6-776d465d58-kt78n"} 1.320063 1633253811046
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-6-776d465d58-lmpb2"} 1.325611 1633253802851
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-89sv8"} 1.245133 1633253805787
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-cqv9q"} 1.143972 1633253817809
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-jqxss"} 1.114323 1633253808832
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-mv8q5"} 1.031236 1633253809841
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-qxwbp"} 1.272054 1633253810863
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-xk856"} 1.466209 1633253811863
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-2fzbb"} 1.161836 1633253803308
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-8vzvk"} 1.223092 1633253807065
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-hqfr6"} 0.967997 1633253810300
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-jwb7f"} 1.274742 1633253809076
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-l8c58"} 0.804828 1633253804298
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-q8cv5"} 0.827456 1633253803269
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-9-5744dbc77c-2wlvs"} 1.079737 1633253817837
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-9-5744dbc77c-5hd96"} 1.231893 1633253814168
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-9-5744dbc77c-b6bfc"} 0.926712 1633253812147
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-9-5744dbc77c-bqtbg"} 1.023795 1633253803171
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-9-5744dbc77c-lm28m"} 1.161298 1633253810755
pod_cpu_usage_seconds_total{namespace="sleeper-1",pod="sleeper-9-5744dbc77c-q6mg9"} 1.065213 1633253800817
# HELP pod_memory_working_set_bytes [ALPHA] Current working set of the pod in bytes
# TYPE pod_memory_working_set_bytes gauge
pod_memory_working_set_bytes{namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 1.2627968e+07 1633253803935
pod_memory_working_set_bytes{namespace="kube-system",pod="coredns-558bd4d5db-kjc9p"} 1.2075008e+07 1633253817681
pod_memory_working_set_bytes{namespace="kube-system",pod="etcd-e2e-control-plane"} 4.048896e+07 1633253810138
pod_memory_working_set_bytes{namespace="kube-system",pod="kindnet-blbtj"} 8.708096e+06 1633253817933
pod_memory_working_set_bytes{namespace="kube-system",pod="kube-apiserver-e2e-control-plane"} 3.53476608e+08 1633253804369
pod_memory_working_set_bytes{namespace="kube-system",pod="kube-controller-manager-e2e-control-plane"} 6.5978368e+07 1633253809849
pod_memory_working_set_bytes{namespace="kube-system",pod="kube-proxy-7vcfn"} 1.3336576e+07 1633253810760
pod_memory_working_set_bytes{namespace="kube-system",pod="kube-scheduler-e2e-control-plane"} 2.445312e+07 1633253817325
pod_memory_working_set_bytes{namespace="kube-system",pod="metrics-server-66547b68cb-z9lkj"} 1.8853888e+07 1633253801239
pod_memory_working_set_bytes{namespace="local-path-storage",pod="local-path-provisioner-547f784dff-dxdq4"} 7.344128e+06 1633253815042
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-1-77f658b69-2msf9"} 782336 1633253812737
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-1-77f658b69-8pggd"} 716800 1633253809753
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-1-77f658b69-jxb4z"} 737280 1633253804005
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-1-77f658b69-mmrsl"} 786432 1633253815036
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-1-77f658b69-nll2q"} 737280 1633253806500
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-1-77f658b69-vkwpw"} 581632 1633253811839
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-10-d7fbc765-6xfst"} 606208 1633253816329
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-10-d7fbc765-7jn5x"} 606208 1633253817028
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-10-d7fbc765-bw4q9"} 626688 1633253808302
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-10-d7fbc765-fl9z6"} 585728 1633253810241
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-10-d7fbc765-pqtkf"} 729088 1633253808802
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-10-d7fbc765-tbglr"} 704512 1633253803108
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-2-674945c58b-d7hx7"} 720896 1633253814517
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-2-674945c58b-dtm8k"} 602112 1633253812716
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-2-674945c58b-flsjc"} 618496 1633253816461
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-2-674945c58b-lkx6j"} 581632 1633253802353
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-2-674945c58b-lndj5"} 798720 1633253811121
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-2-674945c58b-td4wj"} 733184 1633253810876
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-2bxmb"} 729088 1633253813338
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-5bd7k"} 585728 1633253810836
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-6nt7q"} 589824 1633253812098
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-hl2cj"} 729088 1633253816256
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-n4q5l"} 724992 1633253809484
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-3-7bf7fb4487-ws5vx"} 729088 1633253810601
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-4-5476d44584-7sh8g"} 585728 1633253809958
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-4-5476d44584-gkjfq"} 741376 1633253815105
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-4-5476d44584-l47mp"} 733184 1633253814108
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-4-5476d44584-nc4rz"} 716800 1633253809267
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-4-5476d44584-nvdb4"} 446464 1633253806701
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-4-5476d44584-vm68w"} 712704 1633253816933
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-dgljs"} 581632 1633253804056
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-fbtbt"} 462848 1633253812960
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-ftvv5"} 585728 1633253807769
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-kbmqg"} 720896 1633253809149
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-kzltf"} 593920 1633253800470
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-5-76fb4f7cdf-wjrxw"} 684032 1633253804706
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-6-776d465d58-26h78"} 724992 1633253813524
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-6-776d465d58-4m6rl"} 593920 1633253811712
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-6-776d465d58-4tqxg"} 708608 1633253817431
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-6-776d465d58-cwbbq"} 720896 1633253814088
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-6-776d465d58-kt78n"} 729088 1633253811046
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-6-776d465d58-lmpb2"} 712704 1633253802851
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-89sv8"} 602112 1633253805787
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-cqv9q"} 577536 1633253817809
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-jqxss"} 724992 1633253808832
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-mv8q5"} 724992 1633253809841
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-qxwbp"} 741376 1633253810863
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-7-7b9fc7c875-xk856"} 581632 1633253811863
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-2fzbb"} 720896 1633253803308
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-8vzvk"} 585728 1633253807065
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-hqfr6"} 729088 1633253810300
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-jwb7f"} 581632 1633253809076
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-l8c58"} 737280 1633253804298
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-8-5d8d46dcb4-q8cv5"} 729088 1633253803269
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-9-5744dbc77c-2wlvs"} 610304 1633253817837
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-9-5744dbc77c-5hd96"} 737280 1633253814168
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-9-5744dbc77c-b6bfc"} 724992 1633253812147
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-9-5744dbc77c-bqtbg"} 716800 1633253803171
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-9-5744dbc77c-lm28m"} 585728 1633253810755
pod_memory_working_set_bytes{namespace="sleeper-1",pod="sleeper-9-5744dbc77c-q6mg9"} 573440 1633253800817
# HELP scrape_error [ALPHA] 1 if there was an error while getting container metrics, 0 otherwise
# TYPE scrape_error gauge
scrape_error 0
`

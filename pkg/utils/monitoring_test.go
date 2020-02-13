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

package metrics_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"

	. "sigs.k8s.io/metrics-server/pkg/metrics"
)

func TestMetricsUtil(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Prometheus Metrics Util Test")
}

var _ = Describe("Prometheus Bucket Estimator", func() {
	Context("with a scrape timeout longer than the max default bucket", func() {
		It("should generate buckets in strictly increasing order", func() {
			buckets := BucketsForScrapeDuration(15 * time.Second)
			lastBucket := 0.0
			for _, bucket := range buckets {
				Expect(bucket).To(BeNumerically(">", lastBucket))
				lastBucket = bucket
			}
		})

		It("should include some buckets around the scrape timeout", func() {
			Expect(BucketsForScrapeDuration(15 * time.Second)).To(ContainElement(15.0))
			Expect(BucketsForScrapeDuration(15 * time.Second)).To(ContainElement(30.0))
		})
	})
	Context("with a scrape timeout shorter than the max default bucket", func() {
		It("should generate buckets in strictly increasing order", func() {
			buckets := BucketsForScrapeDuration(5 * time.Second)
			lastBucket := 0.0
			for _, bucket := range buckets {
				Expect(bucket).To(BeNumerically(">", lastBucket))
				lastBucket = bucket
			}
		})

		It("should include a bucket for the scrape timeout", func() {
			Expect(BucketsForScrapeDuration(5 * time.Second)).To(ContainElement(5.0))
		})
	})
	Context("with a scrape timeout equalt to the max default bucket", func() {
		maxBucket := prometheus.DefBuckets[len(prometheus.DefBuckets)-1]
		maxBucketDuration := time.Duration(maxBucket) * time.Second

		It("should generate buckets in strictly increasing order", func() {
			buckets := BucketsForScrapeDuration(maxBucketDuration)
			lastBucket := 0.0
			for _, bucket := range buckets {
				Expect(bucket).To(BeNumerically(">", lastBucket))
				lastBucket = bucket
			}
		})

		It("should include a bucket for the scrape timeout", func() {
			Expect(BucketsForScrapeDuration(maxBucketDuration)).To(ContainElement(maxBucket))
		})
	})
})

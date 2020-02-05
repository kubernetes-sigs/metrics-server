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

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// BucketsForScrapeDuration calculates a variant of the prometheus default histogram
// buckets that includes relevant buckets around our scrape timeout.
func BucketsForScrapeDuration(scrapeTimeout time.Duration) []float64 {
	// set up some buckets that include our scrape timeout,
	// so that we can easily pinpoint scrape timeout issues.
	// The default buckets provide a sane starting point for
	// the smaller numbers.
	buckets := append([]float64(nil), prometheus.DefBuckets...)
	maxBucket := buckets[len(buckets)-1]
	timeoutSeconds := float64(scrapeTimeout) / float64(time.Second)
	if timeoutSeconds > maxBucket {
		// [defaults, (scrapeTimeout + (scrapeTimeout - maxBucket)/ 2), scrapeTimeout, scrapeTimeout*1.5, scrapeTimeout*2]
		halfwayToScrapeTimeout := maxBucket + (timeoutSeconds-maxBucket)/2
		buckets = append(buckets, halfwayToScrapeTimeout, timeoutSeconds, timeoutSeconds*1.5, timeoutSeconds*2.0)
	} else if timeoutSeconds < maxBucket {
		var i int
		var bucket float64
		for i, bucket = range buckets {
			if bucket > timeoutSeconds {
				break
			}
		}

		if bucket-timeoutSeconds < buckets[0] || (i > 0 && timeoutSeconds-buckets[i-1] < buckets[0]) {
			// if we're sufficiently close to another bucket, just skip this
			return buckets
		}

		// likely that our scrape timeout is close to another bucket, so don't bother injecting more than just our scrape timeout
		oldRest := append([]float64(nil), buckets[i:]...) // make a copy so we don't overwrite it
		buckets = append(buckets[:i], timeoutSeconds)
		buckets = append(buckets, oldRest...)
	}

	return buckets
}

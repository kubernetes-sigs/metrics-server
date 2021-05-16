// Copyright 2021 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"time"
)

const (
	MiByte     = 1024 * 1024
	CoreSecond = 1000 * 1000 * 1000
)

func newMetricsPoint(st time.Time, ts time.Time, cpu, memory uint64) MetricsPoint {
	return MetricsPoint{
		StartTime:         st,
		Timestamp:         ts,
		CumulativeCpuUsed: cpu,
		MemoryUsage:       memory,
	}
}

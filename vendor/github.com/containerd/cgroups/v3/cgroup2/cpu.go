/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package cgroup2

import (
	"math"
	"strconv"
	"strings"
)

type CPUMax string

const (
	// Default kernel value for cpu quota period is 100000 us (100 ms), same for v1 and v2.
	// v1: https://www.kernel.org/doc/html/latest/scheduler/sched-bwc.html and
	// v2: https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html
	defaultCPUMax          = "max"
	defaultCPUMaxPeriod    = 100000
	defaultCPUMaxPeriodStr = "100000"
)

func NewCPUMax(quota *int64, period *uint64) CPUMax {
	max := defaultCPUMax
	if quota != nil {
		max = strconv.FormatInt(*quota, 10)
	}

	duration := defaultCPUMaxPeriodStr
	if period != nil {
		duration = strconv.FormatUint(*period, 10)
	}
	return CPUMax(strings.Join([]string{max, duration}, " "))
}

type CPU struct {
	Weight *uint64
	Max    CPUMax
	Cpus   string
	Mems   string
}

func (c CPUMax) extractQuotaAndPeriod() (int64, uint64, error) {
	var (
		quota  int64  = math.MaxInt64
		period uint64 = defaultCPUMaxPeriod
		err    error
	)

	// value: quota [period]
	values := strings.Split(string(c), " ")
	if len(values) < 1 || len(values) > 2 {
		return 0, 0, ErrInvalidFormat
	}

	if strings.ToLower(values[0]) != defaultCPUMax {
		quota, err = strconv.ParseInt(values[0], 10, 64)
		if err != nil {
			return 0, 0, err
		}
	}

	if len(values) == 2 {
		period, err = strconv.ParseUint(values[1], 10, 64)
		if err != nil {
			return 0, 0, err
		}
	}

	return quota, period, nil
}

func (r *CPU) Values() (o []Value) {
	if r.Weight != nil {
		o = append(o, Value{
			filename: "cpu.weight",
			value:    *r.Weight,
		})
	}
	if r.Max != "" {
		o = append(o, Value{
			filename: "cpu.max",
			value:    r.Max,
		})
	}
	if r.Cpus != "" {
		o = append(o, Value{
			filename: "cpuset.cpus",
			value:    r.Cpus,
		})
	}
	if r.Mems != "" {
		o = append(o, Value{
			filename: "cpuset.mems",
			value:    r.Mems,
		})
	}
	return o
}

// Copyright The OpenTelemetry Authors
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

package env // import "go.opentelemetry.io/otel/sdk/internal/env"

import (
	"os"
	"strconv"

	"go.opentelemetry.io/otel/internal/global"
)

// Environment variable names
const (
	// BatchSpanProcessorScheduleDelayKey
	// Delay interval between two consecutive exports.
	// i.e. 5000
	BatchSpanProcessorScheduleDelayKey = "OTEL_BSP_SCHEDULE_DELAY"
	// BatchSpanProcessorExportTimeoutKey
	// Maximum allowed time to export data.
	// i.e. 3000
	BatchSpanProcessorExportTimeoutKey = "OTEL_BSP_EXPORT_TIMEOUT"
	// BatchSpanProcessorMaxQueueSizeKey
	// Maximum queue size
	// i.e. 2048
	BatchSpanProcessorMaxQueueSizeKey = "OTEL_BSP_MAX_QUEUE_SIZE"
	// BatchSpanProcessorMaxExportBatchSizeKey
	// Maximum batch size
	// Note: Must be less than or equal to EnvBatchSpanProcessorMaxQueueSize
	// i.e. 512
	BatchSpanProcessorMaxExportBatchSizeKey = "OTEL_BSP_MAX_EXPORT_BATCH_SIZE"
)

// IntEnvOr returns the int value of the environment variable with name key if
// it exists and the value is an int. Otherwise, defaultValue is returned.
func IntEnvOr(key string, defaultValue int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		global.Info("Got invalid value, number value expected.", key, value)
		return defaultValue
	}

	return intValue
}

// BatchSpanProcessorScheduleDelay returns the environment variable value for
// the OTEL_BSP_SCHEDULE_DELAY key if it exists, otherwise defaultValue is
// returned.
func BatchSpanProcessorScheduleDelay(defaultValue int) int {
	return IntEnvOr(BatchSpanProcessorScheduleDelayKey, defaultValue)
}

// BatchSpanProcessorExportTimeout returns the environment variable value for
// the OTEL_BSP_EXPORT_TIMEOUT key if it exists, otherwise defaultValue is
// returned.
func BatchSpanProcessorExportTimeout(defaultValue int) int {
	return IntEnvOr(BatchSpanProcessorExportTimeoutKey, defaultValue)
}

// BatchSpanProcessorMaxQueueSize returns the environment variable value for
// the OTEL_BSP_MAX_QUEUE_SIZE key if it exists, otherwise defaultValue is
// returned.
func BatchSpanProcessorMaxQueueSize(defaultValue int) int {
	return IntEnvOr(BatchSpanProcessorMaxQueueSizeKey, defaultValue)
}

// BatchSpanProcessorMaxExportBatchSize returns the environment variable value for
// the OTEL_BSP_MAX_EXPORT_BATCH_SIZE key if it exists, otherwise defaultValue
// is returned.
func BatchSpanProcessorMaxExportBatchSize(defaultValue int) int {
	return IntEnvOr(BatchSpanProcessorMaxExportBatchSizeKey, defaultValue)
}

// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logging

// ConfigProjectPath returns the path for the project resource.
//
// Deprecated: Use
//   fmt.Sprintf("projects/%s", project)
// instead.
func ConfigProjectPath(project string) string {
	return "" +
		"projects/" +
		project +
		""
}

// ConfigSinkPath returns the path for the sink resource.
//
// Deprecated: Use
//   fmt.Sprintf("projects/%s/sinks/%s", project, sink)
// instead.
func ConfigSinkPath(project, sink string) string {
	return "" +
		"projects/" +
		project +
		"/sinks/" +
		sink +
		""
}

// ConfigExclusionPath returns the path for the exclusion resource.
//
// Deprecated: Use
//   fmt.Sprintf("projects/%s/exclusions/%s", project, exclusion)
// instead.
func ConfigExclusionPath(project, exclusion string) string {
	return "" +
		"projects/" +
		project +
		"/exclusions/" +
		exclusion +
		""
}

// ProjectPath returns the path for the project resource.
//
// Deprecated: Use
//   fmt.Sprintf("projects/%s", project)
// instead.
func ProjectPath(project string) string {
	return "" +
		"projects/" +
		project +
		""
}

// LogPath returns the path for the log resource.
//
// Deprecated: Use
//   fmt.Sprintf("projects/%s/logs/%s", project, log)
// instead.
func LogPath(project, log string) string {
	return "" +
		"projects/" +
		project +
		"/logs/" +
		log +
		""
}

// MetricsProjectPath returns the path for the project resource.
//
// Deprecated: Use
//   fmt.Sprintf("projects/%s", project)
// instead.
func MetricsProjectPath(project string) string {
	return "" +
		"projects/" +
		project +
		""
}

// MetricsMetricPath returns the path for the metric resource.
//
// Deprecated: Use
//   fmt.Sprintf("projects/%s/metrics/%s", project, metric)
// instead.
func MetricsMetricPath(project, metric string) string {
	return "" +
		"projects/" +
		project +
		"/metrics/" +
		metric +
		""
}

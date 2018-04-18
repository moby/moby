// Copyright 2017, OpenCensus Authors
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
//

/*
Package view contains support for collecting and exposing aggregates over stats.

In order to collect measurements, views need to be defined and registered.
A view allows recorded measurements to be filtered and aggregated over a time window.

All recorded measurements can be filtered by a list of tags.

OpenCensus provides several aggregation methods: count, distribution and sum.
Count aggregation only counts the number of measurement points. Distribution
aggregation provides statistical summary of the aggregated data. Sum distribution
sums up the measurement points. Aggregations are cumulative.

Users can dynamically create and delete views.

Libraries can export their own views and claim the view names
by registering them themselves.

Exporting

Collected and aggregated data can be exported to a metric collection
backend by registering its exporter.

Multiple exporters can be registered to upload the data to various
different backends. Users need to unregister the exporters once they
no longer are needed.
*/
package view // import "go.opencensus.io/stats/view"

// TODO(acetechnologist): Add a link to the language independent OpenCensus
// spec when it is available.

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

/*
package otelttrpc implements Opentelemetry instrumentation support for ttRPC.
The package implements unary client and server interceptors for opentelemetry
tracing instrumentation. The interceptors can be passed as ttrpc.ClientOpts
and ttrpc.ServerOpt to ttRPC during client and server creation. The interceptors
then automatically handle generating trace spans for all called and served
unary method calls. If the rest of the code is properly set up to collect and
export tracing data to opentelemetry, these spans should show up as part of the
collected traces.
*/
package otelttrpc

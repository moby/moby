// Copyright 2021 Google LLC. All Rights Reserved.
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

package ct

// We do the protoc generation here (rather than in the individual directories)
// in order to work around the newly-enforced rule that all protobuf file "names"
// must be unique.
// See https://developers.google.com/protocol-buffers/docs/proto#packages and
// https://github.com/golang/protobuf/issues/1122

//go:generate sh -c "protoc -I=. -I$(go list -f '{{ .Dir }}' github.com/google/trillian) -I$(go list -f '{{ .Dir }}' github.com/google/certificate-transparency-go) --go_out=paths=source_relative:. trillian/ctfe/configpb/config.proto"
//go:generate sh -c "protoc -I=. -I$(go list -f '{{ .Dir }}' github.com/google/trillian) -I$(go list -f '{{ .Dir }}' github.com/google/certificate-transparency-go) --go_out=paths=source_relative:. trillian/migrillian/configpb/config.proto"
//go:generate sh -c "protoc -I=. -I$(go list -f '{{ .Dir }}' github.com/google/certificate-transparency-go) --go_out=paths=source_relative:. client/configpb/multilog.proto"

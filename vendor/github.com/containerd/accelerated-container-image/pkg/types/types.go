/*
   Copyright The Accelerated Container Image Authors

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

package types

// OverlayBDBSConfig is the config of overlaybd target.
type OverlayBDBSConfig struct {
	RepoBlobURL       string                   `json:"repoBlobUrl"`
	Lowers            []OverlayBDBSConfigLower `json:"lowers"`
	Upper             OverlayBDBSConfigUpper   `json:"upper"`
	ResultFile        string                   `json:"resultFile"`
	AccelerationLayer bool                     `json:"accelerationLayer,omitempty"`
	RecordTracePath   string                   `json:"recordTracePath,omitempty"`
}

// OverlayBDBSConfigLower
type OverlayBDBSConfigLower struct {
	GzipIndex    string `json:"gzipIndex,omitempty"`
	File         string `json:"file,omitempty"`
	Digest       string `json:"digest,omitempty"`
	TargetFile   string `json:"targetFile,omitempty"`
	TargetDigest string `json:"targetDigest,omitempty"`
	Size         int64  `json:"size,omitempty"`
	Dir          string `json:"dir,omitempty"`
}

type OverlayBDBSConfigUpper struct {
	Index     string `json:"index,omitempty"`
	Data      string `json:"data,omitempty"`
	Target    string `json:"target,omitempty"`
	GzipIndex string `json:"gzipIndex,omitempty"`
}

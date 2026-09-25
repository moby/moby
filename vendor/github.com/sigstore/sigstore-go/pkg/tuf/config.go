// Copyright 2023 The Sigstore Authors.
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

package tuf

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Config struct {
	LastTimestamp time.Time `json:"lastTimestamp"`
}

func LoadConfig(p string) (*Config, error) {
	var c Config

	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	err = json.Unmarshal(b, &c)
	if err != nil {
		return nil, fmt.Errorf("malformed config file: %w", err)
	}

	return &c, nil
}

func (c *Config) Persist(p string) error {
	b, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to JSON marshal config: %w", err)
	}
	err = os.WriteFile(p, b, 0600)
	if err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

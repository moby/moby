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

package runc

import "context"

type extraEnvKey struct{}

// WithExtraEnv returns a context which adds the given "KEY=VALUE" entries to the
// environment of every runc invocation made with it. Entries are appended after
// the inherited environment, so they replace inherited variables of the same
// name, and repeated calls accumulate.
func WithExtraEnv(ctx context.Context, env ...string) context.Context {
	if len(env) == 0 {
		return ctx
	}
	// Copy: the existing array may be shared with sibling contexts.
	old := extraEnv(ctx)
	merged := make([]string, 0, len(old)+len(env))
	merged = append(merged, old...)
	merged = append(merged, env...)
	return context.WithValue(ctx, extraEnvKey{}, merged)
}

func extraEnv(ctx context.Context) []string {
	env, _ := ctx.Value(extraEnvKey{}).([]string)
	return env
}

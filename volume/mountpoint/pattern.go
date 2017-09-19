package mountpoint

import (
	"path"
	"strings"

	"github.com/docker/docker/api/types"
)

// PatternMatches determines if a pattern matches a mount point
// description. Patterns are conjunctions and a higher-level routine
// must implement disjunction.
func PatternMatches(pattern Pattern, mount *MountPoint) bool {
	for _, pattern := range pattern.EffectiveSource {
		if !StringPatternMatches(pattern, mount.EffectiveSource) {
			return false
		}
	}

	if pattern.EffectiveConsistency != nil && *pattern.EffectiveConsistency != mount.EffectiveConsistency {
		return false
	}

	if !volumePatternMatches(pattern.Volume, mount.Volume) {
		return false
	}

	if !containerPatternMatches(pattern.Container, mount.Container) {
		return false
	}

	if !imagePatternMatches(pattern.Image, mount.Image) {
		return false
	}

	for _, pattern := range pattern.Source {
		if !StringPatternMatches(pattern, mount.Source) {
			return false
		}
	}

	for _, pattern := range pattern.Destination {
		if !StringPatternMatches(pattern, mount.Destination) {
			return false
		}
	}

	if pattern.ReadOnly != nil && *pattern.ReadOnly != mount.ReadOnly {
		return false
	}

	if pattern.Type != nil && *pattern.Type != mount.Type {
		return false
	}

	for _, pattern := range pattern.Mode {
		if !StringPatternMatches(pattern, mount.Mode) {
			return false
		}
	}

	if pattern.Propagation != nil && *pattern.Propagation != mount.Propagation {
		return false
	}

	if pattern.CreateSourceIfMissing != nil && *pattern.CreateSourceIfMissing != mount.CreateSourceIfMissing {
		return false
	}

	if !appliedMiddlewareStackPatternMatches(pattern.AppliedMiddleware, mount.AppliedMiddleware) {
		return false
	}

	if pattern.Consistency != nil && *pattern.Consistency != mount.Consistency {
		return false
	}

	return true
}

func volumePatternMatches(pattern VolumePattern, volume Volume) bool {
	for _, pattern := range pattern.Name {
		if !StringPatternMatches(pattern, volume.Name) {
			return false
		}
	}

	for _, pattern := range pattern.Driver {
		if !StringPatternMatches(pattern, volume.Driver) {
			return false
		}
	}

	for _, pattern := range pattern.ID {
		if !StringPatternMatches(pattern, volume.ID) {
			return false
		}
	}

	for _, pattern := range pattern.Labels {
		if !stringMapPatternMatches(pattern, volume.Labels) {
			return false
		}
	}

	for _, pattern := range pattern.DriverOptions {
		if !stringMapPatternMatches(pattern, volume.DriverOptions) {
			return false
		}
	}

	if pattern.Scope != nil && *pattern.Scope != volume.Scope {
		return false
	}

	for _, pattern := range pattern.Options {
		if !stringMapPatternMatches(pattern, volume.Options) {
			return false
		}
	}

	return true
}

func containerPatternMatches(pattern ContainerPattern, container Container) bool {
	for _, pattern := range pattern.Labels {
		if !stringMapPatternMatches(pattern, container.Labels) {
			return false
		}
	}

	return true
}

func imagePatternMatches(pattern ImagePattern, image Image) bool {
	for _, pattern := range pattern.Labels {
		if !stringMapPatternMatches(pattern, image.Labels) {
			return false
		}
	}

	return true
}

func appliedMiddlewareStackPatternMatches(pattern AppliedMiddlewareStackPattern, appliedMiddleware []types.MountPointAppliedMiddleware) bool {

	if !middlewareExist(pattern.Exists, appliedMiddleware, false) {
		return false
	}
	if !middlewareExist(pattern.NotExists, appliedMiddleware, true) {
		return false
	}

	if !middlewareAll(pattern.All, appliedMiddleware, false) {
		return false
	}
	if !middlewareAll(pattern.NotAll, appliedMiddleware, true) {
		return false
	}

	if !middlewareAnySequence(pattern.AnySequence, appliedMiddleware, false) {
		return false
	}
	if !middlewareAnySequence(pattern.NotAnySequence, appliedMiddleware, true) {
		return false
	}

	if !middlewareTopSequence(pattern.TopSequence, appliedMiddleware, false) {
		return false
	}
	if !middlewareTopSequence(pattern.NotTopSequence, appliedMiddleware, true) {
		return false
	}

	if !middlewareBottomSequence(pattern.BottomSequence, appliedMiddleware, false) {
		return false
	}
	if !middlewareBottomSequence(pattern.NotBottomSequence, appliedMiddleware, true) {
		return false
	}

	if !middlewareRelativeOrder(pattern.RelativeOrder, appliedMiddleware, false) {
		return false
	}
	if !middlewareRelativeOrder(pattern.NotRelativeOrder, appliedMiddleware, true) {
		return false
	}

	return true
}

func middlewareExist(patterns []AppliedMiddlewarePattern, middleware []types.MountPointAppliedMiddleware, not bool) bool {
	for _, middlewarePattern := range patterns {
		matched := false
		for _, middleware := range middleware {
			if appliedMiddlewarePatternMatches(middlewarePattern, middleware) {
				matched = true
				break
			}
		}

		if matched == not {
			return false
		}
	}

	return true
}

func middlewareAll(patterns []AppliedMiddlewarePattern, middleware []types.MountPointAppliedMiddleware, not bool) bool {
	for _, middlewarePattern := range patterns {
		matched := true
		for _, middleware := range middleware {
			if !appliedMiddlewarePatternMatches(middlewarePattern, middleware) {
				matched = false
				break
			}
		}

		if matched == not {
			return false
		}
	}

	return true
}

func middlewareAnySequence(patterns []AppliedMiddlewarePattern, middleware []types.MountPointAppliedMiddleware, not bool) bool {
	anySequenceCount := len(patterns)
	appliedMiddlewareCount := len(middleware)
	if anySequenceCount > 0 {
		if anySequenceCount <= appliedMiddlewareCount {
			found := false
			for i := 0; i <= (appliedMiddlewareCount - anySequenceCount); i++ {
				matched := true
				for j, middlewarePattern := range patterns {
					if !appliedMiddlewarePatternMatches(middlewarePattern, middleware[i+j]) {
						matched = false
						break
					}
				}
				if matched {
					found = true
					break
				}
			}
			if found == not {
				return false
			}
		} else if !not {
			return false
		}
	}

	return true
}

func middlewareTopSequence(patterns []AppliedMiddlewarePattern, middleware []types.MountPointAppliedMiddleware, not bool) bool {
	topSequenceCount := len(patterns)
	appliedMiddlewareCount := len(middleware)
	if topSequenceCount > 0 {
		if topSequenceCount <= appliedMiddlewareCount {
			matched := true
			for i, middlewarePattern := range patterns {
				if !appliedMiddlewarePatternMatches(middlewarePattern, middleware[i]) {
					matched = false
					break
				}
			}
			if matched == not {
				return false
			}
		} else if !not {
			return false
		}
	}

	return true
}

func middlewareBottomSequence(patterns []AppliedMiddlewarePattern, middleware []types.MountPointAppliedMiddleware, not bool) bool {
	bottomSequenceCount := len(patterns)
	appliedMiddlewareCount := len(middleware)
	if bottomSequenceCount > 0 {
		if bottomSequenceCount <= appliedMiddlewareCount {
			matched := true
			start := appliedMiddlewareCount - bottomSequenceCount
			for i, middlewarePattern := range patterns {
				if !appliedMiddlewarePatternMatches(middlewarePattern, middleware[start+i]) {
					matched = false
					break
				}
			}
			if matched == not {
				return false
			}
		} else if !not {
			return false
		}
	}

	return true
}

func middlewareRelativeOrder(patterns []AppliedMiddlewarePattern, middleware []types.MountPointAppliedMiddleware, not bool) bool {
	relativeOrderCount := len(patterns)
	appliedMiddlewareCount := len(middleware)
	if relativeOrderCount > 0 {
		if relativeOrderCount <= appliedMiddlewareCount {
			remainingPatterns := patterns
			for _, middleware := range middleware {
				if len(remainingPatterns) == 0 {
					break
				}

				if appliedMiddlewarePatternMatches(remainingPatterns[0], middleware) {
					remainingPatterns = remainingPatterns[1:]
				}
			}
			if (len(remainingPatterns) == 0) == not {
				return false
			}
		} else if !not {
			return false
		}
	}

	return true
}

func appliedMiddlewarePatternMatches(pattern AppliedMiddlewarePattern, appliedMiddleware types.MountPointAppliedMiddleware) bool {
	for _, spattern := range pattern.Name {
		if !StringPatternMatches(spattern, appliedMiddleware.Name) {
			return false
		}
	}

	return changesPatternMatches(pattern.Changes, appliedMiddleware.Changes)
}

func changesPatternMatches(pattern ChangesPattern, changes types.MountPointChanges) bool {

	for _, pattern := range pattern.EffectiveSource {
		if !StringPatternMatches(pattern, changes.EffectiveSource) {
			return false
		}
	}

	if pattern.EffectiveConsistency != nil && *pattern.EffectiveConsistency != changes.EffectiveConsistency {
		return false
	}

	return true
}

func stringMapPatternMatches(pattern StringMapPattern, stringMap map[string]string) bool {

	// dsheets: These loops could almost certainly be fused but
	// reasoning about correctness would likely suffer. I don't think
	// patterns or maps will typically be big enough for the potential
	// constant (3x?) performance improvement to matter.

	for _, keyValuePattern := range pattern.Exists {
		matched := false
		for key, value := range stringMap {
			if StringPatternMatches(keyValuePattern.Key, key) {
				if StringPatternMatches(keyValuePattern.Value, value) {
					matched = true
					break
				}
			}
		}

		if matched == pattern.Not {
			return false
		}
	}

	for _, keyValuePattern := range pattern.All {
		matched := true
		for key, value := range stringMap {
			if StringPatternMatches(keyValuePattern.Key, key) {
				if !StringPatternMatches(keyValuePattern.Value, value) {
					matched = false
					break
				}
			} else if stringPatternIsEmpty(keyValuePattern.Value) {
				matched = false
				break
			}
		}

		if matched == pattern.Not {
			return false
		}
	}

	return true
}

// StringPatternMatches (pattern, string) is true if StringPattern
// pattern matches string
func StringPatternMatches(pattern StringPattern, string string) bool {
	if pattern.Empty && (len(string) == 0) == pattern.Not {
		return false
	}

	if pattern.Prefix != "" && strings.HasPrefix(string, pattern.Prefix) == pattern.Not {
		return false
	}

	if pattern.PathPrefix != "" {
		if !pathPrefix(pattern.PathPrefix, string, pattern.Not) {
			return false
		}
	}

	if pattern.Suffix != "" && strings.HasSuffix(string, pattern.Suffix) == pattern.Not {
		return false
	}

	if pattern.PathContains != "" {
		if !pathPrefix(string, pattern.PathContains, pattern.Not) {
			return false
		}
	}

	if pattern.Exactly != "" && (pattern.Exactly == string) == pattern.Not {
		return false
	}

	if pattern.Contains != "" && strings.Contains(string, pattern.Contains) == pattern.Not {
		return false
	}

	return true
}

func pathPrefix(prefix string, string string, not bool) bool {
	cleanPath := path.Clean(string)
	cleanPattern := path.Clean(prefix)
	patternLen := len(cleanPattern)

	matched := strings.HasPrefix(cleanPath, cleanPattern)
	if matched && cleanPattern[patternLen-1] != '/' {
		if len(cleanPath) > patternLen && cleanPath[patternLen] != '/' {
			matched = false
		}
	}

	if matched == not {
		return false
	}

	return true
}

func stringPatternIsEmpty(p StringPattern) bool {
	return !p.Empty && p.Prefix == "" && p.PathPrefix == "" && p.Suffix == "" && p.Exactly == "" && p.Contains == ""
}

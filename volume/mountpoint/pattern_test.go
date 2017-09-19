package mountpoint

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/stretchr/testify/require"
)

func testStringPatternInverse(pattern StringPattern, f func(pattern StringPattern, tru, fals bool)) {
	f(pattern, true, false)
	pattern.Not = true
	f(pattern, false, true)
}

func TestStringPattern(t *testing.T) {
	pattern := StringPattern{}
	require.Equal(t, true, StringPatternMatches(pattern, ""))
	require.Equal(t, true, StringPatternMatches(pattern, "asdf"))

	pattern = StringPattern{Not: true}
	require.Equal(t, true, StringPatternMatches(pattern, ""))
	require.Equal(t, true, StringPatternMatches(pattern, "asdf"))
}

func TestStringPatternEmpty(t *testing.T) {
	testStringPatternInverse(StringPattern{Empty: true},
		func(pattern StringPattern, tru, fals bool) {
			require.Equal(t, tru, StringPatternMatches(pattern, ""))
			require.Equal(t, fals, StringPatternMatches(pattern, "asdf"))
		})
}

func TestStringPatternPrefix(t *testing.T) {
	testStringPatternInverse(StringPattern{Prefix: "as"},
		func(pattern StringPattern, tru, fals bool) {
			require.Equal(t, fals, StringPatternMatches(pattern, ""))
			require.Equal(t, fals, StringPatternMatches(pattern, "adsf"))
			require.Equal(t, tru, StringPatternMatches(pattern, "asdf"))
		})
}

func TestStringPatternPathPrefix(t *testing.T) {
	testStringPatternInverse(StringPattern{PathPrefix: "///a/./b/c/../foo"},
		func(pattern StringPattern, tru, fals bool) {
			require.Equal(t, fals, StringPatternMatches(pattern, "/a/b/fo"))
			require.Equal(t, tru, StringPatternMatches(pattern, "/a/b/foo"))
			require.Equal(t, tru, StringPatternMatches(pattern, "/a/b/foo/"))
			require.Equal(t, fals, StringPatternMatches(pattern, "/a/b/foobar"))
			require.Equal(t, tru, StringPatternMatches(pattern, "/a/b/foo/bar"))
			require.Equal(t, tru, StringPatternMatches(pattern, "/a//b/c/d/../../foo/./bar"))
		})
}

func TestStringPatternSuffix(t *testing.T) {
	testStringPatternInverse(StringPattern{Suffix: ".dat"},
		func(pattern StringPattern, tru, fals bool) {
			require.Equal(t, fals, StringPatternMatches(pattern, ""))
			require.Equal(t, tru, StringPatternMatches(pattern, ".dat"))
			require.Equal(t, fals, StringPatternMatches(pattern, "foo.dab"))
			require.Equal(t, tru, StringPatternMatches(pattern, "/xyz/foo.dat"))
		})
}

func TestStringPatternPathContains(t *testing.T) {
	testStringPatternInverse(StringPattern{PathContains: "///a/./b/c/../foo"},
		func(pattern StringPattern, tru, fals bool) {
			require.Equal(t, fals, StringPatternMatches(pattern, "/a/b/fo"))
			require.Equal(t, tru, StringPatternMatches(pattern, "/a/b/foo"))
			require.Equal(t, tru, StringPatternMatches(pattern, "/a/b/foo/"))
			require.Equal(t, fals, StringPatternMatches(pattern, "/a/b/foobar"))
			require.Equal(t, fals, StringPatternMatches(pattern, "/a/b/foo/bar"))
			require.Equal(t, fals, StringPatternMatches(pattern, "/a//b/c/d/../../foo/./bar"))
			require.Equal(t, tru, StringPatternMatches(pattern, "/a//b/c/.."))
			require.Equal(t, tru, StringPatternMatches(pattern, "/"))
		})
}

func TestStringPatternExactly(t *testing.T) {
	testStringPatternInverse(StringPattern{Exactly: "xyz"},
		func(pattern StringPattern, tru, fals bool) {
			require.Equal(t, fals, StringPatternMatches(pattern, ""))
			require.Equal(t, fals, StringPatternMatches(pattern, "xy"))
			require.Equal(t, fals, StringPatternMatches(pattern, "xyyz"))
			require.Equal(t, fals, StringPatternMatches(pattern, "wxyz"))
			require.Equal(t, fals, StringPatternMatches(pattern, "xyz0"))
			require.Equal(t, tru, StringPatternMatches(pattern, "xyz"))
		})
}

func TestStringPatternContains(t *testing.T) {
	testStringPatternInverse(StringPattern{Contains: "xyz"},
		func(pattern StringPattern, tru, fals bool) {
			require.Equal(t, fals, StringPatternMatches(pattern, ""))
			require.Equal(t, fals, StringPatternMatches(pattern, "xy"))
			require.Equal(t, fals, StringPatternMatches(pattern, "xyyz"))
			require.Equal(t, tru, StringPatternMatches(pattern, "wxyz"))
			require.Equal(t, tru, StringPatternMatches(pattern, "xyz0"))
			require.Equal(t, tru, StringPatternMatches(pattern, "xyz"))
		})
}

func testStringMapPatternInverse(pattern StringMapPattern, f func(pattern StringMapPattern, tru, fals bool)) {
	f(pattern, true, false)
	pattern.Not = true
	f(pattern, false, true)
}

func TestStringMapPatternExists(t *testing.T) {
	testStringMapPatternInverse(StringMapPattern{
		Exists: []StringMapKeyValuePattern{
			{Key: StringPattern{Exactly: "key"}},
		},
	}, func(pattern StringMapPattern, tru, fals bool) {
		stringMap := map[string]string{}
		require.Equal(t, fals, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{"foo": ""}
		require.Equal(t, fals, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"foo": "",
			"key": "",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"foo": "",
			"key": "value",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
	})

	testStringMapPatternInverse(StringMapPattern{
		Exists: []StringMapKeyValuePattern{{
			Key:   StringPattern{Exactly: "key"},
			Value: StringPattern{Exactly: "value"},
		}},
	}, func(pattern StringMapPattern, tru, fals bool) {
		stringMap := map[string]string{
			"foo": "",
			"key": "",
		}
		require.Equal(t, fals, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"foo": "",
			"key": "value",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
	})

	testStringMapPatternInverse(StringMapPattern{
		Exists: []StringMapKeyValuePattern{
			{Key: StringPattern{Exactly: "key"}},
			{Key: StringPattern{Exactly: "k2"}},
		},
	}, func(pattern StringMapPattern, tru, fals bool) {
		stringMap := map[string]string{}
		require.Equal(t, fals, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key": "",
		}
		require.Equal(t, false, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"k2": "",
		}
		require.Equal(t, false, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"k2":  "",
			"key": "",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"k2":  "",
			"k3":  "",
			"key": "",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
	})

	testStringMapPatternInverse(StringMapPattern{
		Exists: []StringMapKeyValuePattern{
			{
				Key:   StringPattern{},
				Value: StringPattern{Prefix: "abc"},
			},
			{
				Key:   StringPattern{Exactly: "key"},
				Value: StringPattern{Suffix: "bcd"},
			},
		},
	}, func(pattern StringMapPattern, tru, fals bool) {
		stringMap := map[string]string{}
		require.Equal(t, fals, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key": "",
		}
		require.Equal(t, fals, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key": "bcd",
		}
		require.Equal(t, false, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key": "bcd",
			"k2":  "abc",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key": "abcd",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
	})
}

func TestStringMapPatternAll(t *testing.T) {
	testStringMapPatternInverse(StringMapPattern{
		All: []StringMapKeyValuePattern{
			{Key: StringPattern{Prefix: "pre"}},
		},
	}, func(pattern StringMapPattern, tru, fals bool) {
		stringMap := map[string]string{}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"foo": "",
		}
		require.Equal(t, fals, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"prefix": "",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"foo":    "",
			"prefix": "",
		}
		require.Equal(t, fals, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"precursor": "",
			"prefix":    "",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
	})

	testStringMapPatternInverse(StringMapPattern{
		All: []StringMapKeyValuePattern{
			{Key: StringPattern{Prefix: "pre"}},
			{Key: StringPattern{Suffix: "x"}},
		},
	}, func(pattern StringMapPattern, tru, fals bool) {
		stringMap := map[string]string{}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"quux": "",
		}
		require.Equal(t, false, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"prefix": "",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"quux":   "",
			"prefix": "",
		}
		require.Equal(t, false, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"prenex": "",
			"prefix": "",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
	})

	testStringMapPatternInverse(StringMapPattern{
		All: []StringMapKeyValuePattern{
			{
				Key:   StringPattern{Prefix: "key"},
				Value: StringPattern{Exactly: "value"},
			},
		},
	}, func(pattern StringMapPattern, tru, fals bool) {
		stringMap := map[string]string{}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key": "",
		}
		require.Equal(t, fals, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key": "value",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key0": "value",
			"key1": "",
		}
		require.Equal(t, fals, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key0": "value",
			"key1": "value",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key0": "value",
			"key1": "value",
			"k2":   "xyz",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
	})

	testStringMapPatternInverse(StringMapPattern{
		All: []StringMapKeyValuePattern{
			{
				Key:   StringPattern{Prefix: "key"},
				Value: StringPattern{Prefix: "v"},
			},
			{
				Key:   StringPattern{Suffix: "_"},
				Value: StringPattern{Suffix: "e"},
			},
		},
	}, func(pattern StringMapPattern, tru, fals bool) {
		stringMap := map[string]string{}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key_": "",
		}
		require.Equal(t, fals, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key_": "value",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key0_": "value",
			"key1":  "",
		}
		require.Equal(t, false, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key0_": "value",
			"key1_": "value",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
		stringMap = map[string]string{
			"key0_": "value",
			"key1":  "val",
			"d2_":   "abcde",
		}
		require.Equal(t, tru, stringMapPatternMatches(pattern, stringMap))
	})
}

func TestChangesPattern(t *testing.T) {
	pattern := ChangesPattern{}
	att := types.MountPointChanges{}
	require.Equal(t, true, changesPatternMatches(pattern, att))
	att = types.MountPointChanges{EffectiveSource: "/new_dir"}
	require.Equal(t, true, changesPatternMatches(pattern, att))
	att = types.MountPointChanges{EffectiveConsistency: "delegated"}
	require.Equal(t, true, changesPatternMatches(pattern, att))

	pattern = ChangesPattern{
		EffectiveSource: []StringPattern{{Exactly: "/new_dir"}},
	}
	att = types.MountPointChanges{}
	require.Equal(t, false, changesPatternMatches(pattern, att))
	att = types.MountPointChanges{EffectiveSource: "/new_dir"}
	require.Equal(t, true, changesPatternMatches(pattern, att))
	att = types.MountPointChanges{EffectiveConsistency: "delegated"}
	require.Equal(t, false, changesPatternMatches(pattern, att))

	delegated := mount.ConsistencyDelegated
	pattern = ChangesPattern{
		EffectiveConsistency: &delegated,
	}
	att = types.MountPointChanges{}
	require.Equal(t, false, changesPatternMatches(pattern, att))
	att = types.MountPointChanges{EffectiveSource: "/new_dir"}
	require.Equal(t, false, changesPatternMatches(pattern, att))
	att = types.MountPointChanges{EffectiveConsistency: mount.ConsistencyDelegated}
	require.Equal(t, true, changesPatternMatches(pattern, att))
}

func TestAppliedMiddlewarePattern(t *testing.T) {
	pattern := AppliedMiddlewarePattern{}
	middleware := types.MountPointAppliedMiddleware{}
	require.Equal(t, true, appliedMiddlewarePatternMatches(pattern, middleware))
	middleware = types.MountPointAppliedMiddleware{Name: "plugin:plugin"}
	require.Equal(t, true, appliedMiddlewarePatternMatches(pattern, middleware))
	middleware = types.MountPointAppliedMiddleware{
		Changes: types.MountPointChanges{EffectiveSource: "/new/dir"},
	}
	require.Equal(t, true, appliedMiddlewarePatternMatches(pattern, middleware))

	pattern = AppliedMiddlewarePattern{
		Name: []StringPattern{{Exactly: "plugin:plugin"}},
	}
	middleware = types.MountPointAppliedMiddleware{}
	require.Equal(t, false, appliedMiddlewarePatternMatches(pattern, middleware))
	middleware = types.MountPointAppliedMiddleware{Name: "plugin:plugin"}
	require.Equal(t, true, appliedMiddlewarePatternMatches(pattern, middleware))
	middleware = types.MountPointAppliedMiddleware{
		Changes: types.MountPointChanges{EffectiveSource: "/new/dir"},
	}
	require.Equal(t, false, appliedMiddlewarePatternMatches(pattern, middleware))

	pattern = AppliedMiddlewarePattern{
		Changes: ChangesPattern{
			EffectiveSource: []StringPattern{{PathPrefix: "/new"}},
		},
	}
	middleware = types.MountPointAppliedMiddleware{}
	require.Equal(t, false, appliedMiddlewarePatternMatches(pattern, middleware))
	middleware = types.MountPointAppliedMiddleware{Name: "plugin:plugin"}
	require.Equal(t, false, appliedMiddlewarePatternMatches(pattern, middleware))
	middleware = types.MountPointAppliedMiddleware{
		Changes: types.MountPointChanges{EffectiveSource: "/new/dir"},
	}
	require.Equal(t, true, appliedMiddlewarePatternMatches(pattern, middleware))
}

func testAppliedMiddlewareStackPatternInverse(pattern AppliedMiddlewareStackPattern, f func(pattern AppliedMiddlewareStackPattern, tru, fals bool)) {
	f(pattern, true, false)
	pattern.NotExists = pattern.Exists
	pattern.Exists = []AppliedMiddlewarePattern{}
	pattern.NotAll = pattern.All
	pattern.All = []AppliedMiddlewarePattern{}
	pattern.NotAnySequence = pattern.AnySequence
	pattern.AnySequence = []AppliedMiddlewarePattern{}
	pattern.NotTopSequence = pattern.TopSequence
	pattern.TopSequence = []AppliedMiddlewarePattern{}
	pattern.NotBottomSequence = pattern.BottomSequence
	pattern.BottomSequence = []AppliedMiddlewarePattern{}
	pattern.NotRelativeOrder = pattern.RelativeOrder
	pattern.RelativeOrder = []AppliedMiddlewarePattern{}
	f(pattern, false, true)
}

func TestAppliedMiddlewareStackPatternExists(t *testing.T) {
	testAppliedMiddlewareStackPatternInverse(AppliedMiddlewareStackPattern{
		Exists: []AppliedMiddlewarePattern{
			{Name: []StringPattern{{Exactly: "plugin:plugin0"}}},
		},
	}, func(pattern AppliedMiddlewareStackPattern, tru, fals bool) {
		list := []types.MountPointAppliedMiddleware{}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0"},
			{Name: "plugin:plugin1"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
	})

	testAppliedMiddlewareStackPatternInverse(AppliedMiddlewareStackPattern{
		Exists: []AppliedMiddlewarePattern{
			{Name: []StringPattern{{Exactly: "plugin:plugin0"}}},
			{Name: []StringPattern{{Exactly: "plugin:plugin1"}}},
		},
	}, func(pattern AppliedMiddlewareStackPattern, tru, fals bool) {
		list := []types.MountPointAppliedMiddleware{}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0"},
		}
		require.Equal(t, false, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
		}
		require.Equal(t, false, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0"},
			{Name: "plugin:plugin1"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
	})
}

func TestAppliedMiddlewareStackPatternAll(t *testing.T) {
	testAppliedMiddlewareStackPatternInverse(AppliedMiddlewareStackPattern{
		All: []AppliedMiddlewarePattern{
			{Name: []StringPattern{{Exactly: "plugin:plugin0"}}},
		},
	}, func(pattern AppliedMiddlewareStackPattern, tru, fals bool) {
		list := []types.MountPointAppliedMiddleware{}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0"},
			{Name: "plugin:plugin1"},
		}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
	})

	testAppliedMiddlewareStackPatternInverse(AppliedMiddlewareStackPattern{
		All: []AppliedMiddlewarePattern{
			{Name: []StringPattern{{Suffix: "_"}}},
			{Name: []StringPattern{{Prefix: "plugin:p"}}},
		},
	}, func(pattern AppliedMiddlewareStackPattern, tru, fals bool) {
		list := []types.MountPointAppliedMiddleware{}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0"},
			{Name: "plugin:plugin1"},
		}
		require.Equal(t, false, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0_"},
			{Name: "plugin:plugin1_"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0_"},
			{Name: "plugin:plugin1"},
		}
		require.Equal(t, false, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0_"},
			{Name: "plugin:_plugin1"},
		}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
	})
}

func TestAppliedMiddlewareStackPatternAnySequence(t *testing.T) {
	testAppliedMiddlewareStackPatternInverse(AppliedMiddlewareStackPattern{
		AnySequence: []AppliedMiddlewarePattern{
			{Name: []StringPattern{{Exactly: "plugin:plugin1"}}},
			{Name: []StringPattern{{Exactly: "plugin:plugin2"}}},
		},
	}, func(pattern AppliedMiddlewareStackPattern, tru, fals bool) {
		list := []types.MountPointAppliedMiddleware{}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
		}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0"},
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
			{Name: "plugin:plugin3"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0"},
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
			{Name: "plugin:plugin3"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin0"},
			{Name: "plugin:plugin2"},
		}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
	})
}

func TestAppliedMiddlewareStackPatternTopSequence(t *testing.T) {
	testAppliedMiddlewareStackPatternInverse(AppliedMiddlewareStackPattern{
		TopSequence: []AppliedMiddlewarePattern{
			{Name: []StringPattern{{Exactly: "plugin:plugin1"}}},
			{Name: []StringPattern{{Exactly: "plugin:plugin2"}}},
		},
	}, func(pattern AppliedMiddlewareStackPattern, tru, fals bool) {
		list := []types.MountPointAppliedMiddleware{}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
		}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0"},
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
		}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
			{Name: "plugin:plugin3"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
		}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
	})
}

func TestAppliedMiddlewareStackPatternBottomSequence(t *testing.T) {
	testAppliedMiddlewareStackPatternInverse(AppliedMiddlewareStackPattern{
		BottomSequence: []AppliedMiddlewarePattern{
			{Name: []StringPattern{{Exactly: "plugin:plugin1"}}},
			{Name: []StringPattern{{Exactly: "plugin:plugin2"}}},
		},
	}, func(pattern AppliedMiddlewareStackPattern, tru, fals bool) {
		list := []types.MountPointAppliedMiddleware{}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
		}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0"},
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
			{Name: "plugin:plugin3"},
		}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
	})
}

func TestAppliedMiddlewareStackPatternRelativeOrder(t *testing.T) {
	testAppliedMiddlewareStackPatternInverse(AppliedMiddlewareStackPattern{
		RelativeOrder: []AppliedMiddlewarePattern{
			{Name: []StringPattern{{Exactly: "plugin:plugin1"}}},
			{Name: []StringPattern{{Exactly: "plugin:plugin2"}}},
		},
	}, func(pattern AppliedMiddlewareStackPattern, tru, fals bool) {
		list := []types.MountPointAppliedMiddleware{}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
		}
		require.Equal(t, fals, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0"},
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin2"},
			{Name: "plugin:plugin3"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin0"},
			{Name: "plugin:plugin2"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin0"},
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin0"},
			{Name: "plugin:plugin2"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
		list = []types.MountPointAppliedMiddleware{
			{Name: "plugin:plugin1"},
			{Name: "plugin:plugin0"},
			{Name: "plugin:plugin2"},
			{Name: "plugin:plugin0"},
		}
		require.Equal(t, tru, appliedMiddlewareStackPatternMatches(pattern, list))
	})
}

func TestPattern(t *testing.T) {
	mountpoint := &MountPoint{
		EffectiveSource:       "/src",
		Source:                "/src",
		Destination:           "/mnt/pt",
		ReadOnly:              true,
		Type:                  TypeVolume,
		Mode:                  "o=bind,foo=bar",
		Propagation:           mount.PropagationShared,
		CreateSourceIfMissing: true,
		Volume: Volume{
			Name:   "MyVolume",
			Driver: "local",
			ID:     "0123456789abcdef",
			Labels: map[string]string{
				"label0": "value",
				"label1": "",
			},

			DriverOptions: map[string]string{
				"dopt0": "x",
				"dopt1": "y",
			},
			Scope: LocalScope,

			Options: map[string]string{
				"opt0": "x",
				"opt1": "y",
			},
		},
		Container: Container{
			Labels: map[string]string{
				"clabel0": "value",
				"clabel1": "",
			},
		},
		Image: Image{
			Labels: map[string]string{
				"ilabel0": "value",
				"ilabel1": "",
			},
		},

		AppliedMiddleware: []types.MountPointAppliedMiddleware{
			{Name: "plugin:mountPointPlugin0"},
			{Name: "plugin:mountPointPlugin1"},
		},

		EffectiveConsistency: mount.ConsistencyCached,
		Consistency:          mount.ConsistencyCached,
	}

	pattern := Pattern{}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		EffectiveSource: []StringPattern{{Exactly: "/src"}},
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		EffectiveSource: []StringPattern{{Not: true, Exactly: "/src"}},
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Source: []StringPattern{{Exactly: "/src"}},
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Source: []StringPattern{{Not: true, Exactly: "/src"}},
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Destination: []StringPattern{{PathPrefix: "/mnt"}},
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Destination: []StringPattern{{Not: true, PathPrefix: "/mnt"}},
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))
	tru := true
	pattern = Pattern{
		ReadOnly: &tru,
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	fals := false
	pattern = Pattern{
		ReadOnly: &fals,
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Volume: VolumePattern{
			Name: []StringPattern{{Exactly: "MyVolume"}},
		},
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Volume: VolumePattern{
			Name: []StringPattern{{Not: true, Exactly: "MyVolume"}},
		},
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Volume: VolumePattern{
			Driver: []StringPattern{{Exactly: "local"}},
		},
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Volume: VolumePattern{
			Driver: []StringPattern{{Not: true, Exactly: "local"}},
		},
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))
	volume := TypeVolume
	pattern = Pattern{
		Type: &volume,
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	bind := TypeBind
	pattern = Pattern{
		Type: &bind,
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Mode: []StringPattern{{Contains: "o=bind"}},
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Mode: []StringPattern{{Not: true, Contains: "o=bind"}},
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))
	propagationShared := mount.PropagationShared
	pattern = Pattern{
		Propagation: &propagationShared,
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	propagationSlave := mount.PropagationSlave
	pattern = Pattern{
		Propagation: &propagationSlave,
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Volume: VolumePattern{
			ID: []StringPattern{{Exactly: "0123456789abcdef"}},
		},
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Volume: VolumePattern{
			ID: []StringPattern{{Not: true, Exactly: "0123456789abcdef"}},
		},
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		CreateSourceIfMissing: &tru,
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		CreateSourceIfMissing: &fals,
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))

	pattern = Pattern{
		Container: ContainerPattern{
			Labels: []StringMapPattern{{
				Exists: []StringMapKeyValuePattern{
					{Key: StringPattern{Exactly: "clabel0"}},
				},
			}},
		},
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Container: ContainerPattern{
			Labels: []StringMapPattern{{
				Not: true,
				Exists: []StringMapKeyValuePattern{
					{Key: StringPattern{Exactly: "clabel0"}},
				},
			}},
		},
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))

	pattern = Pattern{
		Image: ImagePattern{
			Labels: []StringMapPattern{{
				Exists: []StringMapKeyValuePattern{
					{Key: StringPattern{Exactly: "ilabel0"}},
				},
			}},
		},
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Image: ImagePattern{
			Labels: []StringMapPattern{{
				Not: true,
				Exists: []StringMapKeyValuePattern{
					{Key: StringPattern{Exactly: "ilabel0"}},
				},
			}},
		},
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))

	pattern = Pattern{
		AppliedMiddleware: AppliedMiddlewareStackPattern{
			Exists: []AppliedMiddlewarePattern{
				{Name: []StringPattern{{Exactly: "plugin:mountPointPlugin0"}}},
			},
		},
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		AppliedMiddleware: AppliedMiddlewareStackPattern{
			NotExists: []AppliedMiddlewarePattern{{
				Name: []StringPattern{{Exactly: "plugin:mountPointPlugin0"}},
			}},
		},
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))

	cached := mount.ConsistencyCached
	pattern = Pattern{
		Consistency: &cached,
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	delegated := mount.ConsistencyDelegated
	pattern = Pattern{
		Consistency: &delegated,
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		EffectiveConsistency: &cached,
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		EffectiveConsistency: &delegated,
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))

	pattern = Pattern{
		Volume: VolumePattern{
			Labels: []StringMapPattern{{
				Exists: []StringMapKeyValuePattern{
					{Key: StringPattern{Exactly: "label0"}},
				},
			}},
		},
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Volume: VolumePattern{
			Labels: []StringMapPattern{{
				Not: true,
				Exists: []StringMapKeyValuePattern{
					{Key: StringPattern{Exactly: "label0"}},
				},
			}},
		},
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))

	pattern = Pattern{
		Volume: VolumePattern{
			DriverOptions: []StringMapPattern{{
				Exists: []StringMapKeyValuePattern{
					{Key: StringPattern{Exactly: "dopt0"}},
				},
			}},
		},
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Volume: VolumePattern{
			DriverOptions: []StringMapPattern{{
				Not: true,
				Exists: []StringMapKeyValuePattern{
					{Key: StringPattern{Exactly: "dopt0"}},
				},
			}},
		},
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))
	localScope := LocalScope
	pattern = Pattern{
		Volume: VolumePattern{
			Scope: &localScope,
		},
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	globalScope := GlobalScope
	pattern = Pattern{
		Volume: VolumePattern{
			Scope: &globalScope,
		},
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))

	pattern = Pattern{
		Volume: VolumePattern{
			Options: []StringMapPattern{{
				Exists: []StringMapKeyValuePattern{
					{Key: StringPattern{Exactly: "opt0"}},
				},
			}},
		},
	}
	require.Equal(t, true, PatternMatches(pattern, mountpoint))
	pattern = Pattern{
		Volume: VolumePattern{
			Options: []StringMapPattern{{
				Not: true,
				Exists: []StringMapKeyValuePattern{
					{Key: StringPattern{Exactly: "opt0"}},
				},
			}},
		},
	}
	require.Equal(t, false, PatternMatches(pattern, mountpoint))
}

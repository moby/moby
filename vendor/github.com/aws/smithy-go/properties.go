package smithy

// PropertiesReader provides an interface for reading metadata from the
// underlying metadata container.
type PropertiesReader interface {
	Get(key interface{}) interface{}
}

// Properties provides storing and reading metadata values. Keys may be any
// comparable value type. Get and Set will panic if a key is not comparable.
//
// The zero value for a Properties instance is ready for reads/writes without
// any additional initialization.
type Properties struct {
	values map[interface{}]interface{}
}

// Get attempts to retrieve the value the key points to. Returns nil if the
// key was not found.
//
// Panics if key type is not comparable.
func (m *Properties) Get(key interface{}) interface{} {
	m.lazyInit()
	return m.values[key]
}

// Set stores the value pointed to by the key. If a value already exists at
// that key it will be replaced with the new value.
//
// Panics if the key type is not comparable.
func (m *Properties) Set(key, value interface{}) {
	m.lazyInit()
	m.values[key] = value
}

// Has returns whether the key exists in the metadata.
//
// Panics if the key type is not comparable.
func (m *Properties) Has(key interface{}) bool {
	m.lazyInit()
	_, ok := m.values[key]
	return ok
}

// SetAll accepts all of the given Properties into the receiver, overwriting
// any existing keys in the case of conflicts.
func (m *Properties) SetAll(other *Properties) {
	if other.values == nil {
		return
	}

	m.lazyInit()
	for k, v := range other.values {
		m.values[k] = v
	}
}

func (m *Properties) lazyInit() {
	if m.values == nil {
		m.values = map[interface{}]interface{}{}
	}
}

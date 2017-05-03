// Package introspection provides introspection system.
//
// Format convention for supported types:
//   - struct:       directory
//   - int:          "%d\n"
//   - string:       "%s\n" for non-empty string, "" for empty string
//   - map[string]..: directory
//
// **RFC**: do we need "\n" at terminal?
// Note: For an empty string, an empty file (without "\n" at terminal) is created
package introspection

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/Sirupsen/logrus"
)

const (
	regularPerm    = 0644
	scopeSeparator = "."
)

// Update retrieves scope, path, and content from the field of reflect.Value,
// and passes them to Connector, if scope is in the allowed scopes.
//
// e.g. for `Container.Labels map[string]string`:
// scope: e.g. ".container.labels"
// path:  e.g. "\container\labels\org.example.foo.bar"
func Update(conn Connector, scopes []string, val reflect.Value) error {
	scope, path := scopeSeparator, string(os.PathSeparator)
	if err := callConnectorUpdateForDir(conn, scopes, scope, path); err != nil {
		return err
	}
	return update(conn, scopes, scope, path, val)
}

func update(conn Connector, scopes []string,
	scope, path string, val reflect.Value) error {
	switch val.Kind() {
	case reflect.Struct:
		return updateStruct(conn, scopes, scope, path, val)
	case reflect.Int:
		return updateInt(conn, scopes, scope, path, val)
	case reflect.String:
		return updateString(conn, scopes, scope, path, val)
	case reflect.Map:
		return updateMap(conn, scopes, scope, path, val)
	case reflect.Ptr:
		if val.IsNil() {
			return nil
		}
		return update(conn, scopes, scope, path, val.Elem())
	default:
		return fmt.Errorf("unsupported kind: %v", val.Kind())
	}
}

func joinScope(elem ...string) string {
	// FIXME
	osSep := string(os.PathSeparator)
	s := ""
	for _, e := range elem {
		s = filepath.Join(s, strings.Replace(e, scopeSeparator, osSep, -1))
	}
	return strings.Replace(s, osSep, scopeSeparator, -1)
}

func updateStruct(conn Connector, scopes []string,
	scope, path string, val reflect.Value) error {
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("expected reflect.Struct, got %v", val.Kind())
	}
	if err := callConnectorUpdateForDir(conn, scopes, scope, path); err != nil {
		return err
	}
	typ := val.Type()
	fields := val.NumField()
	for i := 0; i < fields; i++ {
		// **RFC** we call ToLower for the naming convention
		fieldName := strings.ToLower(typ.Field(i).Name)
		fieldScope := joinScope(scope, fieldName)
		fieldPath := filepath.Join(path, fieldName)
		fieldVal := val.Field(i)
		if err := update(conn, scopes, fieldScope, fieldPath, fieldVal); err != nil {
			return err
		}
	}
	return nil
}

func updateInt(conn Connector, scopes []string,
	scope, path string, val reflect.Value) error {
	if val.Kind() != reflect.Int {
		return fmt.Errorf("expected reflect.Int, got %v", val.Kind())
	}
	d := val.Interface().(int)
	return callConnectorUpdateForFile(conn, scopes, scope, path, []byte(fmt.Sprintf("%d\n", d)))
}

func updateString(conn Connector, scopes []string,
	scope, path string, val reflect.Value) error {
	if val.Kind() != reflect.String {
		return fmt.Errorf("expected reflect.String, got %v", val.Kind())
	}
	s := val.Interface().(string)
	if len(s) > 0 {
		s += "\n"
	}
	return callConnectorUpdateForFile(conn, scopes, scope, path, []byte(s))
}

func validateIntrospectionMapKeyString(s string) error {
	banned := "/\\:"
	if strings.ContainsAny(s, banned) {
		return fmt.Errorf("invalid map key string %s: should not contain %s)",
			s, banned)
	}
	return nil
}

func updateMap(conn Connector, scopes []string,
	scope, path string, val reflect.Value) error {
	if val.Kind() != reflect.Map {
		return fmt.Errorf("expected reflect.Map, got %v", val.Kind())
	}
	if err := callConnectorUpdateForDir(conn, scopes, scope, path); err != nil {
		return err
	}
	for _, mapK := range val.MapKeys() {
		if mapK.Kind() != reflect.String {
			return fmt.Errorf("expected reflect.String for map key, got %v", mapK.Kind())
		}
		key := mapK.Interface().(string)
		if err := validateIntrospectionMapKeyString(key); err != nil {
			// err occurs typically when key contains '/'
			logrus.Warn(err)
			continue
		}
		// e.g. mapScope=".container.labels", keyPath="\container\labels\org.example.foo.bar"
		mapScope := scope
		// we don't call strings.ToLower() and keep the original key string here
		keyPath := filepath.Join(path, key)
		mapV := val.MapIndex(mapK)
		if err := updateString(conn, scopes, mapScope, keyPath, mapV); err != nil {
			return err
		}
	}
	return nil
}

func callConnectorUpdateForFile(conn Connector, scopes []string, scope, path string, content []byte) error {
	if InScope(scope, scopes) {
		return conn.Update(scope, path, content, regularPerm)
	}
	return nil
}

func callConnectorUpdateForDir(conn Connector, scopes []string, scope, path string) error {
	if InScope(scope, scopes) {
		return conn.Update(scope, path, nil, regularPerm&os.ModeDir)
	}
	return nil
}

// Connector is an interface for the interaction between the daemon and the introspection system.
// Connector is supposed to a filesystem but not limited to so.
type Connector interface {
	// Update will not be called if scope is out of the scopes.
	// perm can be regular file or dir. content will be nil for dir.
	Update(scope string, path string, content []byte, perm os.FileMode) error
}

// InScope returns true if scope is in scopes
func InScope(scope string, scopes []string) bool {
	for _, s := range scopes {
		if strings.HasPrefix(scope, s) {
			return true
		}
	}
	return false
}

// Scopes returns possible scopes
func Scopes(ref reflect.Value) ([]string, error) {
	conn := &scopesCollector{}
	err := Update(conn, []string{scopeSeparator}, ref)
	return conn.scopes, err
}

type scopesCollector struct {
	scopes []string
}

func (conn *scopesCollector) Update(scope, path string, content []byte, perm os.FileMode) error {
	for _, s := range conn.scopes {
		if s == scope {
			return nil
		}
	}
	conn.scopes = append(conn.scopes, scope)
	return nil
}

// VerifyScopes returns non-nil error if scopes contains wrong scope name
func VerifyScopes(scopes []string, ref reflect.Value) error {
	if len(scopes) == 0 {
		return fmt.Errorf("empty scope set")
	}
	validScopes, err := Scopes(ref)
	if err != nil {
		return err
	}
	for _, s := range scopes {
		found := false
		for _, t := range validScopes {
			if s == t {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid scope: %s (valid scopes: %v)", s, validScopes)
		}
	}
	return nil
}

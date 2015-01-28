package types

import (
	"reflect"
	"testing"
)

func makeAnno(n, v string) Annotation {
	name, err := NewACName(n)
	if err != nil {
		panic(err)
	}
	return Annotation{
		Name:  *name,
		Value: v,
	}
}

func TestAnnotationsAssertValid(t *testing.T) {
	tests := []struct {
		in   []Annotation
		werr bool
	}{
		// duplicate names should fail
		{
			[]Annotation{
				makeAnno("foo", "bar"),
				makeAnno("foo", "baz"),
			},
			true,
		},
		// bad created should fail
		{
			[]Annotation{
				makeAnno("created", "garbage"),
			},
			true,
		},
		// bad homepage should fail
		{
			[]Annotation{
				makeAnno("homepage", "not-A$@#URL"),
			},
			true,
		},
		// bad documentation should fail
		{
			[]Annotation{
				makeAnno("documentation", "ftp://isnotallowed.com"),
			},
			true,
		},
		// good cases
		{
			[]Annotation{
				makeAnno("created", "2004-05-14T23:11:14+00:00"),
				makeAnno("documentation", "http://example.com/docs"),
			},
			false,
		},
		{
			[]Annotation{
				makeAnno("foo", "bar"),
				makeAnno("homepage", "https://homepage.com"),
			},
			false,
		},
		// empty is OK
		{
			[]Annotation{},
			false,
		},
	}
	for i, tt := range tests {
		a := Annotations(tt.in)
		err := a.assertValid()
		if gerr := (err != nil); gerr != tt.werr {
			t.Errorf("#%d: gerr=%t, want %t (err=%v)", i, gerr, tt.werr, err)
		}
	}
}

func TestAnnotationsSet(t *testing.T) {
	a := Annotations{}

	a.Set("foo", "bar")
	w := Annotations{
		Annotation{ACName("foo"), "bar"},
	}
	if !reflect.DeepEqual(w, a) {
		t.Fatalf("want %v, got %v", w, a)
	}

	a.Set("dog", "woof")
	w = Annotations{
		Annotation{ACName("foo"), "bar"},
		Annotation{ACName("dog"), "woof"},
	}
	if !reflect.DeepEqual(w, a) {
		t.Fatalf("want %v, got %v", w, a)
	}

	a.Set("foo", "baz")
	w = Annotations{
		Annotation{ACName("foo"), "baz"},
		Annotation{ACName("dog"), "woof"},
	}
	if !reflect.DeepEqual(w, a) {
		t.Fatalf("want %v, got %v", w, a)
	}
}

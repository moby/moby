package jsonlog

import (
	"time"

	"github.com/go-check/check"
)

// Testing to ensure 'year' fields is between 0 and 9999
func (s *DockerSuite) TestFastTimeMarshalJSONWithInvalidDate(c *check.C) {
	aTime := time.Date(-1, 1, 1, 0, 0, 0, 0, time.Local)
	json, err := FastTimeMarshalJSON(aTime)
	if err == nil {
		c.Fatalf("FastTimeMarshalJSON should throw an error, but was '%v'", json)
	}
	anotherTime := time.Date(10000, 1, 1, 0, 0, 0, 0, time.Local)
	json, err = FastTimeMarshalJSON(anotherTime)
	if err == nil {
		c.Fatalf("FastTimeMarshalJSON should throw an error, but was '%v'", json)
	}

}

func (s *DockerSuite) TestFastTimeMarshalJSON(c *check.C) {
	aTime := time.Date(2015, 5, 29, 11, 1, 2, 3, time.UTC)
	json, err := FastTimeMarshalJSON(aTime)
	if err != nil {
		c.Fatal(err)
	}
	expected := "\"2015-05-29T11:01:02.000000003Z\""
	if json != expected {
		c.Fatalf("Expected %v, got %v", expected, json)
	}

	location, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		c.Fatal(err)
	}
	aTime = time.Date(2015, 5, 29, 11, 1, 2, 3, location)
	json, err = FastTimeMarshalJSON(aTime)
	if err != nil {
		c.Fatal(err)
	}
	expected = "\"2015-05-29T11:01:02.000000003+02:00\""
	if json != expected {
		c.Fatalf("Expected %v, got %v", expected, json)
	}
}

package v2

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gorilla/mux"
)

type routeTestCase struct {
	RequestURI string
	Vars       map[string]string
	RouteName  string
	StatusCode int
}

// TestRouter registers a test handler with all the routes and ensures that
// each route returns the expected path variables. Not method verification is
// present. This not meant to be exhaustive but as check to ensure that the
// expected variables are extracted.
//
// This may go away as the application structure comes together.
func TestRouter(t *testing.T) {

	router := Router()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testCase := routeTestCase{
			RequestURI: r.RequestURI,
			Vars:       mux.Vars(r),
			RouteName:  mux.CurrentRoute(r).GetName(),
		}

		enc := json.NewEncoder(w)

		if err := enc.Encode(testCase); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// Startup test server
	server := httptest.NewServer(router)

	for _, testcase := range []routeTestCase{
		{
			RouteName:  RouteNameBase,
			RequestURI: "/v2/",
			Vars:       map[string]string{},
		},
		{
			RouteName:  RouteNameManifest,
			RequestURI: "/v2/foo/manifests/bar",
			Vars: map[string]string{
				"name":      "foo",
				"reference": "bar",
			},
		},
		{
			RouteName:  RouteNameManifest,
			RequestURI: "/v2/foo/bar/manifests/tag",
			Vars: map[string]string{
				"name":      "foo/bar",
				"reference": "tag",
			},
		},
		{
			RouteName:  RouteNameTags,
			RequestURI: "/v2/foo/bar/tags/list",
			Vars: map[string]string{
				"name": "foo/bar",
			},
		},
		{
			RouteName:  RouteNameBlob,
			RequestURI: "/v2/foo/bar/blobs/tarsum.dev+foo:abcdef0919234",
			Vars: map[string]string{
				"name":   "foo/bar",
				"digest": "tarsum.dev+foo:abcdef0919234",
			},
		},
		{
			RouteName:  RouteNameBlob,
			RequestURI: "/v2/foo/bar/blobs/sha256:abcdef0919234",
			Vars: map[string]string{
				"name":   "foo/bar",
				"digest": "sha256:abcdef0919234",
			},
		},
		{
			RouteName:  RouteNameBlobUpload,
			RequestURI: "/v2/foo/bar/blobs/uploads/",
			Vars: map[string]string{
				"name": "foo/bar",
			},
		},
		{
			RouteName:  RouteNameBlobUploadChunk,
			RequestURI: "/v2/foo/bar/blobs/uploads/uuid",
			Vars: map[string]string{
				"name": "foo/bar",
				"uuid": "uuid",
			},
		},
		{
			RouteName:  RouteNameBlobUploadChunk,
			RequestURI: "/v2/foo/bar/blobs/uploads/D95306FA-FAD3-4E36-8D41-CF1C93EF8286",
			Vars: map[string]string{
				"name": "foo/bar",
				"uuid": "D95306FA-FAD3-4E36-8D41-CF1C93EF8286",
			},
		},
		{
			RouteName:  RouteNameBlobUploadChunk,
			RequestURI: "/v2/foo/bar/blobs/uploads/RDk1MzA2RkEtRkFEMy00RTM2LThENDEtQ0YxQzkzRUY4Mjg2IA==",
			Vars: map[string]string{
				"name": "foo/bar",
				"uuid": "RDk1MzA2RkEtRkFEMy00RTM2LThENDEtQ0YxQzkzRUY4Mjg2IA==",
			},
		},
		{
			// Check ambiguity: ensure we can distinguish between tags for
			// "foo/bar/image/image" and image for "foo/bar/image" with tag
			// "tags"
			RouteName:  RouteNameManifest,
			RequestURI: "/v2/foo/bar/manifests/manifests/tags",
			Vars: map[string]string{
				"name":      "foo/bar/manifests",
				"reference": "tags",
			},
		},
		{
			// This case presents an ambiguity between foo/bar with tag="tags"
			// and list tags for "foo/bar/manifest"
			RouteName:  RouteNameTags,
			RequestURI: "/v2/foo/bar/manifests/tags/list",
			Vars: map[string]string{
				"name": "foo/bar/manifests",
			},
		},
		{
			RouteName:  RouteNameBlobUploadChunk,
			RequestURI: "/v2/foo/../../blob/uploads/D95306FA-FAD3-4E36-8D41-CF1C93EF8286",
			StatusCode: http.StatusNotFound,
		},
	} {
		// Register the endpoint
		router.GetRoute(testcase.RouteName).Handler(testHandler)
		u := server.URL + testcase.RequestURI

		resp, err := http.Get(u)

		if err != nil {
			t.Fatalf("error issuing get request: %v", err)
		}

		if testcase.StatusCode == 0 {
			// Override default, zero-value
			testcase.StatusCode = http.StatusOK
		}

		if resp.StatusCode != testcase.StatusCode {
			t.Fatalf("unexpected status for %s: %v %v", u, resp.Status, resp.StatusCode)
		}

		if testcase.StatusCode != http.StatusOK {
			// We don't care about json response.
			continue
		}

		dec := json.NewDecoder(resp.Body)

		var actualRouteInfo routeTestCase
		if err := dec.Decode(&actualRouteInfo); err != nil {
			t.Fatalf("error reading json response: %v", err)
		}
		// Needs to be set out of band
		actualRouteInfo.StatusCode = resp.StatusCode

		if actualRouteInfo.RouteName != testcase.RouteName {
			t.Fatalf("incorrect route %q matched, expected %q", actualRouteInfo.RouteName, testcase.RouteName)
		}

		if !reflect.DeepEqual(actualRouteInfo, testcase) {
			t.Fatalf("actual does not equal expected: %#v != %#v", actualRouteInfo, testcase)
		}
	}

}

package formatter

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"bytes"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/stretchr/testify/assert"
)

type historyCase struct {
	historyCtx historyContext
	expValue   string
	call       func() string
}

func TestHistoryContext_ID(t *testing.T) {
	id := stringid.GenerateRandomID()

	var ctx historyContext
	cases := []historyCase{
		{
			historyContext{
				h:     image.HistoryResponseItem{ID: id},
				trunc: false,
			}, id, ctx.ID,
		},
		{
			historyContext{
				h:     image.HistoryResponseItem{ID: id},
				trunc: true,
			}, stringid.TruncateID(id), ctx.ID,
		},
	}

	for _, c := range cases {
		ctx = c.historyCtx
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
		}
	}
}

func TestHistoryContext_CreatedSince(t *testing.T) {
	unixTime := time.Now().AddDate(0, 0, -7).Unix()
	expected := "7 days ago"

	var ctx historyContext
	cases := []historyCase{
		{
			historyContext{
				h:     image.HistoryResponseItem{Created: unixTime},
				trunc: false,
				human: true,
			}, expected, ctx.CreatedSince,
		},
	}

	for _, c := range cases {
		ctx = c.historyCtx
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
		}
	}
}

func TestHistoryContext_CreatedBy(t *testing.T) {
	withTabs := `/bin/sh -c apt-key adv --keyserver hkp://pgp.mit.edu:80	--recv-keys 573BFD6B3D8FBC641079A6ABABF5BD827BD9BF62	&& echo "deb http://nginx.org/packages/mainline/debian/ jessie nginx" >> /etc/apt/sources.list  && apt-get update  && apt-get install --no-install-recommends --no-install-suggests -y       ca-certificates       nginx=${NGINX_VERSION}       nginx-module-xslt       nginx-module-geoip       nginx-module-image-filter       nginx-module-perl       nginx-module-njs       gettext-base  && rm -rf /var/lib/apt/lists/*`
	expected := `/bin/sh -c apt-key adv --keyserver hkp://pgp.mit.edu:80 --recv-keys 573BFD6B3D8FBC641079A6ABABF5BD827BD9BF62 && echo "deb http://nginx.org/packages/mainline/debian/ jessie nginx" >> /etc/apt/sources.list  && apt-get update  && apt-get install --no-install-recommends --no-install-suggests -y       ca-certificates       nginx=${NGINX_VERSION}       nginx-module-xslt       nginx-module-geoip       nginx-module-image-filter       nginx-module-perl       nginx-module-njs       gettext-base  && rm -rf /var/lib/apt/lists/*`

	var ctx historyContext
	cases := []historyCase{
		{
			historyContext{
				h:     image.HistoryResponseItem{CreatedBy: withTabs},
				trunc: false,
			}, expected, ctx.CreatedBy,
		},
		{
			historyContext{
				h:     image.HistoryResponseItem{CreatedBy: withTabs},
				trunc: true,
			}, stringutils.Ellipsis(expected, 45), ctx.CreatedBy,
		},
	}

	for _, c := range cases {
		ctx = c.historyCtx
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
		}
	}
}

func TestHistoryContext_Size(t *testing.T) {
	size := int64(182964289)
	expected := "183MB"

	var ctx historyContext
	cases := []historyCase{
		{
			historyContext{
				h:     image.HistoryResponseItem{Size: size},
				trunc: false,
				human: true,
			}, expected, ctx.Size,
		}, {
			historyContext{
				h:     image.HistoryResponseItem{Size: size},
				trunc: false,
				human: false,
			}, strconv.Itoa(182964289), ctx.Size,
		},
	}

	for _, c := range cases {
		ctx = c.historyCtx
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
		}
	}
}

func TestHistoryContext_Comment(t *testing.T) {
	comment := "Some comment"

	var ctx historyContext
	cases := []historyCase{
		{
			historyContext{
				h:     image.HistoryResponseItem{Comment: comment},
				trunc: false,
			}, comment, ctx.Comment,
		},
	}

	for _, c := range cases {
		ctx = c.historyCtx
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
		}
	}
}

func TestHistoryContext_Table(t *testing.T) {
	out := bytes.NewBufferString("")
	unixTime := time.Now().AddDate(0, 0, -1).Unix()
	histories := []image.HistoryResponseItem{
		{ID: "imageID1", Created: unixTime, CreatedBy: "/bin/bash ls && npm i && npm run test && karma -c karma.conf.js start && npm start && more commands here && the list goes on", Size: int64(182964289), Comment: "Hi", Tags: []string{"image:tag2"}},
		{ID: "imageID2", Created: unixTime, CreatedBy: "/bin/bash echo", Size: int64(182964289), Comment: "Hi", Tags: []string{"image:tag2"}},
		{ID: "imageID3", Created: unixTime, CreatedBy: "/bin/bash ls", Size: int64(182964289), Comment: "Hi", Tags: []string{"image:tag2"}},
		{ID: "imageID4", Created: unixTime, CreatedBy: "/bin/bash grep", Size: int64(182964289), Comment: "Hi", Tags: []string{"image:tag2"}},
	}
	expectedNoTrunc := `IMAGE               CREATED             CREATED BY                                                                                                                     SIZE                COMMENT
imageID1            24 hours ago        /bin/bash ls && npm i && npm run test && karma -c karma.conf.js start && npm start && more commands here && the list goes on   183MB               Hi
imageID2            24 hours ago        /bin/bash echo                                                                                                                 183MB               Hi
imageID3            24 hours ago        /bin/bash ls                                                                                                                   183MB               Hi
imageID4            24 hours ago        /bin/bash grep                                                                                                                 183MB               Hi
`
	expectedTrunc := `IMAGE               CREATED             CREATED BY                                      SIZE                COMMENT
imageID1            24 hours ago        /bin/bash ls && npm i && npm run test && k...   183MB               Hi
imageID2            24 hours ago        /bin/bash echo                                  183MB               Hi
imageID3            24 hours ago        /bin/bash ls                                    183MB               Hi
imageID4            24 hours ago        /bin/bash grep                                  183MB               Hi
`

	contexts := []struct {
		context  Context
		expected string
	}{
		{Context{
			Format: NewHistoryFormat("table", false, true),
			Trunc:  true,
			Output: out,
		},
			expectedTrunc,
		},
		{Context{
			Format: NewHistoryFormat("table", false, true),
			Trunc:  false,
			Output: out,
		},
			expectedNoTrunc,
		},
	}

	for _, context := range contexts {
		HistoryWrite(context.context, true, histories)
		assert.Equal(t, context.expected, out.String())
		// Clean buffer
		out.Reset()
	}
}

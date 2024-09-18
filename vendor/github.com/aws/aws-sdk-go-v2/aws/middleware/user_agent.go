package middleware

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

var languageVersion = strings.TrimPrefix(runtime.Version(), "go")

// SDKAgentKeyType is the metadata type to add to the SDK agent string
type SDKAgentKeyType int

// The set of valid SDKAgentKeyType constants. If an unknown value is assigned for SDKAgentKeyType it will
// be mapped to AdditionalMetadata.
const (
	_ SDKAgentKeyType = iota
	APIMetadata
	OperatingSystemMetadata
	LanguageMetadata
	EnvironmentMetadata
	FeatureMetadata
	ConfigMetadata
	FrameworkMetadata
	AdditionalMetadata
	ApplicationIdentifier
)

func (k SDKAgentKeyType) string() string {
	switch k {
	case APIMetadata:
		return "api"
	case OperatingSystemMetadata:
		return "os"
	case LanguageMetadata:
		return "lang"
	case EnvironmentMetadata:
		return "exec-env"
	case FeatureMetadata:
		return "ft"
	case ConfigMetadata:
		return "cfg"
	case FrameworkMetadata:
		return "lib"
	case ApplicationIdentifier:
		return "app"
	case AdditionalMetadata:
		fallthrough
	default:
		return "md"
	}
}

const execEnvVar = `AWS_EXECUTION_ENV`

var validChars = map[rune]bool{
	'!': true, '#': true, '$': true, '%': true, '&': true, '\'': true, '*': true, '+': true,
	'-': true, '.': true, '^': true, '_': true, '`': true, '|': true, '~': true,
}

// requestUserAgent is a build middleware that set the User-Agent for the request.
type requestUserAgent struct {
	sdkAgent, userAgent *smithyhttp.UserAgentBuilder
}

// newRequestUserAgent returns a new requestUserAgent which will set the User-Agent and X-Amz-User-Agent for the
// request.
//
// User-Agent example:
//
//	aws-sdk-go-v2/1.2.3
//
// X-Amz-User-Agent example:
//
//	aws-sdk-go-v2/1.2.3 md/GOOS/linux md/GOARCH/amd64 lang/go/1.15
func newRequestUserAgent() *requestUserAgent {
	userAgent, sdkAgent := smithyhttp.NewUserAgentBuilder(), smithyhttp.NewUserAgentBuilder()
	addProductName(userAgent)
	addProductName(sdkAgent)

	r := &requestUserAgent{
		sdkAgent:  sdkAgent,
		userAgent: userAgent,
	}

	addSDKMetadata(r)

	return r
}

func addSDKMetadata(r *requestUserAgent) {
	r.AddSDKAgentKey(OperatingSystemMetadata, getNormalizedOSName())
	r.AddSDKAgentKeyValue(LanguageMetadata, "go", languageVersion)
	r.AddSDKAgentKeyValue(AdditionalMetadata, "GOOS", runtime.GOOS)
	r.AddSDKAgentKeyValue(AdditionalMetadata, "GOARCH", runtime.GOARCH)
	if ev := os.Getenv(execEnvVar); len(ev) > 0 {
		r.AddSDKAgentKey(EnvironmentMetadata, ev)
	}
}

func addProductName(builder *smithyhttp.UserAgentBuilder) {
	builder.AddKeyValue(aws.SDKName, aws.SDKVersion)
}

// AddUserAgentKey retrieves a requestUserAgent from the provided stack, or initializes one.
func AddUserAgentKey(key string) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		requestUserAgent, err := getOrAddRequestUserAgent(stack)
		if err != nil {
			return err
		}
		requestUserAgent.AddUserAgentKey(key)
		return nil
	}
}

// AddUserAgentKeyValue retrieves a requestUserAgent from the provided stack, or initializes one.
func AddUserAgentKeyValue(key, value string) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		requestUserAgent, err := getOrAddRequestUserAgent(stack)
		if err != nil {
			return err
		}
		requestUserAgent.AddUserAgentKeyValue(key, value)
		return nil
	}
}

// AddSDKAgentKey retrieves a requestUserAgent from the provided stack, or initializes one.
func AddSDKAgentKey(keyType SDKAgentKeyType, key string) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		requestUserAgent, err := getOrAddRequestUserAgent(stack)
		if err != nil {
			return err
		}
		requestUserAgent.AddSDKAgentKey(keyType, key)
		return nil
	}
}

// AddSDKAgentKeyValue retrieves a requestUserAgent from the provided stack, or initializes one.
func AddSDKAgentKeyValue(keyType SDKAgentKeyType, key, value string) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		requestUserAgent, err := getOrAddRequestUserAgent(stack)
		if err != nil {
			return err
		}
		requestUserAgent.AddSDKAgentKeyValue(keyType, key, value)
		return nil
	}
}

// AddRequestUserAgentMiddleware registers a requestUserAgent middleware on the stack if not present.
func AddRequestUserAgentMiddleware(stack *middleware.Stack) error {
	_, err := getOrAddRequestUserAgent(stack)
	return err
}

func getOrAddRequestUserAgent(stack *middleware.Stack) (*requestUserAgent, error) {
	id := (*requestUserAgent)(nil).ID()
	bm, ok := stack.Build.Get(id)
	if !ok {
		bm = newRequestUserAgent()
		err := stack.Build.Add(bm, middleware.After)
		if err != nil {
			return nil, err
		}
	}

	requestUserAgent, ok := bm.(*requestUserAgent)
	if !ok {
		return nil, fmt.Errorf("%T for %s middleware did not match expected type", bm, id)
	}

	return requestUserAgent, nil
}

// AddUserAgentKey adds the component identified by name to the User-Agent string.
func (u *requestUserAgent) AddUserAgentKey(key string) {
	u.userAgent.AddKey(strings.Map(rules, key))
}

// AddUserAgentKeyValue adds the key identified by the given name and value to the User-Agent string.
func (u *requestUserAgent) AddUserAgentKeyValue(key, value string) {
	u.userAgent.AddKeyValue(strings.Map(rules, key), strings.Map(rules, value))
}

// AddUserAgentKey adds the component identified by name to the User-Agent string.
func (u *requestUserAgent) AddSDKAgentKey(keyType SDKAgentKeyType, key string) {
	// TODO: should target sdkAgent
	u.userAgent.AddKey(keyType.string() + "/" + strings.Map(rules, key))
}

// AddUserAgentKeyValue adds the key identified by the given name and value to the User-Agent string.
func (u *requestUserAgent) AddSDKAgentKeyValue(keyType SDKAgentKeyType, key, value string) {
	// TODO: should target sdkAgent
	u.userAgent.AddKeyValue(keyType.string(), strings.Map(rules, key)+"#"+strings.Map(rules, value))
}

// ID the name of the middleware.
func (u *requestUserAgent) ID() string {
	return "UserAgent"
}

// HandleBuild adds or appends the constructed user agent to the request.
func (u *requestUserAgent) HandleBuild(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (
	out middleware.BuildOutput, metadata middleware.Metadata, err error,
) {
	switch req := in.Request.(type) {
	case *smithyhttp.Request:
		u.addHTTPUserAgent(req)
		// TODO: To be re-enabled
		// u.addHTTPSDKAgent(req)
	default:
		return out, metadata, fmt.Errorf("unknown transport type %T", in)
	}

	return next.HandleBuild(ctx, in)
}

func (u *requestUserAgent) addHTTPUserAgent(request *smithyhttp.Request) {
	const userAgent = "User-Agent"
	updateHTTPHeader(request, userAgent, u.userAgent.Build())
}

func (u *requestUserAgent) addHTTPSDKAgent(request *smithyhttp.Request) {
	const sdkAgent = "X-Amz-User-Agent"
	updateHTTPHeader(request, sdkAgent, u.sdkAgent.Build())
}

func updateHTTPHeader(request *smithyhttp.Request, header string, value string) {
	var current string
	if v := request.Header[header]; len(v) > 0 {
		current = v[0]
	}
	if len(current) > 0 {
		current = value + " " + current
	} else {
		current = value
	}
	request.Header[header] = append(request.Header[header][:0], current)
}

func rules(r rune) rune {
	switch {
	case r >= '0' && r <= '9':
		return r
	case r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z':
		return r
	case validChars[r]:
		return r
	default:
		return '-'
	}
}

package actionscache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dimchansky/utfbom"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var UploadConcurrency = 4
var UploadChunkSize = 32 * 1024 * 1024
var noValidateToken bool

const defaultUserAgent = "go-actions-cache/1.0"

var Log = func(string, ...interface{}) {}

type Blob interface {
	io.ReaderAt
	io.Closer
	Size() int64
}

type bufferBlob struct {
	io.ReaderAt
	size int64
}

func (b *bufferBlob) Size() int64 {
	return b.size
}

func (b *bufferBlob) Close() error {
	return nil
}

func NewBlob(dt []byte) Blob {
	return &bufferBlob{
		ReaderAt: bytes.NewReader(dt),
		size:     int64(len(dt)),
	}
}

func TryEnv(opt Opt) (*Cache, error) {
	var v2 bool
	if v, ok := os.LookupEnv("ACTIONS_CACHE_SERVICE_V2"); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			v2 = b
		}
	}

	if v, ok := os.LookupEnv("ACTIONS_CACHE_API_FORCE_VERSION"); ok && v != "" {
		switch v {
		case "v1":
			v2 = false
		case "v2":
			v2 = true
		default:
			return nil, errors.Errorf("invalid ACTIONS_CACHE_API_FORCE_VERSION %q", v)
		}
	}

	var token string
	var cacheURL string
	if v2 {
		cacheURL, _ = os.LookupEnv("ACTIONS_RESULTS_URL")
	} else {
		// ACTIONS_CACHE_URL=https://artifactcache.actions.githubusercontent.com/xxx/
		cacheURL, _ = os.LookupEnv("ACTIONS_CACHE_URL")
	}

	tokenEnc, ok := os.LookupEnv("GHCACHE_TOKEN_ENC")
	if ok {
		url, tkn, err := decryptToken(tokenEnc, os.Getenv("GHCACHE_TOKEN_PW"))
		if err != nil {
			return nil, err
		}
		if cacheURL == "" {
			cacheURL = url
		}
		token = tkn
	}

	if token == "" {
		token, _ = os.LookupEnv("ACTIONS_RUNTIME_TOKEN")
	}

	if token == "" {
		return nil, nil
	}

	return New(token, cacheURL, v2, opt)
}

type Opt struct {
	Client      *http.Client
	Timeout     time.Duration
	BackoffPool *BackoffPool
	UserAgent   string
}

func New(token, url string, v2 bool, opt Opt) (*Cache, error) {
	tk, _, err := new(jwt.Parser).ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	claims, ok := tk.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.Errorf("invalid token without claims map")
	}
	ac, ok := claims["ac"]
	if !ok {
		return nil, errors.Errorf("invalid token without access controls")
	}
	acs, ok := ac.(string)
	if !ok {
		return nil, errors.Errorf("invalid token with access controls type %T", ac)
	}

	exp, ok := claims["exp"]
	if !ok {
		return nil, errors.Errorf("invalid token without expiration time")
	}
	expf, ok := exp.(float64)
	if !ok {
		return nil, errors.Errorf("invalid token with expiration time type %T", acs)
	}
	expt := time.Unix(int64(expf), 0)

	if !noValidateToken && time.Now().After(expt) {
		return nil, errors.Errorf("cache token expired at %v", expt)
	}

	nbf, ok := claims["nbf"]
	if !ok {
		return nil, errors.Errorf("invalid token without expiration time")
	}
	nbff, ok := nbf.(float64)
	if !ok {
		return nil, errors.Errorf("invalid token with expiration time type %T", nbf)
	}
	nbft := time.Unix(int64(nbff), 0)

	if !noValidateToken && time.Now().Before(nbft) {
		return nil, errors.Errorf("invalid token with future issue time time %v", nbft)
	}

	scopes := []Scope{}
	if err := json.Unmarshal([]byte(acs), &scopes); err != nil {
		return nil, errors.Wrap(err, "failed to parse token access controls")
	}
	Log("parsed token: scopes: %+v, issued: %v, expires: %v", scopes, nbft, expt)

	opt = optsWithDefaults(opt)

	return &Cache{
		opt:       opt,
		scopes:    scopes,
		URL:       url,
		Token:     tk,
		IssuedAt:  nbft,
		ExpiresAt: expt,
		IsV2:      v2,
	}, nil
}

func optsWithDefaults(opt Opt) Opt {
	if opt.Client == nil {
		opt.Client = http.DefaultClient
	}
	if opt.Timeout == 0 {
		opt.Timeout = 5 * time.Minute
	}
	if opt.BackoffPool == nil {
		opt.BackoffPool = defaultBackoffPool
	}
	if opt.UserAgent == "" {
		opt.UserAgent = defaultUserAgent
	}
	return opt
}

type Scope struct {
	Scope      string
	Permission Permission
}

type Permission int

const (
	PermissionRead = 1 << iota
	PermissionWrite
)

func (p Permission) String() string {
	out := make([]string, 0, 2)
	if p&PermissionRead != 0 {
		out = append(out, "Read")
	}
	if p&PermissionWrite != 0 {
		out = append(out, "Write")
	}
	if p > PermissionRead|PermissionWrite {
		return strconv.Itoa(int(p))
	}
	return strings.Join(out, "|")
}

type Cache struct {
	opt       Opt
	scopes    []Scope
	URL       string
	Token     *jwt.Token
	IssuedAt  time.Time
	ExpiresAt time.Time
	IsV2      bool
}

func (c *Cache) Scopes() []Scope {
	return c.scopes
}

func (c *Cache) Load(ctx context.Context, keys ...string) (*Entry, error) {
	if c.IsV2 {
		return c.loadV2(ctx, keys...)
	}
	return c.loadV1(ctx, keys...)
}

func (c *Cache) loadV1(ctx context.Context, keys ...string) (*Entry, error) {
	u, err := url.Parse(c.url("cache"))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("keys", strings.Join(keys, ","))
	q.Set("version", version(keys[0]))
	u.RawQuery = q.Encode()

	req := c.newRequest("GET", u.String(), nil)
	Log("load cache %s", u.String())
	resp, err := c.doWithRetries(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var ce Entry
	dt, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if len(dt) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(dt, &ce); err != nil {
		return nil, errors.WithStack(err)
	}
	ce.client = c.opt.Client
	if ce.Key == "" {
		return nil, nil
	}
	return &ce, nil
}

func (c *Cache) reserve(ctx context.Context, key string) (string, string, error) {
	if c.IsV2 {
		url, err := c.reserveV2(ctx, key)
		if err != nil {
			return "", "", err
		}
		return key, url, nil
	}
	cid, err := c.reserveV1(ctx, key)
	if err != nil {
		return "", "", err
	}
	sid := strconv.Itoa(cid)
	return sid, sid, nil
}

func (c *Cache) reserveV1(ctx context.Context, key string) (int, error) {
	dt, err := json.Marshal(ReserveCacheReq{Key: key, Version: version(key)})
	if err != nil {
		return 0, errors.WithStack(err)
	}
	req := c.newRequest("POST", c.url("caches"), func() io.Reader {
		return bytes.NewReader(dt)
	})

	req.headers["Content-Type"] = "application/json"
	Log("save cache req %s body=%s", req.url, dt)
	resp, err := c.doWithRetries(ctx, req)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	dt, err = io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		return 0, errors.WithStack(err)
	}
	var cr ReserveCacheResp
	if err := json.Unmarshal(dt, &cr); err != nil {
		return 0, errors.Wrapf(err, "failed to unmarshal %s", dt)
	}
	if cr.CacheID == 0 {
		return 0, errors.Errorf("invalid response %s", dt)
	}
	Log("save cache resp: %s", dt)
	return cr.CacheID, nil
}

func (c *Cache) commit(ctx context.Context, id string, size int64) error {
	if c.IsV2 {
		return c.commitV2(ctx, id, size)
	}
	return c.commitV1(ctx, id, size)
}

func (c *Cache) commitV1(ctx context.Context, id string, size int64) error {
	dt, err := json.Marshal(CommitCacheReq{Size: size})
	if err != nil {
		return errors.WithStack(err)
	}
	req := c.newRequest("POST", c.url(fmt.Sprintf("caches/%s", id)), func() io.Reader {
		return bytes.NewReader(dt)
	})
	req.headers["Content-Type"] = "application/json"
	Log("commit cache %s, size %d", req.url, size)
	resp, err := c.doWithRetries(ctx, req)
	if err != nil {
		return errors.Wrapf(err, "error committing cache %s", id)
	}
	dt, err = io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		return err
	}
	if len(dt) != 0 {
		Log("commit response: %s", dt)
	}
	return resp.Body.Close()
}

func (c *Cache) upload(ctx context.Context, url string, b Blob) error {
	if c.IsV2 {
		return c.uploadV2(ctx, url, b)
	}
	return c.uploadV1(ctx, url, b)
}

func (c *Cache) uploadV1(ctx context.Context, id string, b Blob) error {
	var mu sync.Mutex
	eg, ctx := errgroup.WithContext(ctx)
	offset := int64(0)
	for i := 0; i < UploadConcurrency; i++ {
		eg.Go(func() error {
			for {
				mu.Lock()
				start := offset
				if start >= b.Size() {
					mu.Unlock()
					return nil
				}
				end := start + int64(UploadChunkSize)
				if end > b.Size() {
					end = b.Size()
				}
				offset = end
				mu.Unlock()

				if err := c.uploadChunk(ctx, id, b, start, end-start); err != nil {
					return err
				}
			}
		})
	}
	return eg.Wait()
}

func (c *Cache) Save(ctx context.Context, key string, b Blob) error {
	id, url, err := c.reserve(ctx, key)
	if err != nil {
		return err
	}

	if err := c.upload(ctx, url, b); err != nil {
		return err
	}

	return c.commit(ctx, id, b.Size())
}

// SaveMutable stores a blob over a possibly existing key. Previous value is passed to callback
// that needs to return new blob. Callback may be called multiple times if two saves happen during
// same time window. In case of a crash a key may remain locked, preventing previous changes. Timeout
// can be set to force changes in this case without guaranteeing that previous value was up to date.
func (c *Cache) SaveMutable(ctx context.Context, key string, forceTimeout time.Duration, f func(old *Entry) (Blob, error)) error {
	var blocked time.Duration
loop0:
	for {
		ce, err := c.Load(ctx, key+"#")
		if err != nil {
			return err
		}
		b, err := f(ce)
		if err != nil {
			return err
		}
		defer b.Close()
		if ce != nil {
			// check if index changed while loading
			ce2, err := c.Load(ctx, key+"#")
			if err != nil {
				return err
			}
			if ce2 == nil || ce2.Key != ce.Key {
				continue
			}
		}
		idx := 0
		if ce != nil {
			idxs := strings.TrimPrefix(ce.Key, key+"#")
			if idxs == "" {
				return errors.Errorf("corrupt empty index for %s", key)
			}
			idx, err = strconv.Atoi(idxs)
			if err != nil {
				return errors.Wrapf(err, "failed to parse %s index", key)
			}
		}
		var cacheID string
		var url string
		for {
			idx++
			cacheID, url, err = c.reserve(ctx, fmt.Sprintf("%s#%d", key, idx))
			if err != nil {
				if errors.Is(err, os.ErrExist) {
					if blocked <= forceTimeout {
						blocked += 2 * time.Second
						select {
						case <-ctx.Done():
							return ctx.Err()
						case <-time.After(2 * time.Second):
						}
						continue loop0
					}
					continue // index has been blocked a long time, maybe crashed, skip to next number
				}
				return err
			}
			break
		}
		if err := c.upload(ctx, url, b); err != nil {
			return nil
		}
		return c.commit(ctx, cacheID, b.Size())
	}
}

func (c *Cache) uploadChunk(ctx context.Context, id string, ra io.ReaderAt, off, n int64) error {
	req := c.newRequest("PATCH", c.url(fmt.Sprintf("caches/%s", id)), func() io.Reader {
		return io.NewSectionReader(ra, off, n)
	})
	req.headers["Content-Type"] = "application/octet-stream"
	req.headers["Content-Range"] = fmt.Sprintf("bytes %d-%d/*", off, off+n-1)

	Log("upload cache chunk %s, range %d-%d", req.url, off, off+n-1)
	resp, err := c.doWithRetries(ctx, req)
	if err != nil {
		return errors.WithStack(err)
	}
	dt, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		return errors.WithStack(err)
	}
	if len(dt) != 0 {
		Log("upload chunk resp: %s", dt)
	}
	return resp.Body.Close()
}

func (c *Cache) newRequest(method, url string, body func() io.Reader) *request {
	return &request{
		method: method,
		url:    url,
		body:   body,
		headers: map[string]string{
			"Authorization": "Bearer " + c.Token.Raw,
			"Accept":        "application/json;api-version=6.0-preview.1",
			"User-Agent":    c.opt.UserAgent,
		},
	}
}

func (c *Cache) doWithRetries(ctx context.Context, r *request) (*http.Response, error) {
	var lastErr error
	max := time.Now().Add(c.opt.Timeout)
	for {
		if err1 := c.opt.BackoffPool.Wait(ctx, time.Until(max)); err1 != nil {
			if lastErr != nil {
				return nil, errors.Wrapf(lastErr, "%v", err1)
			}
			return nil, err1
		}
		req, err := r.httpReq()
		if err != nil {
			return nil, err
		}
		req = req.WithContext(ctx)

		var resp *http.Response
		resp, err = c.opt.Client.Do(req)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if err := checkResponse(resp); err != nil {
			var he HTTPError
			if errors.As(err, &he) {
				if he.StatusCode == http.StatusTooManyRequests {
					c.opt.BackoffPool.Delay()
					lastErr = err
					continue
				}
			}
			c.opt.BackoffPool.Reset()
			return nil, err
		}
		c.opt.BackoffPool.Reset()
		return resp, nil
	}
}

func (c *Cache) url(p string) string {
	return strings.TrimRight(c.URL, "/") + "/_apis/artifactcache/" + p
}

func (c *Cache) AllKeys(ctx context.Context, api *RestAPI, prefix string) (map[string]struct{}, error) {
	m := map[string]struct{}{}
	var mu sync.Mutex
	eg, ctx := errgroup.WithContext(ctx)
	for _, s := range c.scopes {
		s := s
		eg.Go(func() error {
			keys, err := api.ListKeys(ctx, prefix, s.Scope)
			if err != nil {
				return err
			}
			mu.Lock()
			for _, k := range keys {
				m[k.Key] = struct{}{}
			}
			mu.Unlock()
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return m, nil
}

type ReserveCacheReq struct {
	Key     string `json:"key"`
	Version string `json:"version"`
}

type ReserveCacheResp struct {
	CacheID int `json:"cacheID"`
}

type CommitCacheReq struct {
	Size int64 `json:"size"`
}

type Entry struct {
	Key         string `json:"cacheKey"`
	Scope       string `json:"scope"`
	URL         string `json:"archiveLocation"`
	IsAzureBlob bool   `json:"isAzureBlob"`

	client *http.Client
	reload func(context.Context) error
}

func (ce *Entry) WriteTo(ctx context.Context, w io.Writer) error {
	rac := ce.Download(ctx)
	if _, err := io.Copy(w, &rc{ReaderAt: rac}); err != nil {
		return err
	}
	return rac.Close()
}

// Download returns a ReaderAtCloser for pulling the data. Concurrent reads are not allowed
func (ce *Entry) Download(ctx context.Context) ReaderAtCloser {
	if ce.IsAzureBlob {
		return ce.downloadV2(ctx)
	}
	return toReaderAtCloser(func(offset int64) (io.ReadCloser, error) {
		req, err := http.NewRequest("GET", ce.URL, nil)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		req = req.WithContext(ctx)
		if offset != 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}
		client := ce.client
		if client == nil {
			client = http.DefaultClient
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
				return nil, errors.Errorf("invalid status response %v for %s, range: %v", resp.Status, ce.URL, req.Header.Get("Range"))
			}
			return nil, errors.Errorf("invalid status response %v for %s", resp.Status, ce.URL)
		}
		if offset != 0 {
			cr := resp.Header.Get("content-range")
			if !strings.HasPrefix(cr, fmt.Sprintf("bytes %d-", offset)) {
				resp.Body.Close()
				return nil, errors.Errorf("unhandled content range in response: %v", cr)
			}
		}
		return resp.Body, nil
	})
}

type request struct {
	method  string
	url     string
	body    func() io.Reader
	headers map[string]string
}

func (r *request) httpReq() (*http.Request, error) {
	var body io.Reader
	if r.body != nil {
		body = r.body()
	}
	req, err := http.NewRequest(r.method, r.url, body)
	if err != nil {
		return nil, err
	}
	for k, v := range r.headers {
		req.Header.Add(k, v)
	}
	return req, nil
}

func version(k string) string {
	h := sha256.New()
	// h.Write([]byte(k))
	// upstream uses paths in version, we don't seem to have anything that is unique like this
	h.Write([]byte("|go-actionscache-1.0"))
	return hex.EncodeToString(h.Sum(nil))
}

type GithubAPIError struct {
	Message   string `json:"message"`
	TypeName  string `json:"typeName"`
	TypeKey   string `json:"typeKey"`
	ErrorCode int    `json:"errorCode"`
}

func (e GithubAPIError) Error() string {
	return e.Message
}

func (e GithubAPIError) Is(err error) bool {
	if err == os.ErrExist {
		if strings.Contains(e.TypeKey, "AlreadyExists") {
			return true
		}
		// for safety, in case error gets updated
		if strings.Contains(strings.ToLower(e.Message), "already exists") {
			return true
		}
	}
	return false
}

type HTTPError struct {
	StatusCode int
	Err        error
}

func (e HTTPError) Error() string {
	return e.Err.Error()
}

func (e HTTPError) Unwrap() error {
	return e.Err
}

func checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	dt, err := io.ReadAll(utfbom.SkipOnly(io.LimitReader(resp.Body, 32*1024)))
	if err != nil {
		return errors.WithStack(err)
	}
	var gae GithubAPIError
	if err1 := json.Unmarshal(dt, &gae); err1 != nil {
		err = errors.Wrapf(err1, "failed to parse error response %d: %s", resp.StatusCode, dt)
	} else if gae.Message != "" {
		err = errors.WithStack(gae)
	} else {
		var errorV2 struct {
			Code string `json:"code"`
			Msg  string `json:"message"`
		}
		if err1 := json.Unmarshal(dt, &errorV2); err1 == nil && errorV2.Code != "" {
			gae.Message = errorV2.Msg
			if gae.Message == "" {
				gae.Message = errorV2.Code
			} else {
				gae.Message = resp.Status
			}
			if errorV2.Code == "already_exists" {
				errorV2.Code = "ArtifactCacheItemAlreadyExistsException"
			}
			gae.TypeKey = errorV2.Code
			err = errors.WithStack(gae)
		} else {
			err = errors.Errorf("unknown error %s: %s", resp.Status, dt)
		}
	}

	return HTTPError{
		StatusCode: resp.StatusCode,
		Err:        err,
	}
}

func decryptToken(enc, pass string) (string, string, error) {
	// openssl key derivation uses some non-standard algorithm so exec instead of using go libraries
	// this is only used on testing anyway
	cmd := exec.Command("openssl", "enc", "-d", "-aes-256-cbc", "-a", "-A", "-salt", "-md", "sha256", "-pass", "env:GHCACHE_TOKEN_PW")
	cmd.Env = append(cmd.Env, fmt.Sprintf("GHCACHE_TOKEN_PW=%s", pass))
	cmd.Stdin = bytes.NewReader([]byte(enc))
	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", "", err
	}
	parts := bytes.SplitN(buf.Bytes(), []byte(":::"), 2)
	if len(parts) != 2 {
		return "", "", errors.Errorf("invalid decrypt contents %s", buf.String())
	}
	return string(parts[0]), strings.TrimSpace(string(parts[1])), nil
}

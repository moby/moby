package grpcreflect

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	refv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	refv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/internal"
)

// If we try the v1 reflection API and get back "not implemented", we'll wait
// this long before trying v1 again. This allows a long-lived client to
// dynamically switch from v1alpha to v1 if the underlying server is updated
// to support it. But it also prevents every stream request from always trying
// v1 first: if we try it and see it fail, we shouldn't continually retry it
// if we expect it will fail again.
const durationBetweenV1Attempts = time.Hour

// elementNotFoundError is the error returned by reflective operations where the
// server does not recognize a given file name, symbol name, or extension.
type elementNotFoundError struct {
	name    string
	kind    elementKind
	symType symbolType // only used when kind == elementKindSymbol
	tag     int32      // only used when kind == elementKindExtension

	// only errors with a kind of elementKindFile will have a cause, which means
	// the named file count not be resolved because of a dependency that could
	// not be found where cause describes the missing dependency
	cause *elementNotFoundError
}

type elementKind int

const (
	elementKindSymbol elementKind = iota
	elementKindFile
	elementKindExtension
)

type symbolType string

const (
	symbolTypeService = "Service"
	symbolTypeMessage = "Message"
	symbolTypeEnum    = "Enum"
	symbolTypeUnknown = "Symbol"
)

func symbolNotFound(symbol string, symType symbolType, cause *elementNotFoundError) error {
	if cause != nil && cause.kind == elementKindSymbol && cause.name == symbol {
		// no need to wrap
		if symType != symbolTypeUnknown && cause.symType == symbolTypeUnknown {
			// We previously didn't know symbol type but now do?
			// Create a new error that has the right symbol type.
			return &elementNotFoundError{name: symbol, symType: symType, kind: elementKindSymbol}
		}
		return cause
	}
	return &elementNotFoundError{name: symbol, symType: symType, kind: elementKindSymbol, cause: cause}
}

func extensionNotFound(extendee string, tag int32, cause *elementNotFoundError) error {
	if cause != nil && cause.kind == elementKindExtension && cause.name == extendee && cause.tag == tag {
		// no need to wrap
		return cause
	}
	return &elementNotFoundError{name: extendee, tag: tag, kind: elementKindExtension, cause: cause}
}

func fileNotFound(file string, cause *elementNotFoundError) error {
	if cause != nil && cause.kind == elementKindFile && cause.name == file {
		// no need to wrap
		return cause
	}
	return &elementNotFoundError{name: file, kind: elementKindFile, cause: cause}
}

func (e *elementNotFoundError) Error() string {
	first := true
	var b bytes.Buffer
	for ; e != nil; e = e.cause {
		if first {
			first = false
		} else {
			_, _ = fmt.Fprint(&b, "\ncaused by: ")
		}
		switch e.kind {
		case elementKindSymbol:
			_, _ = fmt.Fprintf(&b, "%s not found: %s", e.symType, e.name)
		case elementKindExtension:
			_, _ = fmt.Fprintf(&b, "Extension not found: tag %d for %s", e.tag, e.name)
		default:
			_, _ = fmt.Fprintf(&b, "File not found: %s", e.name)
		}
	}
	return b.String()
}

// IsElementNotFoundError determines if the given error indicates that a file
// name, symbol name, or extension field was could not be found by the server.
func IsElementNotFoundError(err error) bool {
	_, ok := err.(*elementNotFoundError)
	return ok
}

// ProtocolError is an error returned when the server sends a response of the
// wrong type.
type ProtocolError struct {
	missingType reflect.Type
}

func (p ProtocolError) Error() string {
	return fmt.Sprintf("Protocol error: response was missing %v", p.missingType)
}

type extDesc struct {
	extendedMessageName string
	extensionNumber     int32
}

type resolvers struct {
	descriptorResolver protodesc.Resolver
	extensionResolver  protoregistry.ExtensionTypeResolver
}

type fileEntry struct {
	fd       *desc.FileDescriptor
	fallback bool
}

// Client is a client connection to a server for performing reflection calls
// and resolving remote symbols.
type Client struct {
	ctx              context.Context
	now              func() time.Time
	stubV1           refv1.ServerReflectionClient
	stubV1Alpha      refv1alpha.ServerReflectionClient
	allowMissing     atomic.Bool
	fallbackResolver atomic.Pointer[resolvers]

	connMu      sync.Mutex
	cancel      context.CancelFunc
	stream      refv1.ServerReflection_ServerReflectionInfoClient
	useV1Alpha  bool
	lastTriedV1 time.Time

	cacheMu          sync.RWMutex
	protosByName     map[string]*descriptorpb.FileDescriptorProto
	filesByName      map[string]fileEntry
	filesBySymbol    map[string]fileEntry
	filesByExtension map[extDesc]fileEntry
}

// NewClient creates a new Client with the given root context and using the
// given RPC stub for talking to the server.
//
// Deprecated: Use NewClientV1Alpha if you are intentionally pinning the
// v1alpha version of the reflection service. Otherwise, use NewClientAuto
// instead.
func NewClient(ctx context.Context, stub refv1alpha.ServerReflectionClient) *Client {
	return NewClientV1Alpha(ctx, stub)
}

// NewClientV1Alpha creates a new Client using the v1alpha version of reflection
// with the given root context and using the given RPC stub for talking to the
// server.
func NewClientV1Alpha(ctx context.Context, stub refv1alpha.ServerReflectionClient) *Client {
	return newClient(ctx, nil, stub)
}

// NewClientV1 creates a new Client using the v1 version of reflection with the
// given root context and using the given RPC stub for talking to the server.
func NewClientV1(ctx context.Context, stub refv1.ServerReflectionClient) *Client {
	return newClient(ctx, stub, nil)
}

func newClient(ctx context.Context, stubv1 refv1.ServerReflectionClient, stubv1alpha refv1alpha.ServerReflectionClient) *Client {
	cr := &Client{
		ctx:              ctx,
		now:              time.Now,
		stubV1:           stubv1,
		stubV1Alpha:      stubv1alpha,
		protosByName:     map[string]*descriptorpb.FileDescriptorProto{},
		filesByName:      map[string]fileEntry{},
		filesBySymbol:    map[string]fileEntry{},
		filesByExtension: map[extDesc]fileEntry{},
	}
	// don't leak a grpc stream
	runtime.SetFinalizer(cr, (*Client).Reset)
	return cr
}

// NewClientAuto creates a new Client that will use either v1 or v1alpha version
// of reflection (based on what the server supports) with the given root context
// and using the given client connection.
//
// It will first the v1 version of the reflection service. If it gets back an
// "Unimplemented" error, it will fall back to using the v1alpha version. It
// will remember which version the server supports for any subsequent operations
// that need to re-invoke the streaming RPC. But, if it's a very long-lived
// client, it will periodically retry the v1 version (in case the server is
// updated to support it also). The period for these retries is every hour.
func NewClientAuto(ctx context.Context, cc grpc.ClientConnInterface) *Client {
	stubv1 := refv1.NewServerReflectionClient(cc)
	stubv1alpha := refv1alpha.NewServerReflectionClient(cc)
	return newClient(ctx, stubv1, stubv1alpha)
}

// AllowMissingFileDescriptors configures the client to allow missing files
// when building descriptors when possible. Missing files are often fatal
// errors, but with this option they can sometimes be worked around. Building
// a schema can only succeed with some files missing if the files in question
// only provide custom options and/or other unused types.
func (cr *Client) AllowMissingFileDescriptors() {
	cr.allowMissing.Store(true)
}

// AllowFallbackResolver configures the client to allow falling back to the
// given resolvers if the server is unable to supply descriptors for a particular
// query. This allows working around issues where servers' reflection service
// provides an incomplete set of descriptors, but the client has knowledge of
// the missing descriptors from another source. It is usually most appropriate
// to pass [protoregistry.GlobalFiles] and [protoregistry.GlobalTypes] as the
// resolver values.
//
// The first value is used as a fallback for FileByFilename and FileContainingSymbol
// queries. The second value is used as a fallback for FileContainingExtension. It
// can also be used as a fallback for AllExtensionNumbersForType if it provides
// a method with the following signature (which *[protoregistry.Types] provides):
//
//	RangeExtensionsByMessage(message protoreflect.FullName, f func(protoreflect.ExtensionType) bool)
func (cr *Client) AllowFallbackResolver(descriptors protodesc.Resolver, exts protoregistry.ExtensionTypeResolver) {
	if descriptors == nil && exts == nil {
		cr.fallbackResolver.Store(nil)
	} else {
		cr.fallbackResolver.Store(&resolvers{
			descriptorResolver: descriptors,
			extensionResolver:  exts,
		})
	}
}

// FileByFilename asks the server for a file descriptor for the proto file with
// the given name.
func (cr *Client) FileByFilename(filename string) (*desc.FileDescriptor, error) {
	// hit the cache first
	cr.cacheMu.RLock()
	if entry, ok := cr.filesByName[filename]; ok {
		cr.cacheMu.RUnlock()
		return entry.fd, nil
	}
	// not there? see if we've downloaded the proto
	fdp, ok := cr.protosByName[filename]
	cr.cacheMu.RUnlock()
	if ok {
		return cr.descriptorFromProto(fdp)
	}

	req := &refv1.ServerReflectionRequest{
		MessageRequest: &refv1.ServerReflectionRequest_FileByFilename{
			FileByFilename: filename,
		},
	}
	accept := func(fd *desc.FileDescriptor) bool {
		return fd.GetName() == filename
	}

	fd, err := cr.getAndCacheFileDescriptors(req, filename, "", accept)
	if isNotFound(err) {
		// File not found? see if we can look up via alternate name
		if alternate, ok := internal.StdFileAliases[filename]; ok {
			req := &refv1.ServerReflectionRequest{
				MessageRequest: &refv1.ServerReflectionRequest_FileByFilename{
					FileByFilename: alternate,
				},
			}
			fd, err = cr.getAndCacheFileDescriptors(req, alternate, filename, accept)
		}
	}
	if isNotFound(err) {
		// Still no? See if we can use a fallback resolver
		resolver := cr.fallbackResolver.Load()
		if resolver != nil && resolver.descriptorResolver != nil {
			fileDesc, fallbackErr := resolver.descriptorResolver.FindFileByPath(filename)
			if fallbackErr == nil {
				var wrapErr error
				fd, wrapErr = desc.WrapFile(fileDesc)
				if wrapErr == nil {
					fd = cr.cacheFile(fd, true)
					err = nil // clear error since we've succeeded via the fallback
				}
			}
		}
	}
	if isNotFound(err) {
		err = fileNotFound(filename, nil)
	} else if e, ok := err.(*elementNotFoundError); ok {
		err = fileNotFound(filename, e)
	}

	return fd, err
}

// FileContainingSymbol asks the server for a file descriptor for the proto file
// that declares the given fully-qualified symbol.
func (cr *Client) FileContainingSymbol(symbol string) (*desc.FileDescriptor, error) {
	// hit the cache first
	cr.cacheMu.RLock()
	entry, ok := cr.filesBySymbol[symbol]
	cr.cacheMu.RUnlock()
	if ok {
		return entry.fd, nil
	}

	req := &refv1.ServerReflectionRequest{
		MessageRequest: &refv1.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: symbol,
		},
	}
	accept := func(fd *desc.FileDescriptor) bool {
		return fd.FindSymbol(symbol) != nil
	}
	fd, err := cr.getAndCacheFileDescriptors(req, "", "", accept)
	if isNotFound(err) {
		// Symbol not found? See if we can use a fallback resolver
		resolver := cr.fallbackResolver.Load()
		if resolver != nil && resolver.descriptorResolver != nil {
			d, fallbackErr := resolver.descriptorResolver.FindDescriptorByName(protoreflect.FullName(symbol))
			if fallbackErr == nil {
				var wrapErr error
				fd, wrapErr = desc.WrapFile(d.ParentFile())
				if wrapErr == nil {
					fd = cr.cacheFile(fd, true)
					err = nil // clear error since we've succeeded via the fallback
				}
			}
		}
	}
	if isNotFound(err) {
		err = symbolNotFound(symbol, symbolTypeUnknown, nil)
	} else if e, ok := err.(*elementNotFoundError); ok {
		err = symbolNotFound(symbol, symbolTypeUnknown, e)
	}
	return fd, err
}

// FileContainingExtension asks the server for a file descriptor for the proto
// file that declares an extension with the given number for the given
// fully-qualified message name.
func (cr *Client) FileContainingExtension(extendedMessageName string, extensionNumber int32) (*desc.FileDescriptor, error) {
	// hit the cache first
	cr.cacheMu.RLock()
	entry, ok := cr.filesByExtension[extDesc{extendedMessageName, extensionNumber}]
	cr.cacheMu.RUnlock()
	if ok {
		return entry.fd, nil
	}

	req := &refv1.ServerReflectionRequest{
		MessageRequest: &refv1.ServerReflectionRequest_FileContainingExtension{
			FileContainingExtension: &refv1.ExtensionRequest{
				ContainingType:  extendedMessageName,
				ExtensionNumber: extensionNumber,
			},
		},
	}
	accept := func(fd *desc.FileDescriptor) bool {
		return fd.FindExtension(extendedMessageName, extensionNumber) != nil
	}
	fd, err := cr.getAndCacheFileDescriptors(req, "", "", accept)
	if isNotFound(err) {
		// Extension not found? See if we can use a fallback resolver
		resolver := cr.fallbackResolver.Load()
		if resolver != nil && resolver.extensionResolver != nil {
			extType, fallbackErr := resolver.extensionResolver.FindExtensionByNumber(protoreflect.FullName(extendedMessageName), protoreflect.FieldNumber(extensionNumber))
			if fallbackErr == nil {
				var wrapErr error
				fd, wrapErr = desc.WrapFile(extType.TypeDescriptor().ParentFile())
				if wrapErr == nil {
					fd = cr.cacheFile(fd, true)
					err = nil // clear error since we've succeeded via the fallback
				}
			}
		}
	}
	if isNotFound(err) {
		err = extensionNotFound(extendedMessageName, extensionNumber, nil)
	} else if e, ok := err.(*elementNotFoundError); ok {
		err = extensionNotFound(extendedMessageName, extensionNumber, e)
	}
	return fd, err
}

func (cr *Client) getAndCacheFileDescriptors(req *refv1.ServerReflectionRequest, expectedName, alias string, accept func(*desc.FileDescriptor) bool) (*desc.FileDescriptor, error) {
	resp, err := cr.send(req)
	if err != nil {
		return nil, err
	}

	fdResp := resp.GetFileDescriptorResponse()
	if fdResp == nil {
		return nil, &ProtocolError{reflect.TypeOf(fdResp).Elem()}
	}

	// Response can contain the result file descriptor, but also its transitive
	// deps. Furthermore, protocol states that subsequent requests do not need
	// to send transitive deps that have been sent in prior responses. So we
	// need to cache all file descriptors and then return the first one (which
	// should be the answer). If we're looking for a file by name, we can be
	// smarter and make sure to grab one by name instead of just grabbing the
	// first one.
	var fds []*descriptorpb.FileDescriptorProto
	for _, fdBytes := range fdResp.FileDescriptorProto {
		fd := &descriptorpb.FileDescriptorProto{}
		if err = proto.Unmarshal(fdBytes, fd); err != nil {
			return nil, err
		}

		if expectedName != "" && alias != "" && expectedName != alias && fd.GetName() == expectedName {
			// we found a file was aliased, so we need to update the proto to reflect that
			fd.Name = proto.String(alias)
		}

		cr.cacheMu.Lock()
		// store in cache of raw descriptor protos, but don't overwrite existing protos
		if existingFd, ok := cr.protosByName[fd.GetName()]; ok {
			fd = existingFd
		} else {
			cr.protosByName[fd.GetName()] = fd
		}
		cr.cacheMu.Unlock()

		fds = append(fds, fd)
	}

	// find the right result from the files returned
	for _, fd := range fds {
		result, err := cr.descriptorFromProto(fd)
		if err != nil {
			return nil, err
		}
		if accept(result) {
			return result, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "response does not include expected file")
}

func (cr *Client) descriptorFromProto(fd *descriptorpb.FileDescriptorProto) (*desc.FileDescriptor, error) {
	allowMissing := cr.allowMissing.Load()
	deps := make([]*desc.FileDescriptor, 0, len(fd.GetDependency()))
	var deferredErr error
	var missingDeps []int
	for i, depName := range fd.GetDependency() {
		if dep, err := cr.FileByFilename(depName); err != nil {
			if _, ok := err.(*elementNotFoundError); !ok || !allowMissing {
				return nil, err
			}
			// We'll ignore for now to see if the file is really necessary.
			// (If it only supplies custom options, we can get by without it.)
			if deferredErr == nil {
				deferredErr = err
			}
			missingDeps = append(missingDeps, i)
		} else {
			deps = append(deps, dep)
		}
	}
	if len(missingDeps) > 0 {
		fd = fileWithoutDeps(fd, missingDeps)
	}
	d, err := desc.CreateFileDescriptor(fd, deps...)
	if err != nil {
		if deferredErr != nil {
			// assume the issue is the missing dep
			return nil, deferredErr
		}
		return nil, err
	}
	d = cr.cacheFile(d, false)
	return d, nil
}

func (cr *Client) cacheFile(fd *desc.FileDescriptor, fallback bool) *desc.FileDescriptor {
	cr.cacheMu.Lock()
	defer cr.cacheMu.Unlock()

	// Cache file descriptor by name. If we can't overwrite an existing
	// entry, return it. (Existing entry could come from concurrent caller.)
	if existing, ok := cr.filesByName[fd.GetName()]; ok && !canOverwrite(existing, fallback) {
		return existing.fd
	}
	entry := fileEntry{fd: fd, fallback: fallback}
	cr.filesByName[fd.GetName()] = entry

	// also cache by symbols and extensions
	for _, m := range fd.GetMessageTypes() {
		cr.cacheMessageLocked(m, entry)
	}
	for _, e := range fd.GetEnumTypes() {
		if !cr.maybeCacheFileBySymbol(e.GetFullyQualifiedName(), entry) {
			continue
		}
		for _, v := range e.GetValues() {
			cr.maybeCacheFileBySymbol(v.GetFullyQualifiedName(), entry)
		}
	}
	for _, e := range fd.GetExtensions() {
		if !cr.maybeCacheFileBySymbol(e.GetFullyQualifiedName(), entry) {
			continue
		}
		cr.maybeCacheFileByExtension(extDesc{e.GetOwner().GetFullyQualifiedName(), e.GetNumber()}, entry)
	}
	for _, s := range fd.GetServices() {
		if !cr.maybeCacheFileBySymbol(s.GetFullyQualifiedName(), entry) {
			continue
		}
		for _, m := range s.GetMethods() {
			cr.maybeCacheFileBySymbol(m.GetFullyQualifiedName(), entry)
		}
	}

	return fd
}

func (cr *Client) cacheMessageLocked(md *desc.MessageDescriptor, entry fileEntry) {
	if !cr.maybeCacheFileBySymbol(md.GetFullyQualifiedName(), entry) {
		return
	}
	for _, f := range md.GetFields() {
		cr.maybeCacheFileBySymbol(f.GetFullyQualifiedName(), entry)
	}
	for _, o := range md.GetOneOfs() {
		cr.maybeCacheFileBySymbol(o.GetFullyQualifiedName(), entry)
	}
	for _, e := range md.GetNestedEnumTypes() {
		if !cr.maybeCacheFileBySymbol(e.GetFullyQualifiedName(), entry) {
			continue
		}
		for _, v := range e.GetValues() {
			cr.maybeCacheFileBySymbol(v.GetFullyQualifiedName(), entry)
		}
	}
	for _, e := range md.GetNestedExtensions() {
		if !cr.maybeCacheFileBySymbol(e.GetFullyQualifiedName(), entry) {
			continue
		}
		cr.maybeCacheFileByExtension(extDesc{e.GetOwner().GetFullyQualifiedName(), e.GetNumber()}, entry)
	}
	for _, m := range md.GetNestedMessageTypes() {
		cr.cacheMessageLocked(m, entry) // recurse
	}
}

func canOverwrite(existing fileEntry, fallback bool) bool {
	return !fallback && existing.fallback
}

func (cr *Client) maybeCacheFileBySymbol(symbol string, entry fileEntry) bool {
	existing, ok := cr.filesBySymbol[symbol]
	if ok && !canOverwrite(existing, entry.fallback) {
		return false
	}
	cr.filesBySymbol[symbol] = entry
	return true
}

func (cr *Client) maybeCacheFileByExtension(ext extDesc, entry fileEntry) {
	existing, ok := cr.filesByExtension[ext]
	if ok && !canOverwrite(existing, entry.fallback) {
		return
	}
	cr.filesByExtension[ext] = entry
}

// AllExtensionNumbersForType asks the server for all known extension numbers
// for the given fully-qualified message name.
func (cr *Client) AllExtensionNumbersForType(extendedMessageName string) ([]int32, error) {
	req := &refv1.ServerReflectionRequest{
		MessageRequest: &refv1.ServerReflectionRequest_AllExtensionNumbersOfType{
			AllExtensionNumbersOfType: extendedMessageName,
		},
	}
	resp, err := cr.send(req)
	var exts []int32
	if err != nil && !isNotFound(err) {
		// If the server doesn't know about the message type and returns "not found",
		// we'll treat that as "no known extensions" instead of returning an error.
		return nil, err
	}
	if err == nil {
		extResp := resp.GetAllExtensionNumbersResponse()
		if extResp == nil {
			return nil, &ProtocolError{reflect.TypeOf(extResp).Elem()}
		}
		exts = extResp.ExtensionNumber
	}

	resolver := cr.fallbackResolver.Load()
	if resolver != nil && resolver.extensionResolver != nil {
		type extRanger interface {
			RangeExtensionsByMessage(message protoreflect.FullName, f func(protoreflect.ExtensionType) bool)
		}
		if ranger, ok := resolver.extensionResolver.(extRanger); ok {
			// Merge results with fallback resolver
			extSet := map[int32]struct{}{}
			ranger.RangeExtensionsByMessage(protoreflect.FullName(extendedMessageName), func(extType protoreflect.ExtensionType) bool {
				extSet[int32(extType.TypeDescriptor().Number())] = struct{}{}
				return true
			})
			if len(extSet) > 0 {
				// De-dupe with the set of extension numbers we got
				// from the server and merge the results back into exts.
				for _, ext := range exts {
					extSet[ext] = struct{}{}
				}
				exts = make([]int32, 0, len(extSet))
				for ext := range extSet {
					exts = append(exts, ext)
				}
				sort.Slice(exts, func(i, j int) bool { return exts[i] < exts[j] })
			}
		}
	}
	return exts, nil
}

// ListServices asks the server for the fully-qualified names of all exposed
// services.
func (cr *Client) ListServices() ([]string, error) {
	req := &refv1.ServerReflectionRequest{
		MessageRequest: &refv1.ServerReflectionRequest_ListServices{
			// proto doesn't indicate any purpose for this value and server impl
			// doesn't actually use it...
			ListServices: "*",
		},
	}
	resp, err := cr.send(req)
	if err != nil {
		return nil, err
	}

	listResp := resp.GetListServicesResponse()
	if listResp == nil {
		return nil, &ProtocolError{reflect.TypeOf(listResp).Elem()}
	}
	serviceNames := make([]string, len(listResp.Service))
	for i, s := range listResp.Service {
		serviceNames[i] = s.Name
	}
	return serviceNames, nil
}

func (cr *Client) send(req *refv1.ServerReflectionRequest) (*refv1.ServerReflectionResponse, error) {
	// we allow one immediate retry, in case we have a stale stream
	// (e.g. closed by server)
	resp, err := cr.doSend(req)
	if err != nil {
		return nil, err
	}

	// convert error response messages into errors
	errResp := resp.GetErrorResponse()
	if errResp != nil {
		return nil, status.Errorf(codes.Code(errResp.ErrorCode), "%s", errResp.ErrorMessage)
	}

	return resp, nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	s, ok := status.FromError(err)
	return ok && s.Code() == codes.NotFound
}

func (cr *Client) doSend(req *refv1.ServerReflectionRequest) (*refv1.ServerReflectionResponse, error) {
	// TODO: Streams are thread-safe, so we shouldn't need to lock. But without locking, we'll need more machinery
	// (goroutines and channels) to ensure that responses are correctly correlated with their requests and thus
	// delivered in correct oder.
	cr.connMu.Lock()
	defer cr.connMu.Unlock()
	return cr.doSendLocked(0, nil, req)
}

func (cr *Client) doSendLocked(attemptCount int, prevErr error, req *refv1.ServerReflectionRequest) (*refv1.ServerReflectionResponse, error) {
	if attemptCount >= 3 && prevErr != nil {
		return nil, prevErr
	}
	if (status.Code(prevErr) == codes.Unimplemented ||
		status.Code(prevErr) == codes.Unavailable) &&
		cr.useV1() {
		// If v1 is unimplemented, fallback to v1alpha.
		// We also fallback on unavailable because some servers have been
		// observed to close the connection/cancel the stream, w/out sending
		// back status or headers, when the service name is not known. When
		// this happens, the RPC status code is unavailable.
		// See https://github.com/fullstorydev/grpcurl/issues/434
		cr.useV1Alpha = true
		cr.lastTriedV1 = cr.now()
	}
	attemptCount++

	if err := cr.initStreamLocked(); err != nil {
		return nil, err
	}

	if err := cr.stream.Send(req); err != nil {
		if err == io.EOF {
			// if send returns EOF, must call Recv to get real underlying error
			_, err = cr.stream.Recv()
		}
		cr.resetLocked()
		return cr.doSendLocked(attemptCount, err, req)
	}

	resp, err := cr.stream.Recv()
	if err != nil {
		cr.resetLocked()
		return cr.doSendLocked(attemptCount, err, req)
	}
	return resp, nil
}

func (cr *Client) initStreamLocked() error {
	if cr.stream != nil {
		return nil
	}
	var newCtx context.Context
	newCtx, cr.cancel = context.WithCancel(cr.ctx)
	if cr.useV1Alpha == true && cr.now().Sub(cr.lastTriedV1) > durationBetweenV1Attempts {
		// we're due for periodic retry of v1
		cr.useV1Alpha = false
	}
	if cr.useV1() {
		// try the v1 API
		streamv1, err := cr.stubV1.ServerReflectionInfo(newCtx)
		if err == nil {
			cr.stream = streamv1
			return nil
		}
		if status.Code(err) != codes.Unimplemented {
			return err
		}
		// oh well, fall through below to try v1alpha and update state
		// so we skip straight to v1alpha next time
		cr.useV1Alpha = true
		cr.lastTriedV1 = cr.now()
	}
	var err error
	streamv1alpha, err := cr.stubV1Alpha.ServerReflectionInfo(newCtx)
	if err == nil {
		cr.stream = adaptStreamFromV1Alpha{streamv1alpha}
		return nil
	}
	return err
}

func (cr *Client) useV1() bool {
	return !cr.useV1Alpha && cr.stubV1 != nil
}

// Reset ensures that any active stream with the server is closed, releasing any
// resources.
func (cr *Client) Reset() {
	cr.connMu.Lock()
	defer cr.connMu.Unlock()
	cr.resetLocked()
}

func (cr *Client) resetLocked() {
	if cr.stream != nil {
		_ = cr.stream.CloseSend()
		for {
			// drain the stream, this covers io.EOF too
			if _, err := cr.stream.Recv(); err != nil {
				break
			}
		}
		cr.stream = nil
	}
	if cr.cancel != nil {
		cr.cancel()
		cr.cancel = nil
	}
}

// ResolveService asks the server to resolve the given fully-qualified service
// name into a service descriptor.
func (cr *Client) ResolveService(serviceName string) (*desc.ServiceDescriptor, error) {
	file, err := cr.FileContainingSymbol(serviceName)
	if err != nil {
		return nil, setSymbolType(err, serviceName, symbolTypeService)
	}
	d := file.FindSymbol(serviceName)
	if d == nil {
		return nil, symbolNotFound(serviceName, symbolTypeService, nil)
	}
	if s, ok := d.(*desc.ServiceDescriptor); ok {
		return s, nil
	} else {
		return nil, symbolNotFound(serviceName, symbolTypeService, nil)
	}
}

// ResolveMessage asks the server to resolve the given fully-qualified message
// name into a message descriptor.
func (cr *Client) ResolveMessage(messageName string) (*desc.MessageDescriptor, error) {
	file, err := cr.FileContainingSymbol(messageName)
	if err != nil {
		return nil, setSymbolType(err, messageName, symbolTypeMessage)
	}
	d := file.FindSymbol(messageName)
	if d == nil {
		return nil, symbolNotFound(messageName, symbolTypeMessage, nil)
	}
	if s, ok := d.(*desc.MessageDescriptor); ok {
		return s, nil
	} else {
		return nil, symbolNotFound(messageName, symbolTypeMessage, nil)
	}
}

// ResolveEnum asks the server to resolve the given fully-qualified enum name
// into an enum descriptor.
func (cr *Client) ResolveEnum(enumName string) (*desc.EnumDescriptor, error) {
	file, err := cr.FileContainingSymbol(enumName)
	if err != nil {
		return nil, setSymbolType(err, enumName, symbolTypeEnum)
	}
	d := file.FindSymbol(enumName)
	if d == nil {
		return nil, symbolNotFound(enumName, symbolTypeEnum, nil)
	}
	if s, ok := d.(*desc.EnumDescriptor); ok {
		return s, nil
	} else {
		return nil, symbolNotFound(enumName, symbolTypeEnum, nil)
	}
}

func setSymbolType(err error, name string, symType symbolType) error {
	if e, ok := err.(*elementNotFoundError); ok {
		if e.kind == elementKindSymbol && e.name == name && e.symType == symbolTypeUnknown {
			e.symType = symType
		}
	}
	return err
}

// ResolveEnumValues asks the server to resolve the given fully-qualified enum
// name into a map of names to numbers that represents the enum's values.
func (cr *Client) ResolveEnumValues(enumName string) (map[string]int32, error) {
	enumDesc, err := cr.ResolveEnum(enumName)
	if err != nil {
		return nil, err
	}
	vals := map[string]int32{}
	for _, valDesc := range enumDesc.GetValues() {
		vals[valDesc.GetName()] = valDesc.GetNumber()
	}
	return vals, nil
}

// ResolveExtension asks the server to resolve the given extension number and
// fully-qualified message name into a field descriptor.
func (cr *Client) ResolveExtension(extendedType string, extensionNumber int32) (*desc.FieldDescriptor, error) {
	file, err := cr.FileContainingExtension(extendedType, extensionNumber)
	if err != nil {
		return nil, err
	}
	d := findExtension(extendedType, extensionNumber, fileDescriptorExtensions{file})
	if d == nil {
		return nil, extensionNotFound(extendedType, extensionNumber, nil)
	} else {
		return d, nil
	}
}

func fileWithoutDeps(fd *descriptorpb.FileDescriptorProto, missingDeps []int) *descriptorpb.FileDescriptorProto {
	// We need to rebuild the file without the missing deps.
	fd = proto.Clone(fd).(*descriptorpb.FileDescriptorProto)
	newNumDeps := len(fd.GetDependency()) - len(missingDeps)
	newDeps := make([]string, 0, newNumDeps)
	remapped := make(map[int]int, newNumDeps)
	missingIdx := 0
	for i, dep := range fd.GetDependency() {
		if missingIdx < len(missingDeps) {
			if i == missingDeps[missingIdx] {
				// This dep was missing. Skip it.
				missingIdx++
				continue
			}
		}
		remapped[i] = len(newDeps)
		newDeps = append(newDeps, dep)
	}
	// Also rebuild public and weak import slices.
	newPublic := make([]int32, 0, len(fd.GetPublicDependency()))
	for _, idx := range fd.GetPublicDependency() {
		newIdx, ok := remapped[int(idx)]
		if ok {
			newPublic = append(newPublic, int32(newIdx))
		}
	}
	newWeak := make([]int32, 0, len(fd.GetWeakDependency()))
	for _, idx := range fd.GetWeakDependency() {
		newIdx, ok := remapped[int(idx)]
		if ok {
			newWeak = append(newWeak, int32(newIdx))
		}
	}

	fd.Dependency = newDeps
	fd.PublicDependency = newPublic
	fd.WeakDependency = newWeak
	return fd
}

func findExtension(extendedType string, extensionNumber int32, scope extensionScope) *desc.FieldDescriptor {
	// search extensions in this scope
	for _, ext := range scope.extensions() {
		if ext.GetNumber() == extensionNumber && ext.GetOwner().GetFullyQualifiedName() == extendedType {
			return ext
		}
	}

	// if not found, search nested scopes
	for _, nested := range scope.nestedScopes() {
		ext := findExtension(extendedType, extensionNumber, nested)
		if ext != nil {
			return ext
		}
	}

	return nil
}

type extensionScope interface {
	extensions() []*desc.FieldDescriptor
	nestedScopes() []extensionScope
}

// fileDescriptorExtensions implements extensionHolder interface on top of
// FileDescriptorProto
type fileDescriptorExtensions struct {
	proto *desc.FileDescriptor
}

func (fde fileDescriptorExtensions) extensions() []*desc.FieldDescriptor {
	return fde.proto.GetExtensions()
}

func (fde fileDescriptorExtensions) nestedScopes() []extensionScope {
	scopes := make([]extensionScope, len(fde.proto.GetMessageTypes()))
	for i, m := range fde.proto.GetMessageTypes() {
		scopes[i] = msgDescriptorExtensions{m}
	}
	return scopes
}

// msgDescriptorExtensions implements extensionHolder interface on top of
// DescriptorProto
type msgDescriptorExtensions struct {
	proto *desc.MessageDescriptor
}

func (mde msgDescriptorExtensions) extensions() []*desc.FieldDescriptor {
	return mde.proto.GetNestedExtensions()
}

func (mde msgDescriptorExtensions) nestedScopes() []extensionScope {
	scopes := make([]extensionScope, len(mde.proto.GetNestedMessageTypes()))
	for i, m := range mde.proto.GetNestedMessageTypes() {
		scopes[i] = msgDescriptorExtensions{m}
	}
	return scopes
}

type adaptStreamFromV1Alpha struct {
	refv1alpha.ServerReflection_ServerReflectionInfoClient
}

func (a adaptStreamFromV1Alpha) Send(request *refv1.ServerReflectionRequest) error {
	v1req := toV1AlphaRequest(request)
	return a.ServerReflection_ServerReflectionInfoClient.Send(v1req)
}

func (a adaptStreamFromV1Alpha) Recv() (*refv1.ServerReflectionResponse, error) {
	v1resp, err := a.ServerReflection_ServerReflectionInfoClient.Recv()
	if err != nil {
		return nil, err
	}
	return toV1Response(v1resp), nil
}

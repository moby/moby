package etw

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Provider represents an ETW event provider. It is identified by a provider
// name and ID (GUID), which should always have a 1:1 mapping to each other
// (e.g. don't use multiple provider names with the same ID, or vice versa).
type Provider struct {
	ID         *windows.GUID
	handle     providerHandle
	metadata   []byte
	callback   EnableCallback
	index      uint
	enabled    bool
	level      Level
	keywordAny uint64
	keywordAll uint64
}

// String returns the `provider`.ID as a string
func (provider *Provider) String() string {
	data1 := make([]byte, 4)
	binary.BigEndian.PutUint32(data1, provider.ID.Data1)
	data2 := make([]byte, 2)
	binary.BigEndian.PutUint16(data2, provider.ID.Data2)
	data3 := make([]byte, 2)
	binary.BigEndian.PutUint16(data3, provider.ID.Data3)
	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		hex.EncodeToString(data1),
		hex.EncodeToString(data2),
		hex.EncodeToString(data3),
		hex.EncodeToString(provider.ID.Data4[:2]),
		hex.EncodeToString(provider.ID.Data4[2:]))
}

type providerHandle windows.Handle

// ProviderState informs the provider EnableCallback what action is being
// performed.
type ProviderState uint32

const (
	// ProviderStateDisable indicates the provider is being disabled.
	ProviderStateDisable ProviderState = iota
	// ProviderStateEnable indicates the provider is being enabled.
	ProviderStateEnable
	// ProviderStateCaptureState indicates the provider is having its current
	// state snap-shotted.
	ProviderStateCaptureState
)

type eventInfoClass uint32

const (
	eventInfoClassProviderBinaryTrackInfo eventInfoClass = iota
	eventInfoClassProviderSetReserved1
	eventInfoClassProviderSetTraits
	eventInfoClassProviderUseDescriptorType
)

// EnableCallback is the form of the callback function that receives provider
// enable/disable notifications from ETW.
type EnableCallback func(*windows.GUID, ProviderState, Level, uint64, uint64, uintptr)

func providerCallback(sourceID *windows.GUID, state ProviderState, level Level, matchAnyKeyword uint64, matchAllKeyword uint64, filterData uintptr, i uintptr) {
	provider := providers.getProvider(uint(i))

	switch state {
	case ProviderStateDisable:
		provider.enabled = false
	case ProviderStateEnable:
		provider.enabled = true
		provider.level = level
		provider.keywordAny = matchAnyKeyword
		provider.keywordAll = matchAllKeyword
	}

	if provider.callback != nil {
		provider.callback(sourceID, state, level, matchAnyKeyword, matchAllKeyword, filterData)
	}
}

// providerCallbackAdapter acts as the first-level callback from the C/ETW side
// for provider notifications. Because Go has trouble with callback arguments of
// different size, it has only pointer-sized arguments, which are then cast to
// the appropriate types when calling providerCallback.
func providerCallbackAdapter(sourceID *windows.GUID, state uintptr, level uintptr, matchAnyKeyword uintptr, matchAllKeyword uintptr, filterData uintptr, i uintptr) uintptr {
	providerCallback(sourceID, ProviderState(state), Level(level), uint64(matchAnyKeyword), uint64(matchAllKeyword), filterData, i)
	return 0
}

// providerIDFromName generates a provider ID based on the provider name. It
// uses the same algorithm as used by .NET's EventSource class, which is based
// on RFC 4122. More information on the algorithm can be found here:
// https://blogs.msdn.microsoft.com/dcook/2015/09/08/etw-provider-names-and-guids/
// The algorithm is roughly:
// Hash = Sha1(namespace + arg.ToUpper().ToUtf16be())
// Guid = Hash[0..15], with Hash[7] tweaked according to RFC 4122
func providerIDFromName(name string) *windows.GUID {
	buffer := sha1.New()

	namespace := []byte{0x48, 0x2C, 0x2D, 0xB2, 0xC3, 0x90, 0x47, 0xC8, 0x87, 0xF8, 0x1A, 0x15, 0xBF, 0xC1, 0x30, 0xFB}
	buffer.Write(namespace)

	binary.Write(buffer, binary.BigEndian, utf16.Encode([]rune(strings.ToUpper(name))))

	sum := buffer.Sum(nil)
	sum[7] = (sum[7] & 0xf) | 0x50

	return &windows.GUID{
		Data1: binary.LittleEndian.Uint32(sum[0:4]),
		Data2: binary.LittleEndian.Uint16(sum[4:6]),
		Data3: binary.LittleEndian.Uint16(sum[6:8]),
		Data4: [8]byte{sum[8], sum[9], sum[10], sum[11], sum[12], sum[13], sum[14], sum[15]},
	}
}

// NewProvider creates and registers a new ETW provider. The provider ID is
// generated based on the provider name.
func NewProvider(name string, callback EnableCallback) (provider *Provider, err error) {
	return NewProviderWithID(name, providerIDFromName(name), callback)
}

// NewProviderWithID creates and registers a new ETW provider, allowing the
// provider ID to be manually specified. This is most useful when there is an
// existing provider ID that must be used to conform to existing diagnostic
// infrastructure.
func NewProviderWithID(name string, id *windows.GUID, callback EnableCallback) (provider *Provider, err error) {
	providerCallbackOnce.Do(func() {
		globalProviderCallback = windows.NewCallback(providerCallbackAdapter)
	})

	provider = providers.newProvider()
	defer func() {
		if err != nil {
			providers.removeProvider(provider)
		}
	}()
	provider.ID = id
	provider.callback = callback

	if err := eventRegister(provider.ID, globalProviderCallback, uintptr(provider.index), &provider.handle); err != nil {
		return nil, err
	}

	metadata := &bytes.Buffer{}
	binary.Write(metadata, binary.LittleEndian, uint16(0)) // Write empty size for buffer (to update later)
	metadata.WriteString(name)
	metadata.WriteByte(0)                                                   // Null terminator for name
	binary.LittleEndian.PutUint16(metadata.Bytes(), uint16(metadata.Len())) // Update the size at the beginning of the buffer
	provider.metadata = metadata.Bytes()

	if err := eventSetInformation(
		provider.handle,
		eventInfoClassProviderSetTraits,
		uintptr(unsafe.Pointer(&provider.metadata[0])),
		uint32(len(provider.metadata))); err != nil {

		return nil, err
	}

	return provider, nil
}

// Close unregisters the provider.
func (provider *Provider) Close() error {
	providers.removeProvider(provider)
	return eventUnregister(provider.handle)
}

// IsEnabled calls IsEnabledForLevelAndKeywords with LevelAlways and all
// keywords set.
func (provider *Provider) IsEnabled() bool {
	return provider.IsEnabledForLevelAndKeywords(LevelAlways, ^uint64(0))
}

// IsEnabledForLevel calls IsEnabledForLevelAndKeywords with the specified level
// and all keywords set.
func (provider *Provider) IsEnabledForLevel(level Level) bool {
	return provider.IsEnabledForLevelAndKeywords(level, ^uint64(0))
}

// IsEnabledForLevelAndKeywords allows event producer code to check if there are
// any event sessions that are interested in an event, based on the event level
// and keywords. Although this check happens automatically in the ETW
// infrastructure, it can be useful to check if an event will actually be
// consumed before doing expensive work to build the event data.
func (provider *Provider) IsEnabledForLevelAndKeywords(level Level, keywords uint64) bool {
	if !provider.enabled {
		return false
	}

	// ETW automatically sets the level to 255 if it is specified as 0, so we
	// don't need to worry about the level=0 (all events) case.
	if level > provider.level {
		return false
	}

	if keywords != 0 && (keywords&provider.keywordAny == 0 || keywords&provider.keywordAll != provider.keywordAll) {
		return false
	}

	return true
}

// WriteEvent writes a single ETW event from the provider. The event is
// constructed based on the EventOpt and FieldOpt values that are passed as
// opts.
func (provider *Provider) WriteEvent(name string, eventOpts []EventOpt, fieldOpts []FieldOpt) error {
	options := eventOptions{descriptor: NewEventDescriptor()}
	em := &EventMetadata{}
	ed := &EventData{}

	// We need to evaluate the EventOpts first since they might change tags, and
	// we write out the tags before evaluating FieldOpts.
	for _, opt := range eventOpts {
		opt(&options)
	}

	if !provider.IsEnabledForLevelAndKeywords(options.descriptor.Level, options.descriptor.Keyword) {
		return nil
	}

	em.WriteEventHeader(name, options.tags)

	for _, opt := range fieldOpts {
		opt(em, ed)
	}

	// Don't pass a data blob if there is no event data. There will always be
	// event metadata (e.g. for the name) so we don't need to do this check for
	// the metadata.
	dataBlobs := [][]byte{}
	if len(ed.Bytes()) > 0 {
		dataBlobs = [][]byte{ed.Bytes()}
	}

	return provider.WriteEventRaw(options.descriptor, nil, nil, [][]byte{em.Bytes()}, dataBlobs)
}

// WriteEventRaw writes a single ETW event from the provider. This function is
// less abstracted than WriteEvent, and presents a fairly direct interface to
// the event writing functionality. It expects a series of event metadata and
// event data blobs to be passed in, which must conform to the TraceLogging
// schema. The functions on EventMetadata and EventData can help with creating
// these blobs. The blobs of each type are effectively concatenated together by
// the ETW infrastructure.
func (provider *Provider) WriteEventRaw(
	descriptor *EventDescriptor,
	activityID *windows.GUID,
	relatedActivityID *windows.GUID,
	metadataBlobs [][]byte,
	dataBlobs [][]byte) error {

	dataDescriptorCount := uint32(1 + len(metadataBlobs) + len(dataBlobs))
	dataDescriptors := make([]eventDataDescriptor, 0, dataDescriptorCount)

	dataDescriptors = append(dataDescriptors, newEventDataDescriptor(eventDataDescriptorTypeProviderMetadata, provider.metadata))
	for _, blob := range metadataBlobs {
		dataDescriptors = append(dataDescriptors, newEventDataDescriptor(eventDataDescriptorTypeEventMetadata, blob))
	}
	for _, blob := range dataBlobs {
		dataDescriptors = append(dataDescriptors, newEventDataDescriptor(eventDataDescriptorTypeUserData, blob))
	}

	return eventWriteTransfer(provider.handle, descriptor, activityID, relatedActivityID, dataDescriptorCount, &dataDescriptors[0])
}

//go:build windows
// +build windows

// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package certtostore

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/google/deck"
	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"
	"golang.org/x/sys/windows"
)

// WinCertStorage provides windows-specific additions to the CertStorage interface.
type WinCertStorage interface {
	CertStorage

	// Remove removes certificates issued by any of w.issuers from the user and/or system cert stores.
	// If it is unable to remove any certificates, it returns an error.
	Remove(removeSystem bool) error

	// Link will associate the certificate installed in the system store to the user store.
	Link() error

	// Close frees the handle to the certificate provider, the certificate store, etc.
	Close() error

	// CertWithContext performs a certificate lookup using value of issuers that
	// was provided when WinCertStore was created. It returns both the certificate
	// and its Windows context, which can be used to perform other operations,
	// such as looking up the private key with CertKey().
	//
	// You must call FreeCertContext on the context after use.
	CertWithContext() (*x509.Certificate, *windows.CertContext, error)

	// CertKey wraps CryptAcquireCertificatePrivateKey. It obtains the CNG private
	// key of a known certificate and returns a pointer to a Key which implements
	// both crypto.Signer and crypto.Decrypter. When a nil cert context is passed
	// a nil key is intentionally returned, to model the expected behavior of a
	// non-existent cert having no private key.
	// https://docs.microsoft.com/en-us/windows/win32/api/wincrypt/nf-wincrypt-cryptacquirecertificateprivatekey
	CertKey(cert *windows.CertContext) (*Key, error)

	// StoreWithDisposition imports certificates into the Windows certificate store.
	// disposition specifies the action to take if a matching certificate
	// or a link to a matching certificate already exists in the store
	// https://learn.microsoft.com/en-us/windows/win32/api/wincrypt/nf-wincrypt-certaddcertificatecontexttostore
	StoreWithDisposition(cert *x509.Certificate, intermediate *x509.Certificate, disposition uint32) error
}

const (
	// wincrypt.h constants
	acquireCached           = 0x1                                             // CRYPT_ACQUIRE_CACHE_FLAG
	acquireSilent           = 0x40                                            // CRYPT_ACQUIRE_SILENT_FLAG
	acquireOnlyNCryptKey    = 0x40000                                         // CRYPT_ACQUIRE_ONLY_NCRYPT_KEY_FLAG
	encodingX509ASN         = 1                                               // X509_ASN_ENCODING
	encodingPKCS7           = 65536                                           // PKCS_7_ASN_ENCODING
	certStoreProvSystem     = 10                                              // CERT_STORE_PROV_SYSTEM
	certStoreCurrentUser    = uint32(certStoreCurrentUserID << compareShift)  // CERT_SYSTEM_STORE_CURRENT_USER
	certStoreLocalMachine   = uint32(certStoreLocalMachineID << compareShift) // CERT_SYSTEM_STORE_LOCAL_MACHINE
	certStoreCurrentUserID  = 1                                               // CERT_SYSTEM_STORE_CURRENT_USER_ID
	certStoreLocalMachineID = 2                                               // CERT_SYSTEM_STORE_LOCAL_MACHINE_ID
	infoIssuerFlag          = 4                                               // CERT_INFO_ISSUER_FLAG
	compareNameStrW         = 8                                               // CERT_COMPARE_NAME_STR_A
	compareShift            = 16                                              // CERT_COMPARE_SHIFT
	findIssuerStr           = compareNameStrW<<compareShift | infoIssuerFlag  // CERT_FIND_ISSUER_STR_W
	signatureKeyUsage       = 0x80                                            // CERT_DIGITAL_SIGNATURE_KEY_USAGE

	// Legacy CryptoAPI flags
	bCryptPadPKCS1 uintptr = 0x2
	bCryptPadPSS   uintptr = 0x8

	// Magic numbers for public key blobs.
	rsa1Magic      = 0x31415352 // "RSA1" BCRYPT_RSAPUBLIC_MAGIC
	ecdsaP256Magic = 0x31534345 // BCRYPT_ECDSA_PUBLIC_P256_MAGIC
	ecdsaP384Magic = 0x33534345 // BCRYPT_ECDSA_PUBLIC_P384_MAGIC
	ecdsaP521Magic = 0x35534345 // BCRYPT_ECDSA_PUBLIC_P521_MAGIC
	ecdhP256Magic  = 0x314B4345 // BCRYPT_ECDH_PUBLIC_P256_MAGIC
	ecdhP384Magic  = 0x334B4345 // BCRYPT_ECDH_PUBLIC_P384_MAGIC
	ecdhP521Magic  = 0x354B4345 // BCRYPT_ECDH_PUBLIC_P521_MAGIC

	// ncrypt.h constants
	ncryptPersistFlag           = 0x80000000 // NCRYPT_PERSIST_FLAG
	ncryptAllowDecryptFlag      = 0x1        // NCRYPT_ALLOW_DECRYPT_FLAG
	ncryptAllowSigningFlag      = 0x2        // NCRYPT_ALLOW_SIGNING_FLAG
	ncryptWriteKeyToLegacyStore = 0x00000200 // NCRYPT_WRITE_KEY_TO_LEGACY_STORE_FLAG

	// NCryptPadOAEPFlag is used with Decrypt to specify whether to use OAEP.
	NCryptPadOAEPFlag = 0x00000004 // NCRYPT_PAD_OAEP_FLAG

	// key creation flags.
	nCryptMachineKey   = 0x20 // NCRYPT_MACHINE_KEY_FLAG
	nCryptOverwriteKey = 0x80 // NCRYPT_OVERWRITE_KEY_FLAG

	// winerror.h constants
	cryptENotFound syscall.Errno = 0x80092004 // CRYPT_E_NOT_FOUND

	// ProviderMSPlatform represents the Microsoft Platform Crypto Provider
	ProviderMSPlatform = "Microsoft Platform Crypto Provider"
	// ProviderMSSoftware represents the Microsoft Software Key Storage Provider
	ProviderMSSoftware = "Microsoft Software Key Storage Provider"
	// ProviderMSLegacy represents the CryptoAPI compatible Enhanced Cryptographic Provider
	ProviderMSLegacy = "Microsoft Enhanced Cryptographic Provider v1.0"

	// Chain resolution constants
	hcceLocalMachine                  = windows.Handle(0x01) // HCCE_LOCAL_MACHINE
	certChainCacheOnlyURLRetrieval    = 0x00000004           // CERT_CHAIN_CACHE_ONLY_URL_RETRIEVAL
	certChainDisableAIA               = 0x00002000           // CERT_CHAIN_DISABLE_AIA
	certChainRevocationCheckCacheOnly = 0x80000000           // CERT_CHAIN_REVOCATION_CHECK_CACHE_ONLY

	// CertStoreReadOnly represents read only permissions
	CertStoreReadOnly = 0x00008000 // CERT_STORE_READONLY_FLAG
	// CertStoreSaveToFile represents write to file permissions
	CertStoreSaveToFile = 0x00001000 // CERT_STORE_SAVE_TO_FILE
	// CertStoreOpenMaximumAllowed represents all permissions
	CertStoreOpenMaximumAllowed = 0x00001000 // CERT_STORE_MAXIMUM_ALLOWED_FLAG
)

const ncryptKeySpec uint32 = 0xFFFFFFFF // CERT_NCRYPT_KEY_SPEC
var (
	// Key blob type constants.
	bCryptRSAPublicBlob = wide("RSAPUBLICBLOB")
	bCryptECCPublicBlob = wide("ECCPUBLICBLOB")

	// Key storage properties
	nCryptAlgorithmGroupProperty = wide("Algorithm Group") // NCRYPT_ALGORITHM_GROUP_PROPERTY
	nCryptUniqueNameProperty     = wide("Unique Name")     // NCRYPT_UNIQUE_NAME_PROPERTY
	nCryptECCCurveNameProperty   = wide("ECCCurveName")    // NCRYPT_ECC_CURVE_NAME_PROPERTY
	nCryptImplTypeProperty       = wide("Impl Type")       // NCRYPT_IMPL_TYPE_PROPERTY
	nCryptProviderHandleProperty = wide("Provider Handle") // NCRYPT_PROV_HANDLE

	// Flags for NCRYPT_IMPL_TYPE_PROPERTY
	nCryptImplHardwareFlag    uint32 = 0x00000001 // NCRYPT_IMPL_HARDWARE_FLAG
	nCryptImplSoftwareFlag    uint32 = 0x00000002 // NCRYPT_IMPL_SOFTWARE_FLAG
	nCryptImplRemovableFlag   uint32 = 0x00000008 // NCRYPT_IMPL_REMOVABLE_FLAG
	nCryptImplHardwareRngFlag uint32 = 0x00000010 // NCRYPT_IMPL_HARDWARE_RNG_FLAG

	// curveIDs maps bcrypt key blob magic numbers to elliptic curves.
	curveIDs = map[uint32]elliptic.Curve{
		ecdsaP256Magic: elliptic.P256(), // BCRYPT_ECDSA_PUBLIC_P256_MAGIC
		ecdsaP384Magic: elliptic.P384(), // BCRYPT_ECDSA_PUBLIC_P384_MAGIC
		ecdsaP521Magic: elliptic.P521(), // BCRYPT_ECDSA_PUBLIC_P521_MAGIC
		ecdhP256Magic:  elliptic.P256(), // BCRYPT_ECDH_PUBLIC_P256_MAGIC
		ecdhP384Magic:  elliptic.P384(), // BCRYPT_ECDH_PUBLIC_P384_MAGIC
		ecdhP521Magic:  elliptic.P521(), // BCRYPT_ECDH_PUBLIC_P521_MAGIC
	}

	// curveNames maps bcrypt curve names to elliptic curves. We use it
	// for a fallback mechanism when magic is incorrect (see b/185945636).
	curveNames = map[string]elliptic.Curve{
		"nistP256": elliptic.P256(), // BCRYPT_ECC_CURVE_NISTP256
		"nistP384": elliptic.P384(), // BCRYPT_ECC_CURVE_NISTP384
		"nistP521": elliptic.P521(), // BCRYPT_ECC_CURVE_NISTP521
	}

	// algIDs maps crypto.Hash values to bcrypt.h constants.
	algIDs = map[crypto.Hash]*uint16{
		crypto.SHA1:   wide("SHA1"),   // BCRYPT_SHA1_ALGORITHM
		crypto.SHA256: wide("SHA256"), // BCRYPT_SHA256_ALGORITHM
		crypto.SHA384: wide("SHA384"), // BCRYPT_SHA384_ALGORITHM
		crypto.SHA512: wide("SHA512"), // BCRYPT_SHA512_ALGORITHM
	}

	// MY, CA and ROOT are well-known system stores that holds certificates.
	// The store that is opened (system or user) depends on the system call used.
	// see https://msdn.microsoft.com/en-us/library/windows/desktop/aa376560(v=vs.85).aspx)
	my   = wide("MY")
	ca   = wide("CA")
	root = wide("ROOT")

	crypt32 = windows.MustLoadDLL("crypt32.dll")
	nCrypt  = windows.MustLoadDLL("ncrypt.dll")

	certDeleteCertificateFromStore    = crypt32.MustFindProc("CertDeleteCertificateFromStore")
	certFindCertificateInStore        = crypt32.MustFindProc("CertFindCertificateInStore")
	certFreeCertificateChain          = crypt32.MustFindProc("CertFreeCertificateChain")
	certGetCertificateChain           = crypt32.MustFindProc("CertGetCertificateChain")
	certGetIntendedKeyUsage           = crypt32.MustFindProc("CertGetIntendedKeyUsage")
	cryptAcquireCertificatePrivateKey = crypt32.MustFindProc("CryptAcquireCertificatePrivateKey")
	cryptFindCertificateKeyProvInfo   = crypt32.MustFindProc("CryptFindCertificateKeyProvInfo")
	nCryptCreatePersistedKey          = nCrypt.MustFindProc("NCryptCreatePersistedKey")
	nCryptDecrypt                     = nCrypt.MustFindProc("NCryptDecrypt")
	nCryptExportKey                   = nCrypt.MustFindProc("NCryptExportKey")
	nCryptFinalizeKey                 = nCrypt.MustFindProc("NCryptFinalizeKey")
	nCryptFreeObject                  = nCrypt.MustFindProc("NCryptFreeObject")
	nCryptOpenKey                     = nCrypt.MustFindProc("NCryptOpenKey")
	nCryptOpenStorageProvider         = nCrypt.MustFindProc("NCryptOpenStorageProvider")
	nCryptGetProperty                 = nCrypt.MustFindProc("NCryptGetProperty")
	nCryptSetProperty                 = nCrypt.MustFindProc("NCryptSetProperty")
	nCryptSignHash                    = nCrypt.MustFindProc("NCryptSignHash")

	// Path to icacls binary
	icaclsPath = filepath.Join(os.Getenv("SystemRoot"), "System32", "icacls.exe")

	// Test helpers
	fnGetProperty = getProperty
)

// pkcs1PaddingInfo is the BCRYPT_PKCS1_PADDING_INFO struct in bcrypt.h.
type pkcs1PaddingInfo struct {
	pszAlgID *uint16
}

// pssPaddingInfo is the BCRYPT_PSS_PADDING_INFO struct in bcrypt.h.
type pssPaddingInfo struct {
	pszAlgID *uint16
	cbSalt   uint32
}

// wide returns a pointer to a a uint16 representing the equivalent
// to a Windows LPCWSTR.
func wide(s string) *uint16 {
	w := utf16.Encode([]rune(s))
	w = append(w, 0)
	return &w[0]
}

func openProvider(provider string) (uintptr, error) {
	var hProv uintptr
	pname := wide(provider)
	// Open the provider, the last parameter is not used
	r, _, err := nCryptOpenStorageProvider.Call(uintptr(unsafe.Pointer(&hProv)), uintptr(unsafe.Pointer(pname)), 0)
	if r == 0 {
		return hProv, nil
	}
	return hProv, fmt.Errorf("NCryptOpenStorageProvider returned %X: %v", r, err)
}

// findCert wraps the CertFindCertificateInStore call. Note that any cert context passed
// into prev will be freed. If no certificate was found, nil will be returned.
func findCert(store windows.Handle, enc, findFlags, findType uint32, para *uint16, prev *windows.CertContext) (*windows.CertContext, error) {
	h, _, err := certFindCertificateInStore.Call(
		uintptr(store),
		uintptr(enc),
		uintptr(findFlags),
		uintptr(findType),
		uintptr(unsafe.Pointer(para)),
		uintptr(unsafe.Pointer(prev)),
	)
	if h == 0 {
		// Actual error, or simply not found?
		if errno, ok := err.(syscall.Errno); ok && errno == cryptENotFound {
			return nil, nil
		}
		return nil, err
	}
	return (*windows.CertContext)(unsafe.Pointer(h)), nil
}

// FreeCertContext frees a certificate context after use.
func FreeCertContext(ctx *windows.CertContext) error {
	return windows.CertFreeCertificateContext(ctx)
}

// intendedKeyUsage wraps CertGetIntendedKeyUsage. If there are key usage bytes they will be returned,
// otherwise 0 will be returned. The final parameter (2) represents the size in bytes of &usage.
func intendedKeyUsage(enc uint32, cert *windows.CertContext) (usage uint16) {
	certGetIntendedKeyUsage.Call(uintptr(enc), uintptr(unsafe.Pointer(cert.CertInfo)), uintptr(unsafe.Pointer(&usage)), 2)
	return
}

// WinCertStoreOptions contains configuration options for opening a certificate store.
// This struct provides comprehensive control over how a Windows certificate store
// is opened and configured, including cryptographic providers, key storage options,
// and store access flags.
type WinCertStoreOptions struct {
	// Provider specifies the cryptographic provider to use for key operations.
	// Common values include:
	//   - ProviderMSPlatform: "Microsoft Platform Crypto Provider"
	//   - ProviderMSSoftware: "Microsoft Software Key Storage Provider"
	//   - ProviderMSLegacy: "Microsoft Enhanced Cryptographic Provider v1.0"
	Provider string

	// Container specifies the key container name within the cryptographic provider.
	// This name uniquely identifies the key pair within the provider.
	Container string

	// Issuers contains the list of certificate issuer distinguished names to search for.
	// The certificate lookup will match against these issuer names.
	Issuers []string

	// IntermediateIssuers contains the list of intermediate certificate issuer distinguished names.
	// These are used for certificate chain validation and storage.
	IntermediateIssuers []string

	// LegacyKey indicates whether to use a legacy key format compatible with CryptoAPI.
	// When true, keys will be stored in a format accessible to older Windows applications.
	LegacyKey bool

	// CurrentUser indicates whether to use the current user's certificate store instead
	// of the local machine store. When false, the local machine store is used, which
	// requires administrator privileges but makes certificates available to all users.
	CurrentUser bool

	// StoreFlags contains additional flags for certificate store operations.
	// These flags control how the certificate store is opened and accessed.
	// Common flags include:
	//   - certStoreReadOnly: Open store in read-only mode
	//   - certStoreSaveToFile: Enable saving store to file
	//   - certStoreCreateNewFlag: Create new store if it doesn't exist
	//   - certStoreOpenExistingFlag: Only open existing stores
	StoreFlags uint32
}

// WinCertStore is a CertStorage implementation for the Windows Certificate Store.
type WinCertStore struct {
	Prov                uintptr
	ProvName            string
	issuers             []string
	intermediateIssuers []string
	container           string
	keyStorageFlags     uintptr
	certChains          [][]*x509.Certificate
	stores              map[string]*storeHandle
	keyAccessFlags      uintptr
	storeFlags          uint32

	mu sync.Mutex
}

// DefaultWinCertStoreOptions returns the default options for opening a certificate store.
// These options represent a safe, commonly-used configuration suitable for most applications.
//
// Parameters:
//   - provider: The cryptographic provider name (e.g., ProviderMSSoftware)
//   - container: The key container name
//   - issuers: List of certificate issuer distinguished names
//   - intermediateIssuers: List of intermediate certificate issuer distinguished names
//   - legacyKey: Whether to use legacy CryptoAPI-compatible key format
//
// Returns a WinCertStoreOptions struct with safe defaults:
//   - CurrentUser: false (uses machine store)
//   - StoreFlags: 0 (no special flags)
func DefaultWinCertStoreOptions(provider, container string, issuers, intermediateIssuers []string, legacyKey bool) WinCertStoreOptions {
	return WinCertStoreOptions{
		Provider:            provider,
		Container:           container,
		Issuers:             issuers,
		IntermediateIssuers: intermediateIssuers,
		LegacyKey:           legacyKey,
		CurrentUser:         false,
		StoreFlags:          0,
	}
}

// OpenWinCertStore creates a WinCertStore with keys accessible by all users on a machine.
// Call Close() when finished using the store.
func OpenWinCertStore(provider, container string, issuers, intermediateIssuers []string, legacyKey bool) (*WinCertStore, error) {
	opts := DefaultWinCertStoreOptions(provider, container, issuers, intermediateIssuers, legacyKey)
	return OpenWinCertStoreWithOptions(opts)
}

// OpenWinCertStoreCurrentUser creates a WinCertStore with keys accessible by current user.
// Call Close() when finished using the store.
func OpenWinCertStoreCurrentUser(provider, container string, issuers, intermediateIssuers []string, legacyKey bool) (*WinCertStore, error) {
	opts := DefaultWinCertStoreOptions(provider, container, issuers, intermediateIssuers, legacyKey)
	opts.CurrentUser = true
	return OpenWinCertStoreWithOptions(opts)
}

// OpenWinCertStoreWithOptions creates a WinCertStore with the provided options.
// This function provides maximum flexibility for configuring the certificate store,
// including advanced options like custom store flags and provider selection.
//
// The function validates all options before attempting to open the store, returning
// detailed error information if any configuration is invalid or incompatible.
//
// Parameters:
//   - opts: Comprehensive configuration options for the certificate store
//
// Returns:
//   - *WinCertStore: A configured certificate store ready for use
//   - error: Detailed error information if store creation fails
//
// Example usage:
//
//	opts := WinCertStoreOptions{
//	    Provider: ProviderMSSoftware,
//	    Container: "MY",
//	    Issuers: []string{"CN=My CA"},
//	    StoreFlags: certStoreReadOnly,
//	    CurrentUser: true,
//	}
//	store, err := OpenWinCertStoreWithOptions(opts)
//	if err != nil {
//	    return fmt.Errorf("failed to open certificate store: %v", err)
//	}
//	defer store.Close()
//
// Common errors:
//   - Provider not available or accessible
//   - Invalid or incompatible store flags
//   - Missing required options (provider, container, issuers)
//   - Insufficient privileges for machine store access
func OpenWinCertStoreWithOptions(opts WinCertStoreOptions) (*WinCertStore, error) {
	// Open a handle to the crypto provider we will use for private key operations
	cngProv, err := openProvider(opts.Provider)
	if err != nil {
		return nil, fmt.Errorf("unable to open crypto provider %q: %v", opts.Provider, err)
	}

	wcs := &WinCertStore{
		Prov:                cngProv,
		ProvName:            opts.Provider,
		issuers:             make([]string, len(opts.Issuers)),
		intermediateIssuers: make([]string, len(opts.IntermediateIssuers)),
		container:           opts.Container,
		stores:              make(map[string]*storeHandle),
		storeFlags:          opts.StoreFlags,
	}

	// Deep copy the issuer slices to prevent external modification
	copy(wcs.issuers, opts.Issuers)
	copy(wcs.intermediateIssuers, opts.IntermediateIssuers)

	if opts.LegacyKey {
		wcs.keyStorageFlags = ncryptWriteKeyToLegacyStore
		wcs.ProvName = ProviderMSLegacy
	}

	if !opts.CurrentUser {
		wcs.keyAccessFlags = nCryptMachineKey
	}

	return wcs, nil
}

// certContextToX509 creates an x509.Certificate from a Windows cert context.
func certContextToX509(ctx *windows.CertContext) (*x509.Certificate, error) {
	var der []byte
	slice := (*reflect.SliceHeader)(unsafe.Pointer(&der))
	slice.Data = uintptr(unsafe.Pointer(ctx.EncodedCert))
	slice.Len = int(ctx.Length)
	slice.Cap = int(ctx.Length)
	return x509.ParseCertificate(append([]byte{}, der...))
}

// extractSimpleChain extracts the requested certificate chain from a CertSimpleChain.
// Adapted from crypto.x509.root_windows
func extractSimpleChain(simpleChain **windows.CertSimpleChain, chainCount, chainIndex int) ([]*x509.Certificate, error) {
	if simpleChain == nil || chainCount == 0 || chainIndex >= chainCount {
		return nil, errors.New("invalid simple chain")
	}
	// Convert the simpleChain array to a huge slice and slice it to the length we want.
	// https://github.com/golang/go/wiki/cgo#turning-c-arrays-into-go-slices
	simpleChains := (*[1 << 20]*windows.CertSimpleChain)(unsafe.Pointer(simpleChain))[:chainCount:chainCount]
	// Each simple chain contains the chain of certificates, summary trust information
	// about the chain, and trust information about each certificate element in the chain.
	lastChain := simpleChains[chainIndex]
	chainLen := int(lastChain.NumElements)
	elements := (*[1 << 20]*windows.CertChainElement)(unsafe.Pointer(lastChain.Elements))[:chainLen:chainLen]
	chain := make([]*x509.Certificate, 0, chainLen)
	for _, element := range elements {
		xc, err := certContextToX509(element.CertContext)
		if err != nil {
			return nil, err
		}
		chain = append(chain, xc)
	}
	return chain, nil
}

func (w *WinCertStore) storeDomain() uint32 {
	if w.keyAccessFlags == nCryptMachineKey {
		return certStoreLocalMachine
	}
	return certStoreCurrentUser
}

// resolveCertChains builds chains to roots from a given certificate using the local machine store.
func (w *WinCertStore) resolveChains(cert *windows.CertContext) error {
	var (
		chainPara windows.CertChainPara
		chainCtx  *windows.CertChainContext
	)

	// Search the system for candidate certificate chains.
	chainPara.Size = uint32(unsafe.Sizeof(chainPara))
	success, _, err := certGetCertificateChain.Call(
		uintptr(unsafe.Pointer(hcceLocalMachine)),
		uintptr(unsafe.Pointer(cert)),
		uintptr(unsafe.Pointer(nil)), // Use current system time as validation time.
		uintptr(cert.Store),
		uintptr(unsafe.Pointer(&chainPara)),
		certChainRevocationCheckCacheOnly|certChainCacheOnlyURLRetrieval|certChainDisableAIA,
		uintptr(unsafe.Pointer(nil)), // Reserved.
		uintptr(unsafe.Pointer(&chainCtx)),
	)
	if success == 0 {
		return fmt.Errorf("certGetCertificateChain: %v", err)
	}
	defer certFreeCertificateChain.Call(uintptr(unsafe.Pointer(chainCtx)))

	chainCount := int(chainCtx.ChainCount)
	certChains := make([][]*x509.Certificate, 0, chainCount)
	for i := 0; i < chainCount; i++ {
		x509Certs, err := extractSimpleChain(chainCtx.Chains, chainCount, i)
		if err != nil {
			return fmt.Errorf("extractSimpleChain: %v", err)
		}
		certChains = append(certChains, x509Certs)
	}
	w.certChains = certChains

	return nil
}

// Cert returns the current cert associated with this WinCertStore or nil if there isn't one.
func (w *WinCertStore) Cert() (*x509.Certificate, error) {
	c, ctx, err := w.CertWithContext()
	if err != nil {
		return nil, err
	}
	FreeCertContext(ctx)
	return c, nil
}

// CertWithContext performs a certificate lookup using value of issuers that
// was provided when WinCertStore was created. It returns both the certificate
// and its Windows context, which can be used to perform other operations,
// such as looking up the private key with CertKey().
//
// You must call FreeCertContext on the context after use.
func (w *WinCertStore) CertWithContext() (*x509.Certificate, *windows.CertContext, error) {
	c, ctx, err := w.cert(w.issuers, my, w.storeDomain())
	if err != nil {
		return nil, nil, err
	}
	// If no cert was returned, skip resolving chains and return.
	if c == nil {
		return nil, nil, nil
	}
	if err := w.resolveChains(ctx); err != nil {
		return nil, nil, err
	}
	return c, ctx, nil
}

// cert is a helper function to lookup certificates based on a known issuer.
// store is used to specify which store to perform the lookup in (system or user).
func (w *WinCertStore) cert(issuers []string, searchRoot *uint16, store uint32) (*x509.Certificate, *windows.CertContext, error) {
	h, err := w.storeHandle(store, searchRoot)
	if err != nil {
		return nil, nil, err
	}

	var prev *windows.CertContext
	var cert *x509.Certificate
	for _, issuer := range issuers {
		i, err := windows.UTF16PtrFromString(issuer)
		if err != nil {
			return nil, nil, err
		}

		// pass 0 as the third parameter because it is not used
		// https://msdn.microsoft.com/en-us/library/windows/desktop/aa376064(v=vs.85).aspx
		nc, err := findCert(h, encodingX509ASN|encodingPKCS7, 0, findIssuerStr, i, prev)
		if err != nil {
			return nil, nil, fmt.Errorf("finding certificates: %v", err)
		}
		if nc == nil {
			// No certificate found
			continue
		}
		prev = nc
		if (intendedKeyUsage(encodingX509ASN, nc) & signatureKeyUsage) == 0 {
			continue
		}

		// Extract the DER-encoded certificate from the cert context.
		xc, err := certContextToX509(nc)
		if err != nil {
			continue
		}

		cert = xc
		break
	}
	if cert == nil {
		return nil, nil, nil
	}
	return cert, prev, nil
}

func freeObject(h uintptr) error {
	r, _, err := nCryptFreeObject.Call(h)
	if r == 0 {
		return nil
	}
	return fmt.Errorf("NCryptFreeObject returned %X: %v", r, err)
}

// Close frees the handle to the certificate provider, the certificate store, etc.
func (w *WinCertStore) Close() error {
	var result error
	for _, v := range w.stores {
		if v != nil {
			if err := v.Close(); err != nil {
				errors.Join(result, err)
			}
		}
	}
	if err := freeObject(w.Prov); err != nil {
		errors.Join(result, err)
	}
	w.certChains = nil
	return result
}

// Link will associate the certificate installed in the system store to the user store.
func (w *WinCertStore) Link() error {
	if w.isReadOnly() {
		return fmt.Errorf("cannot link certificates in a read-only store")
	}
	cert, _, err := w.cert(w.issuers, my, certStoreLocalMachine)
	if err != nil {
		return fmt.Errorf("checking for existing machine certificates returned: %v", err)
	}

	if cert == nil {
		return nil
	}

	// If the user cert is already there and matches the system cert, return early.
	userCert, _, err := w.cert(w.issuers, my, certStoreCurrentUser)
	if err != nil {
		return fmt.Errorf("checking for existing user certificates returned: %v", err)
	}
	if userCert != nil {
		if cert.SerialNumber.Cmp(userCert.SerialNumber) == 0 {
			fmt.Fprintf(os.Stdout, "Certificate %s is already linked to the user certificate store.\n", cert.SerialNumber)
			return nil
		}
	}

	// The user context is missing the cert, or it doesn't match, so proceed with the link.
	certContext, err := windows.CertCreateCertificateContext(
		encodingX509ASN|encodingPKCS7,
		&cert.Raw[0],
		uint32(len(cert.Raw)))
	if err != nil {
		return fmt.Errorf("CertCreateCertificateContext returned: %v", err)
	}
	defer windows.CertFreeCertificateContext(certContext)

	// Associate the private key we previously generated
	r, _, err := cryptFindCertificateKeyProvInfo.Call(
		uintptr(unsafe.Pointer(certContext)),
		uintptr(uint32(0)),
		0,
	)
	// Windows calls will fill err with a success message, r is what must be checked instead
	if r == 0 {
		fmt.Printf("found a matching private key for the certificate, but association failed: %v", err)
	}

	h, err := w.storeHandle(certStoreCurrentUser, my)
	if err != nil {
		return err
	}

	// Add the cert context to the users certificate store
	if err := windows.CertAddCertificateContextToStore(h, certContext, windows.CERT_STORE_ADD_ALWAYS, nil); err != nil {
		return fmt.Errorf("CertAddCertificateContextToStore returned: %v", err)
	}

	deck.Infof("Successfully linked to existing system certificate with serial %s.", cert.SerialNumber)
	fmt.Fprintf(os.Stdout, "Successfully linked to existing system certificate with serial %s.\n", cert.SerialNumber)

	// Link legacy crypto only if requested.
	if w.ProvName == ProviderMSLegacy {
		return w.linkLegacy()
	}

	return nil
}

type storeHandle struct {
	handle *windows.Handle
}

func newStoreHandle(provider uint32, store *uint16, flags uint32) (*storeHandle, error) {
	var s storeHandle
	if s.handle != nil {
		return &s, nil
	}
	st, err := windows.CertOpenStore(
		certStoreProvSystem,
		0,
		0,
		provider|flags,
		uintptr(unsafe.Pointer(store)))
	if err != nil {
		return nil, fmt.Errorf("CertOpenStore for the user store returned: %v", err)
	}
	s.handle = &st
	return &s, nil
}

func (s *storeHandle) Close() error {
	if s.handle != nil {
		return windows.CertCloseStore(*s.handle, 1)
	}
	return nil
}

// linkLegacy will associate the private key for a system certificate backed by cryptoAPI to
// the copy of the certificate stored in the user store. This makes the key available to legacy
// applications which may require it be specifically present in the users store to be read.
func (w *WinCertStore) linkLegacy() error {
	if w.ProvName != ProviderMSLegacy {
		return fmt.Errorf("cannot link legacy key, Provider mismatch: got %q, want %q", w.ProvName, ProviderMSLegacy)
	}
	deck.Info("Linking legacy key to the user private store.")

	cert, context, err := w.cert(w.issuers, my, certStoreLocalMachine)
	if err != nil {
		return fmt.Errorf("cert lookup returned: %v", err)
	}
	if context == nil {
		return errors.New("cert lookup returned: nil")
	}

	// Lookup the private key for the certificate.
	k, err := w.CertKey(context)
	if err != nil {
		return fmt.Errorf("unable to find legacy private key for %s: %v", cert.SerialNumber, err)
	}
	if k == nil {
		return errors.New("private key lookup returned: nil")
	}
	if k.LegacyContainer == "" {
		return fmt.Errorf("unable to find legacy private key for %s: container was empty", cert.SerialNumber)
	}

	// Generate the path to the expected current user's private key file.
	sid, err := UserSID()
	if err != nil {
		return fmt.Errorf("unable to determine user SID: %v", err)
	}
	_, file := filepath.Split(k.LegacyContainer)
	userContainer := fmt.Sprintf(`%s\Microsoft\Crypto\RSA\%s\%s`, os.Getenv("AppData"), sid, file)

	// Link the private key to the users private key store.
	if err := copyFile(k.LegacyContainer, userContainer); err != nil {
		return err
	}
	deck.Infof("Legacy key %q was located and linked to the user store.", k.LegacyContainer)
	return nil
}

// Remove removes certificates issued by any of w.issuers from the user and/or system cert stores.
// If it is unable to remove any certificates, it returns an error.
func (w *WinCertStore) Remove(removeSystem bool) error {
	if w.isReadOnly() {
		return fmt.Errorf("cannot remove certificates from a read-only store")
	}
	for _, issuer := range w.issuers {
		if err := w.remove(issuer, removeSystem); err != nil {
			return err
		}
	}
	return nil
}

// remove removes a certificate issued by w.issuer from the user and/or system cert stores.
func (w *WinCertStore) remove(issuer string, removeSystem bool) error {
	h, err := w.storeHandle(certStoreCurrentUser, my)
	if err != nil {
		return err
	}

	userCertContext, err := findCert(
		h,
		encodingX509ASN|encodingPKCS7,
		0,
		findIssuerStr,
		wide(issuer),
		nil)
	if err != nil {
		return fmt.Errorf("remove: finding user certificate issued by %s failed: %v", issuer, err)
	}

	if userCertContext != nil {
		if err := RemoveCertByContext(userCertContext); err != nil {
			return fmt.Errorf("failed to remove user cert: %v", err)
		}
		deck.Info("Cleaned up a user certificate.")
		fmt.Fprintln(os.Stderr, "Cleaned up a user certificate.")
	}

	// if we're only removing the user cert, return early.
	if !removeSystem {
		return nil
	}

	h2, err := w.storeHandle(certStoreLocalMachine, my)
	if err != nil {
		return err
	}

	systemCertContext, err := findCert(
		h2,
		encodingX509ASN|encodingPKCS7,
		0,
		findIssuerStr,
		wide(issuer),
		nil)
	if err != nil {
		return fmt.Errorf("remove: finding system certificate issued by %s failed: %v", issuer, err)
	}

	if systemCertContext != nil {
		if err := RemoveCertByContext(systemCertContext); err != nil {
			return fmt.Errorf("failed to remove system cert: %v", err)
		}
		deck.Info("Cleaned up a system certificate.")
		fmt.Fprintln(os.Stderr, "Cleaned up a system certificate.")
	}

	return nil
}

// RemoveCertByContext wraps CertDeleteCertificateFromStore. If the call succeeds, nil is returned, otherwise
// the extended error is returned.
func RemoveCertByContext(certContext *windows.CertContext) error {
	r, _, err := certDeleteCertificateFromStore.Call(uintptr(unsafe.Pointer(certContext)))
	if r != 1 {
		return fmt.Errorf("certdeletecertificatefromstore failed with %X: %v", r, err)
	}
	return nil
}

// Intermediate returns the current intermediate cert associated with this
// WinCertStore or nil if there isn't one.
func (w *WinCertStore) Intermediate() (*x509.Certificate, error) {
	c, _, err := w.cert(w.intermediateIssuers, my, w.storeDomain())
	return c, err
}

// Root returns the certificate issued by the specified issuer from the
// root certificate store 'ROOT/Certificates'.
func (w *WinCertStore) Root(issuer []string) (*x509.Certificate, error) {
	c, _, err := w.cert(issuer, root, w.storeDomain())
	return c, err
}

// CertificateChain returns the leaf and subsequent certificates.
func (w *WinCertStore) CertificateChain() ([][]*x509.Certificate, error) {
	// TODO: Once https://github.com/golang/go/issues/34977 is resolved
	//       use certificateChain() instead.
	cert, err := w.Cert()
	if err != nil {
		return nil, fmt.Errorf("unable to load leaf: %v", err)
	}
	if cert == nil {
		return nil, fmt.Errorf("load leaf: no certificate found")
	}
	// Calling Cert() builds certChains.
	return w.certChains, nil
}

// Key implements crypto.Signer and crypto.Decrypter for key based operations.
type Key struct {
	handle          uintptr
	pub             crypto.PublicKey
	Container       string
	LegacyContainer string
	AlgorithmGroup  string
}

// Public exports a public key to implement crypto.Signer
func (k Key) Public() crypto.PublicKey {
	return k.pub
}

// Sign returns the signature of a hash to implement crypto.Signer
func (k Key) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	switch k.AlgorithmGroup {
	case "ECDSA", "ECDH":
		return signECDSA(k.handle, digest)
	case "RSA":
		return signRSA(k.handle, digest, opts)
	default:
		return nil, fmt.Errorf("unsupported algorithm group %v", k.AlgorithmGroup)
	}
}

// TransientTpmHandle returns the key's underlying transient TPM handle.
func (k Key) TransientTpmHandle() uintptr {
	return k.handle
}

func signECDSA(kh uintptr, digest []byte) ([]byte, error) {
	var size uint32
	// Obtain the size of the signature
	r, _, err := nCryptSignHash.Call(
		kh,
		0,
		uintptr(unsafe.Pointer(&digest[0])),
		uintptr(len(digest)),
		0,
		0,
		uintptr(unsafe.Pointer(&size)),
		0)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSignHash returned %X during size check: %v", r, err)
	}

	// Obtain the signature data
	buf := make([]byte, size)
	r, _, err = nCryptSignHash.Call(
		kh,
		0,
		uintptr(unsafe.Pointer(&digest[0])),
		uintptr(len(digest)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&size)),
		0)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSignHash returned %X during signing: %v", r, err)
	}
	if len(buf) != int(size) {
		return nil, errors.New("invalid length")
	}

	return packECDSASigValue(bytes.NewReader(buf[:size]), len(digest))
}

func packECDSASigValue(r io.Reader, digestLength int) ([]byte, error) {
	sigR := make([]byte, digestLength)
	if _, err := io.ReadFull(r, sigR); err != nil {
		return nil, fmt.Errorf("failed to read R: %v", err)
	}

	sigS := make([]byte, digestLength)
	if _, err := io.ReadFull(r, sigS); err != nil {
		return nil, fmt.Errorf("failed to read S: %v", err)
	}

	var b cryptobyte.Builder
	b.AddASN1(asn1.SEQUENCE, func(b *cryptobyte.Builder) {
		b.AddASN1BigInt(new(big.Int).SetBytes(sigR))
		b.AddASN1BigInt(new(big.Int).SetBytes(sigS))
	})
	return b.Bytes()
}

func signRSA(kh uintptr, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	paddingInfo, flags, err := rsaPadding(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to construct padding info: %v", err)
	}
	var size uint32
	// Obtain the size of the signature
	r, _, err := nCryptSignHash.Call(
		kh,
		uintptr(paddingInfo),
		uintptr(unsafe.Pointer(&digest[0])),
		uintptr(len(digest)),
		0,
		0,
		uintptr(unsafe.Pointer(&size)),
		flags)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSignHash returned %X during size check: %v", r, err)
	}

	// Obtain the signature data
	sig := make([]byte, size)
	r, _, err = nCryptSignHash.Call(
		kh,
		uintptr(paddingInfo),
		uintptr(unsafe.Pointer(&digest[0])),
		uintptr(len(digest)),
		uintptr(unsafe.Pointer(&sig[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&size)),
		flags)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSignHash returned %X during signing: %v", r, err)
	}

	return sig[:size], nil
}

// rsaPadding constructs the padding info structure and flags from the crypto.SignerOpts.
// https://learn.microsoft.com/en-us/windows/win32/api/bcrypt/nf-bcrypt-bcryptsignhash
func rsaPadding(opts crypto.SignerOpts) (unsafe.Pointer, uintptr, error) {
	algID, ok := algIDs[opts.HashFunc()]
	if !ok {
		return nil, 0, fmt.Errorf("unsupported RSA hash algorithm %v", opts.HashFunc())
	}
	if o, ok := opts.(*rsa.PSSOptions); ok {
		saltLength := o.SaltLength
		switch saltLength {
		case rsa.PSSSaltLengthAuto:
			return nil, 0, fmt.Errorf("rsa.PSSSaltLengthAuto is not supported")
		case rsa.PSSSaltLengthEqualsHash:
			saltLength = o.HashFunc().Size()
		}
		return unsafe.Pointer(&pssPaddingInfo{
			pszAlgID: algID,
			cbSalt:   uint32(saltLength),
		}), bCryptPadPSS, nil
	}
	return unsafe.Pointer(&pkcs1PaddingInfo{
		pszAlgID: algID,
	}), bCryptPadPKCS1, nil
}

// DecrypterOpts implements crypto.DecrypterOpts and contains the
// flags required for the NCryptDecrypt system call.
type DecrypterOpts struct {
	// Hashfunc represents the hashing function that was used during
	// encryption and is mapped to the Microsoft equivalent LPCWSTR.
	Hashfunc crypto.Hash
	// Flags represents the dwFlags parameter for NCryptDecrypt
	Flags uint32
}

// oaepPaddingInfo is the BCRYPT_OAEP_PADDING_INFO struct in bcrypt.h.
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa375526(v=vs.85).aspx
type oaepPaddingInfo struct {
	pszAlgID *uint16 // pszAlgId
	pbLabel  *uint16 // pbLabel
	cbLabel  uint32  // cbLabel
}

// Decrypt returns the decrypted contents of the encrypted blob, and implements
// crypto.Decrypter for Key.
func (k Key) Decrypt(rand io.Reader, blob []byte, opts crypto.DecrypterOpts) ([]byte, error) {
	decrypterOpts, ok := opts.(DecrypterOpts)
	if !ok {
		return nil, errors.New("opts was not certtostore.DecrypterOpts")
	}

	algID, ok := algIDs[decrypterOpts.Hashfunc]
	if !ok {
		return nil, fmt.Errorf("unsupported hash algorithm %v", decrypterOpts.Hashfunc)
	}

	padding := oaepPaddingInfo{
		pszAlgID: algID,
		pbLabel:  wide(""),
		cbLabel:  0,
	}

	return decrypt(k.handle, blob, padding, decrypterOpts.Flags)
}

// decrypt wraps the NCryptDecrypt function and returns the decrypted bytes
// that were previously encrypted by NCryptEncrypt or another compatible
// function such as rsa.EncryptOAEP.
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa376249(v=vs.85).aspx
func decrypt(kh uintptr, blob []byte, padding oaepPaddingInfo, flags uint32) ([]byte, error) {
	var size uint32
	// Obtain the size of the decrypted data
	r, _, err := nCryptDecrypt.Call(
		kh,
		uintptr(unsafe.Pointer(&blob[0])),
		uintptr(len(blob)),
		uintptr(unsafe.Pointer(&padding)),
		0, // Must be null on first run.
		0, // Ignored on first run.
		uintptr(unsafe.Pointer(&size)),
		uintptr(flags))
	if r != 0 {
		return nil, fmt.Errorf("NCryptDecrypt returned %X during size check: %v", r, err)
	}

	// Decrypt the message
	plainText := make([]byte, size)
	r, _, err = nCryptDecrypt.Call(
		kh,
		uintptr(unsafe.Pointer(&blob[0])),
		uintptr(len(blob)),
		uintptr(unsafe.Pointer(&padding)),
		uintptr(unsafe.Pointer(&plainText[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&size)),
		uintptr(flags))
	if r != 0 {
		return nil, fmt.Errorf("NCryptDecrypt returned %X during decryption: %v", r, err)
	}

	return plainText[:size], nil
}

// SetACL sets the requested permissions on the private key. If a
// cryptoAPI compatible copy of the key is present, the same ACL is set.
func (k *Key) SetACL(access string, sid string, perm string) error {
	if err := setACL(k.Container, access, sid, perm); err != nil {
		return err
	}
	if k.LegacyContainer == "" {
		return nil
	}
	return setACL(k.LegacyContainer, access, sid, perm)
}

// setACL sets permissions for the private key by wrapping the Microsoft
// icacls utility. icacls is used for simplicity working with NTFS ACLs.
func setACL(file, access, sid, perm string) error {
	deck.Infof("running: %s %s /%s %s:%s", icaclsPath, file, access, sid, perm)
	// Parameter validation isn't required, icacls handles this on its own.
	err := exec.Command(icaclsPath, file, "/"+access, sid+":"+perm).Run()
	// Error 1798 can safely be ignored, because it occurs when trying to set an acl
	// for a non-existend sid, which only happens for certain permissions needed on later
	// versions of Windows.
	if err1, ok := err.(*exec.ExitError); ok && !strings.Contains(err1.Error(), "1798") {
		deck.Infof("ignoring error while %sing '%s' access to %s for sid: %v", access, perm, file, sid)
		return nil
	} else if err1 != nil {
		return fmt.Errorf("certstorage.SetFileACL is unable to %s %s access on %s to sid %s, %v", access, perm, file, sid, err1)
	} else if !ok && err != nil {
		return fmt.Errorf("certstorage.SetFileACL failed to pull exit error while %s %s access on %s to sid %s, %v", access, perm, file, sid, err)
	}
	return nil
}

// Key opens a handle to an existing private key and returns key.
// Key implements both crypto.Signer and crypto.Decrypter.
//
// Important: The Key lookup is based on the provider passed to OpenWinCertStore. This
// *may not match* the certificate obtained by Cert() for the same store, which may be associated
// with a different provider. Use CertKey() to derive a key directly from a Cert in situations
// where both are needed.
func (w *WinCertStore) Key() (Credential, error) {
	var kh uintptr
	r, _, err := nCryptOpenKey.Call(
		uintptr(w.Prov),
		uintptr(unsafe.Pointer(&kh)),
		uintptr(unsafe.Pointer(wide(w.container))),
		0,
		w.keyAccessFlags)
	if r != 0 {
		return nil, fmt.Errorf("NCryptOpenKey for container %q returned %X: %v", w.container, r, err)
	}

	return keyMetadata(kh, w)
}

// CertKey wraps CryptAcquireCertificatePrivateKey. It obtains the CNG private
// key of a known certificate and returns a pointer to a Key which implements
// both crypto.Signer and crypto.Decrypter. When a nil cert context is passed
// a nil key is intentionally returned, to model the expected behavior of a
// non-existent cert having no private key.
// https://docs.microsoft.com/en-us/windows/win32/api/wincrypt/nf-wincrypt-cryptacquirecertificateprivatekey
func (w *WinCertStore) CertKey(cert *windows.CertContext) (*Key, error) {
	// Return early if a nil cert was passed.
	if cert == nil {
		return nil, nil
	}
	var (
		kh       uintptr
		spec     uint32
		mustFree int
	)
	r, _, err := cryptAcquireCertificatePrivateKey.Call(
		uintptr(unsafe.Pointer(cert)),
		acquireCached|acquireSilent|acquireOnlyNCryptKey,
		0, // Reserved, must be null.
		uintptr(unsafe.Pointer(&kh)),
		uintptr(unsafe.Pointer(&spec)),
		uintptr(unsafe.Pointer(&mustFree)),
	)
	// If the function succeeds, the return value is nonzero (TRUE).
	if r == 0 {
		return nil, fmt.Errorf("cryptAcquireCertificatePrivateKey returned %X: %v", r, err)
	}
	if mustFree != 0 {
		return nil, fmt.Errorf("wrong mustFree [%d != 0]", mustFree)
	}
	if spec != ncryptKeySpec {
		return nil, fmt.Errorf("wrong keySpec [%d != %d]", spec, ncryptKeySpec)
	}

	return keyMetadata(kh, w)
}

// Generate returns a crypto.Signer representing either a TPM-backed or
// software backed key, depending on support from the host OS
// key size is set to the maximum supported by Microsoft Software Key Storage Provider
func (w *WinCertStore) Generate(opts GenerateOpts) (crypto.Signer, error) {
	if w.isReadOnly() {
		return nil, fmt.Errorf("cannot generate keys in a read-only store")
	}
	deck.Infof("Provider: %s", w.ProvName)
	switch opts.Algorithm {
	case EC:
		switch opts.Size {
		case 0, 256:
			return w.generateECDSA("ECDSA_P256")
		case 384:
			return w.generateECDSA("ECDSA_P384")
		case 521:
			return w.generateECDSA("ECDSA_P521")
		default:
			return nil, fmt.Errorf("unsupported curve size: %d", opts.Size)
		}
	case RSA:
		return w.generateRSA(opts.Size)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", opts.Algorithm)
	}
}

func (w *WinCertStore) generateECDSA(algID string) (crypto.Signer, error) {
	var kh uintptr
	// Pass 0 as the fifth parameter because it is not used (legacy)
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa376247(v=vs.85).aspx
	r, _, err := nCryptCreatePersistedKey.Call(
		uintptr(w.Prov),
		uintptr(unsafe.Pointer(&kh)),
		uintptr(unsafe.Pointer(wide(algID))),
		uintptr(unsafe.Pointer(wide(w.container))),
		0,
		w.keyAccessFlags|nCryptOverwriteKey)
	if r != 0 {
		return nil, fmt.Errorf("NCryptCreatePersistedKey returned %X: %v", r, err)
	}

	usage := uint32(ncryptAllowSigningFlag)
	r, _, err = nCryptSetProperty.Call(
		kh,
		uintptr(unsafe.Pointer(wide("Key Usage"))),
		uintptr(unsafe.Pointer(&usage)),
		unsafe.Sizeof(usage),
		ncryptPersistFlag)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSetProperty (Key Usage) returned %X: %v", r, err)
	}

	// keystorage flags are typically zero except when an RSA legacykey is required.
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa376265(v=vs.85).aspx
	r, _, err = nCryptFinalizeKey.Call(kh, w.keyStorageFlags)
	if r != 0 {
		return nil, fmt.Errorf("NCryptFinalizeKey returned %X: %v", r, err)
	}

	return keyMetadata(kh, w)
}

func (w *WinCertStore) generateRSA(keySize int) (crypto.Signer, error) {
	// The MPCP only supports a max keywidth of 2048, due to the TPM specification.
	// https://www.microsoft.com/en-us/download/details.aspx?id=52487
	// The Microsoft Software Key Storage Provider supports a max keywidth of 16384.
	if keySize > 16384 {
		return nil, fmt.Errorf("unsupported keysize, got: %d, want: < %d", keySize, 16384)
	}

	var kh uintptr
	var length = uint32(keySize)
	// Pass 0 as the fifth parameter because it is not used (legacy)
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa376247(v=vs.85).aspx
	r, _, err := nCryptCreatePersistedKey.Call(
		uintptr(w.Prov),
		uintptr(unsafe.Pointer(&kh)),
		uintptr(unsafe.Pointer(wide("RSA"))),
		uintptr(unsafe.Pointer(wide(w.container))),
		0,
		w.keyAccessFlags|nCryptOverwriteKey)
	if r != 0 {
		return nil, fmt.Errorf("NCryptCreatePersistedKey returned %X: %v", r, err)
	}

	// Microsoft function calls return actionable return codes in r, err is often filled with text, even when successful
	r, _, err = nCryptSetProperty.Call(
		kh,
		uintptr(unsafe.Pointer(wide("Length"))),
		uintptr(unsafe.Pointer(&length)),
		unsafe.Sizeof(length),
		ncryptPersistFlag)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSetProperty (Length) returned %X: %v", r, err)
	}

	usage := uint32(ncryptAllowDecryptFlag | ncryptAllowSigningFlag)
	r, _, err = nCryptSetProperty.Call(
		kh,
		uintptr(unsafe.Pointer(wide("Key Usage"))),
		uintptr(unsafe.Pointer(&usage)),
		unsafe.Sizeof(usage),
		ncryptPersistFlag)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSetProperty (Key Usage) returned %X: %v", r, err)
	}

	// keystorage flags are typically zero except when an RSA legacykey is required.
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa376265(v=vs.85).aspx
	r, _, err = nCryptFinalizeKey.Call(kh, w.keyStorageFlags)
	if r != 0 {
		return nil, fmt.Errorf("NCryptFinalizeKey returned %X: %v", r, err)
	}

	return keyMetadata(kh, w)
}

func keyMetadata(kh uintptr, store *WinCertStore) (*Key, error) {
	// uc is used to populate the unique container name attribute of the private key
	uc, err := getPropertyStr(kh, nCryptUniqueNameProperty)
	if err != nil {
		return nil, fmt.Errorf("unable to determine key unique name: %v", err)
	}

	// get the provider handle
	ph, err := getPropertyHandle(kh, nCryptProviderHandleProperty)
	if err != nil {
		return nil, fmt.Errorf("unable to determine key provider: %v", err)
	}
	defer freeObject(ph)

	// get the provider implementation from the provider handle
	impl, err := getPropertyUint32(ph, nCryptImplTypeProperty)
	if err != nil {
		return nil, fmt.Errorf("unable to determine provider implementation: %v", err)
	}

	// Populate key storage locations for software backed keys.
	var lc string

	// Functions like cert() pull certs from the local store *regardless*
	// of the provider OpenWinCertStore was given. This means we cannot rely on
	// store.Prov to tell us which provider a given key resides in. Instead, we
	// lookup the provider directly from the key properties.
	if (impl & nCryptImplSoftwareFlag) != 0 {
		uc, lc, err = softwareKeyContainers(uc, store.storeDomain())
		if err != nil {
			return nil, err
		}
	}

	alg, err := getPropertyStr(kh, nCryptAlgorithmGroupProperty)
	if err != nil {
		return nil, fmt.Errorf("unable to determine key algorithm: %v", err)
	}
	var pub crypto.PublicKey
	switch alg {
	case "ECDSA", "ECDH":
		buf, err := export(kh, bCryptECCPublicBlob)
		if err != nil {
			return nil, fmt.Errorf("failed to export ECC public key: %v", err)
		}
		pub, err = unmarshalECC(buf, kh)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal ECC public key: %v", err)
		}
	default:
		buf, err := export(kh, bCryptRSAPublicBlob)
		if err != nil {
			return nil, fmt.Errorf("failed to export %v public key: %v", alg, err)
		}
		pub, err = unmarshalRSA(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal %v public key: %v", alg, err)
		}
	}

	return &Key{handle: kh, pub: pub, Container: uc, LegacyContainer: lc, AlgorithmGroup: alg}, nil
}

func getProperty(kh uintptr, property *uint16) ([]byte, error) {
	var strSize uint32
	r, _, err := nCryptGetProperty.Call(
		kh,
		uintptr(unsafe.Pointer(property)),
		0,
		0,
		uintptr(unsafe.Pointer(&strSize)),
		0,
		0)
	if r != 0 {
		return nil, fmt.Errorf("NCryptGetProperty(%v) returned %X during size check: %v", property, r, err)
	}

	buf := make([]byte, strSize)
	r, _, err = nCryptGetProperty.Call(
		kh,
		uintptr(unsafe.Pointer(property)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(strSize),
		uintptr(unsafe.Pointer(&strSize)),
		0,
		0)
	if r != 0 {
		return nil, fmt.Errorf("NCryptGetProperty %v returned %X during export: %v", property, r, err)
	}

	return buf, nil
}

func getPropertyHandle(kh uintptr, property *uint16) (uintptr, error) {
	buf, err := getProperty(kh, property)
	if err != nil {
		return 0, err
	}
	if len(buf) < 1 {
		return 0, fmt.Errorf("empty result")
	}
	return **(**uintptr)(unsafe.Pointer(&buf)), nil
}

func getPropertyUint32(kh uintptr, property *uint16) (uint32, error) {
	buf, err := fnGetProperty(kh, property)
	if err != nil {
		return 0, err
	}
	if len(buf) < 1 {
		return 0, fmt.Errorf("empty result")
	}
	return **(**uint32)(unsafe.Pointer(&buf)), nil
}

func getPropertyStr(kh uintptr, property *uint16) (string, error) {
	buf, err := fnGetProperty(kh, property)
	if err != nil {
		return "", err
	}
	uc := bytes.ReplaceAll(buf, []byte{0x00}, []byte(""))
	return string(uc), nil
}

func export(kh uintptr, blobType *uint16) ([]byte, error) {
	var size uint32
	// When obtaining the size of a public key, most parameters are not required
	r, _, err := nCryptExportKey.Call(
		kh,
		0,
		uintptr(unsafe.Pointer(blobType)),
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&size)),
		0)
	if r != 0 {
		return nil, fmt.Errorf("NCryptExportKey returned %X during size check: %v", r, err)
	}

	// Place the exported key in buf now that we know the size required
	buf := make([]byte, size)
	r, _, err = nCryptExportKey.Call(
		kh,
		0,
		uintptr(unsafe.Pointer(blobType)),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&size)),
		0)
	if r != 0 {
		return nil, fmt.Errorf("NCryptExportKey returned %X during export: %v", r, err)
	}
	return buf, nil
}

func unmarshalRSA(buf []byte) (*rsa.PublicKey, error) {
	// BCRYPT_RSA_BLOB from bcrypt.h
	header := struct {
		Magic         uint32
		BitLength     uint32
		PublicExpSize uint32
		ModulusSize   uint32
		UnusedPrime1  uint32
		UnusedPrime2  uint32
	}{}

	r := bytes.NewReader(buf)
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if header.Magic != rsa1Magic {
		return nil, fmt.Errorf("invalid header magic %x", header.Magic)
	}

	if header.PublicExpSize > 8 {
		return nil, fmt.Errorf("unsupported public exponent size (%d bits)", header.PublicExpSize*8)
	}

	exp := make([]byte, 8)
	if n, err := r.Read(exp[8-header.PublicExpSize:]); n != int(header.PublicExpSize) || err != nil {
		return nil, fmt.Errorf("failed to read public exponent (%d, %v)", n, err)
	}

	mod := make([]byte, header.ModulusSize)
	if n, err := r.Read(mod); n != int(header.ModulusSize) || err != nil {
		return nil, fmt.Errorf("failed to read modulus (%d, %v)", n, err)
	}

	pub := &rsa.PublicKey{
		N: new(big.Int).SetBytes(mod),
		E: int(binary.BigEndian.Uint64(exp)),
	}
	return pub, nil
}

func unmarshalECC(buf []byte, kh uintptr) (*ecdsa.PublicKey, error) {
	// BCRYPT_ECCKEY_BLOB from bcrypt.h
	header := struct {
		Magic uint32
		Key   uint32
	}{}

	r := bytes.NewReader(buf)
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	curve, ok := curveIDs[header.Magic]
	if !ok {
		// Fix for b/185945636, where despite specifying the curve, nCrypt returns
		// an incorrect response with BCRYPT_ECDSA_PUBLIC_GENERIC_MAGIC.
		var err error
		curve, err = curveName(kh)
		if err != nil {
			return nil, fmt.Errorf("unsupported header magic: %x and cannot match the curve by name: %v", header.Magic, err)
		}
	}

	keyX := make([]byte, header.Key)
	if n, err := r.Read(keyX); n != int(header.Key) || err != nil {
		return nil, fmt.Errorf("failed to read key X (%d, %v)", n, err)
	}

	keyY := make([]byte, header.Key)
	if n, err := r.Read(keyY); n != int(header.Key) || err != nil {
		return nil, fmt.Errorf("failed to read key Y (%d, %v)", n, err)
	}

	pub := &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(keyX),
		Y:     new(big.Int).SetBytes(keyY),
	}
	return pub, nil
}

// curveName reads the curve name property and returns the corresponding curve.
func curveName(kh uintptr) (elliptic.Curve, error) {
	cn, err := getPropertyStr(kh, nCryptECCCurveNameProperty)
	if err != nil {
		return nil, fmt.Errorf("unable to determine the curve property name: %v", err)
	}
	curve, ok := curveNames[cn]
	if !ok {
		return nil, fmt.Errorf("unknown curve name")
	}
	return curve, nil
}

// Store imports certificates into the Windows certificate store
func (w *WinCertStore) Store(cert *x509.Certificate, intermediate *x509.Certificate) error {
	if w.isReadOnly() {
		return fmt.Errorf("cannot store certificates in a read-only store")
	}
	return w.StoreWithDisposition(cert, intermediate, windows.CERT_STORE_ADD_ALWAYS)
}

// StoreWithDisposition imports certificates into the Windows certificate store.
// disposition specifies the action to take if a matching certificate
// or a link to a matching certificate already exists in the store
// https://learn.microsoft.com/en-us/windows/win32/api/wincrypt/nf-wincrypt-certaddcertificatecontexttostore
func (w *WinCertStore) StoreWithDisposition(cert *x509.Certificate, intermediate *x509.Certificate, disposition uint32) error {
	if w.isReadOnly() {
		return fmt.Errorf("cannot store certificates in a read-only store")
	}
	certContext, err := windows.CertCreateCertificateContext(
		encodingX509ASN|encodingPKCS7,
		&cert.Raw[0],
		uint32(len(cert.Raw)))
	if err != nil {
		return fmt.Errorf("CertCreateCertificateContext returned: %v", err)
	}
	defer windows.CertFreeCertificateContext(certContext)

	// Associate the private key we previously generated
	r, _, err := cryptFindCertificateKeyProvInfo.Call(
		uintptr(unsafe.Pointer(certContext)),
		uintptr(uint32(0)),
		0,
	)
	// Windows calls will fill err with a success message, r is what must be checked instead
	if r == 0 {
		return fmt.Errorf("found a matching private key for this certificate, but association failed: %v", err)
	}

	// Open a handle to the system cert store
	h, err := w.storeHandle(w.storeDomain(), my)
	if err != nil {
		return err
	}

	// Add the cert context to the system certificate store
	if err := windows.CertAddCertificateContextToStore(h, certContext, disposition, nil); err != nil {
		return fmt.Errorf("CertAddCertificateContextToStore returned: %v", err)
	}

	// Prep the intermediate cert context
	intContext, err := windows.CertCreateCertificateContext(
		encodingX509ASN|encodingPKCS7,
		&intermediate.Raw[0],
		uint32(len(intermediate.Raw)))
	if err != nil {
		return fmt.Errorf("CertCreateCertificateContext returned: %v", err)
	}
	defer windows.CertFreeCertificateContext(intContext)

	h2, err := w.storeHandle(w.storeDomain(), ca)
	if err != nil {
		return err
	}

	// Add the intermediate cert context to the store
	if err := windows.CertAddCertificateContextToStore(h2, intContext, disposition, nil); err != nil {
		return fmt.Errorf("CertAddCertificateContextToStore returned: %v", err)
	}

	return nil
}

// Returns a handle to a given cert store, opening the handle as needed.
func (w *WinCertStore) storeHandle(provider uint32, store *uint16) (windows.Handle, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	key := fmt.Sprintf("%d%s", provider, windows.UTF16PtrToString(store))
	var err error
	if w.stores[key] == nil {
		w.stores[key], err = newStoreHandle(provider, store, w.storeFlags)
		if err != nil {
			return 0, err
		}
	}
	return *w.stores[key].handle, nil
}

// copyFile copies the contents of one file from one location to another
func copyFile(from, to string) error {
	source, err := os.Open(from)
	if err != nil {
		return fmt.Errorf("os.Open(%s) returned: %v", from, err)
	}
	defer source.Close()

	dest, err := os.OpenFile(to, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("os.OpenFile(%s) returned: %v", to, err)
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	if err != nil {
		return fmt.Errorf("io.Copy(%q, %q) returned: %v", to, from, err)
	}

	return nil
}

// softwareKeyContainers returns the file path for a software backed key. If the key
// was finalized with with NCRYPT_WRITE_KEY_TO_LEGACY_STORE_FLAG, it also returns its
// equivalent CryptoAPI key file path.
// https://docs.microsoft.com/en-us/windows/win32/api/ncrypt/nf-ncrypt-ncryptfinalizekey.
func softwareKeyContainers(uniqueID string, storeDomain uint32) (string, string, error) {
	var cngRoot, capiRoot string
	switch storeDomain {
	case certStoreLocalMachine:
		cngRoot = os.Getenv("ProgramData") + `\Microsoft\Crypto\Keys\`
		capiRoot = os.Getenv("ProgramData") + `\Microsoft\Crypto\RSA\MachineKeys\`
	case certStoreCurrentUser:
		cngRoot = os.Getenv("AppData") + `\Microsoft\Crypto\Keys\`
		sid, err := UserSID()
		if err != nil {
			return "", "", fmt.Errorf("unable to determine user SID: %v", err)
		}
		capiRoot = fmt.Sprintf(`%s\Microsoft\Crypto\RSA\%s\`, os.Getenv("AppData"), sid)
	default:
		return "", "", fmt.Errorf("unexpected store domain %d", storeDomain)
	}

	// Determine the key type, so that we know which container we are
	// working with.
	var keyType, cng, capi string
	if _, err := os.Stat(cngRoot + uniqueID); err == nil {
		keyType = "CNG"
	}
	if _, err := os.Stat(capiRoot + uniqueID); err == nil {
		keyType = "CAPI"
	}

	// Generate the container path for the keyType we already have,
	// and lookup the container path for the keyType we need to infer.
	var err error
	switch keyType {
	case "CNG":
		cng = cngRoot + uniqueID
		capi, err = keyMatch(cng, capiRoot)
		if err != nil {
			return "", "", fmt.Errorf("error locating legacy key: %v", err)
		}
	case "CAPI":
		capi = capiRoot + uniqueID
		cng, err = keyMatch(capi, cngRoot)
		if err != nil {
			return "", "", fmt.Errorf("unable to locate CNG key: %v", err)
		}
		if cng == "" {
			return "", "", errors.New("CNG key was empty")
		}
	default:
		return "", "", fmt.Errorf("unexpected key type %q", keyType)
	}

	return cng, capi, nil
}

// keyMatch takes a known path to a private key and searches for a
// matching key in a provided directory.
func keyMatch(keyPath, dir string) (string, error) {
	key, err := os.Stat(keyPath)
	if err != nil {
		return "", fmt.Errorf("unable to determine key creation date: %v", err)
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("unable to locate search directory: %v", err)
	}
	// A matching key is present in the target directory when it has a modified
	// timestamp within 5 minutes of the known key. Checking the timestamp is
	// necessary to select the right key. Typically, there are several machine
	// keys present, only one of which was created at the same time as the
	// known key.
	for _, f := range files {
		age := int(key.ModTime().Sub(f.ModTime()) / time.Second)
		if age >= -300 && age < 300 {
			return dir + f.Name(), nil
		}
	}
	return "", nil
}

// Verify interface conformance.
var _ CertStorage = &WinCertStore{}
var _ Credential = &Key{}

func (w *WinCertStore) isReadOnly() bool {
	return (w.storeFlags & CertStoreReadOnly) != 0
}

// CertByCommonName searches for a certificate by its common name in the store.
// The returned *windows.CertContext must be freed by the caller using
// FreeCertContext to avoid resource leaks.
func (w *WinCertStore) CertByCommonName(commonName string) (*x509.Certificate,
	*windows.CertContext, [][]*x509.Certificate, error) {
	storeHandle, err := w.storeHandle(w.storeDomain(), my)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to open certificate store: %v", err)
	}
	var certContext *windows.CertContext
	var cert *x509.Certificate
	for {
		certContext, err = findCert(
			storeHandle,
			encodingX509ASN|encodingPKCS7,
			0,
			windows.CERT_FIND_SUBJECT_STR,
			wide(commonName),
			certContext,
		)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("could not find certificate by common name %q: %w",
				commonName, err)
		}
		if certContext == nil {
			break // No more certificates found
		}
		cert, err = certContextToX509(certContext)
		if err != nil {
			FreeCertContext(certContext) // Free context to avoid memory leak
			continue                     // Skip invalid certificates
		}
		if err := w.resolveChains(certContext); err != nil {
			FreeCertContext(certContext)
			return nil, nil, nil, err
		}
		// Found a valid certificate, return it.
		return cert, certContext, w.certChains, nil
	}
	return nil, nil, nil, cryptENotFound
}

package platform

import (
	"fmt"
	"runtime"
	"sync"

	"golang.org/x/sys/unix"
)

const (
	atHWCAP  = 16
	atHWCAP2 = 26
)

// HWCAP bits from arch/arm64/include/uapi/asm/hwcap.h.
const (
	hwcapCRC32    = 1 << 7
	hwcapATOMICS  = 1 << 8
	hwcapFPHP     = 1 << 9
	hwcapASIMDHP  = 1 << 10
	hwcapASIMDRDM = 1 << 12
	hwcapJSCVT    = 1 << 13
	hwcapFCMA     = 1 << 14
	hwcapLRCPC    = 1 << 15
	hwcapDCPOP    = 1 << 16
	hwcapASIMDDP  = 1 << 20
	hwcapDIT      = 1 << 24
	hwcapUSCAT    = 1 << 25
	hwcapILRCPC   = 1 << 26
	hwcapFLAGM    = 1 << 27
	hwcapSB       = 1 << 29
	hwcapPACA     = 1 << 30
	hwcapPACG     = 1 << 31

	hwcap2DCPODP = 1 << 0
	hwcap2SVE2   = 1 << 1
	hwcap2FLAGM2 = 1 << 7
	hwcap2FRINT  = 1 << 8
	hwcap2I8MM   = 1 << 13
	hwcap2BF16   = 1 << 14
	hwcap2BTI    = 1 << 17
	hwcap2ECV    = 1 << 19
	hwcap2WFXT   = 1 << 31
	hwcap2CSSC   = 1 << 34
	hwcap2MOPS   = 1 << 43
	hwcap2SVE2P1 = 1 << 36
	hwcap2HBC    = 1 << 44
)

// arm64Levels lists the features required by each v8 minor version, in addition
// to the ones required by the previous versions. Features implied or enabled by
// default for each architecture version in LLVM's AArch64Features.td, limited
// to the ones user space can use and the kernel reports in HWCAP.
var arm64Levels = []struct {
	hwcap, hwcap2 uint64
}{
	// v8.1: LLVM: FeatureCRC, FeatureLSE, FeatureRDM, FeaturePAN, FeatureLOR,
	// FeatureVH
	{hwcap: hwcapCRC32 | hwcapATOMICS | hwcapASIMDRDM},
	// v8.2: LLVM: FeaturePsUAO, FeaturePAN_RWV, FeatureRAS, FeatureCCPP
	{hwcap: hwcapDCPOP},
	// v8.3: LLVM: FeatureRCPC, FeaturePAuth, FeatureJS, FeatureComplxNum
	{hwcap: hwcapLRCPC | hwcapPACA | hwcapPACG | hwcapJSCVT | hwcapFCMA},
	// v8.4: LLVM: FeatureDotProd, FeatureNV, FeatureMPAM, FeatureDIT,
	// FeatureTRACEV8_4, FeatureAM, FeatureSEL2, FeatureTLB_RMI, FeatureFlagM,
	// FeatureRCPC_IMMO, FeatureLSE2
	{hwcap: hwcapASIMDDP | hwcapDIT | hwcapFLAGM | hwcapILRCPC | hwcapUSCAT},
	// v8.5: LLVM: FeatureAltFPCmp, FeatureFRInt3264, FeatureSpecRestrict,
	// FeatureSB, FeaturePredRes, FeatureCacheDeepPersist, FeatureBranchTargetId
	{hwcap: hwcapSB, hwcap2: hwcap2FLAGM2 | hwcap2FRINT | hwcap2DCPODP | hwcap2BTI},
	// v8.6: LLVM: FeatureAMVS, FeatureBF16, FeatureFineGrainedTraps,
	// FeatureEnhancedCounterVirtualization, FeatureMatMulInt8
	{hwcap2: hwcap2BF16 | hwcap2ECV | hwcap2I8MM},
	// v8.7: LLVM: FeatureXS, FeatureWFxT
	{hwcap2: hwcap2WFXT},
	// v8.8: LLVM: FeatureHBC, FeatureMOPS, FeatureNMI
	{hwcap2: hwcap2HBC | hwcap2MOPS},
	// v8.9: LLVM: FeatureCLRBHB, FeaturePRFM_SLC, FeatureSPECRES2, FeatureCSSC,
	// FeatureRASv2, FeatureCHK
	{hwcap2: hwcap2CSSC},
}

// arm64VariantFromHWCAP returns the highest arm64 variant whose features are
// all present in hwcap and hwcap2.
func arm64VariantFromHWCAP(hwcap, hwcap2 uint64) string {
	minor := 0
	for _, l := range arm64Levels {
		if hwcap&l.hwcap != l.hwcap || hwcap2&l.hwcap2 != l.hwcap2 {
			break
		}
		minor++
	}

	// Armv9.x requires SVE2, FP16 and the features of Armv8.(x+5).
	const v9hwcap = hwcapFPHP | hwcapASIMDHP
	const v9hwcap2 = hwcap2SVE2
	if minor >= 5 && hwcap&v9hwcap == v9hwcap && hwcap2&v9hwcap2 == v9hwcap2 {
		minor -= 5
		// Armv9.4 also requires SVE2.1.
		if minor >= 4 && hwcap2&hwcap2SVE2P1 == 0 {
			minor = 3
		}
		if minor == 0 {
			return "v9"
		}
		return fmt.Sprintf("v9.%d", minor)
	}
	if minor == 0 {
		return "v8"
	}
	return fmt.Sprintf("v8.%d", minor)
}

var arm64Variant = sync.OnceValue(func() string {
	if runtime.GOARCH != "arm64" {
		return ""
	}
	auxv, err := unix.Auxv()
	if err != nil {
		return ""
	}
	var hwcap, hwcap2 uint64
	for _, kv := range auxv {
		switch kv[0] {
		case atHWCAP:
			hwcap = uint64(kv[1])
		case atHWCAP2:
			hwcap2 = uint64(kv[1])
		}
	}
	return arm64VariantFromHWCAP(hwcap, hwcap2)
})

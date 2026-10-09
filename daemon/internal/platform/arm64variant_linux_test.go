package platform

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// upTo returns the features of the first n levels of arm64Levels.
func upTo(n int) (hwcap, hwcap2 uint64) {
	for _, l := range arm64Levels[:n] {
		hwcap |= l.hwcap
		hwcap2 |= l.hwcap2
	}
	return hwcap, hwcap2
}

func TestARM64VariantFromHWCAP(t *testing.T) {
	const fp16 = hwcapFPHP | hwcapASIMDHP
	v81, v81hwcap2 := upTo(1)
	v82, v82hwcap2 := upTo(2)
	v84, v84hwcap2 := upTo(4)
	v85, v85hwcap2 := upTo(5)
	v89, v89hwcap2 := upTo(len(arm64Levels))
	v83 := arm64Levels[2]

	tests := []struct {
		name          string
		hwcap, hwcap2 uint64
		expected      string
	}{
		{name: "none", expected: "v8"},
		{name: "v8.1", hwcap: v81, hwcap2: v81hwcap2, expected: "v8.1"},
		{name: "v8.9", hwcap: v89, hwcap2: v89hwcap2, expected: "v8.9"},
		{name: "v8.3 partial", hwcap: v82 | v83.hwcap&(v83.hwcap-1), hwcap2: v82hwcap2 | v83.hwcap2, expected: "v8.2"},
		{name: "v8.3 without v8.2", hwcap: v81 | v83.hwcap, hwcap2: v81hwcap2 | v83.hwcap2, expected: "v8.1"},
		{name: "v9.0", hwcap: v85 | fp16, hwcap2: v85hwcap2 | hwcap2SVE2, expected: "v9"},
		{name: "v9.4", hwcap: v89 | fp16, hwcap2: v89hwcap2 | hwcap2SVE2 | hwcap2SVE2P1, expected: "v9.4"},
		{name: "v9.4 without SVE2.1", hwcap: v89 | fp16, hwcap2: v89hwcap2 | hwcap2SVE2, expected: "v9.3"},
		{name: "SVE2 before v8.5", hwcap: v84 | fp16, hwcap2: v84hwcap2 | hwcap2SVE2, expected: "v8.4"},
		{name: "SVE2 without FP16", hwcap: v85, hwcap2: v85hwcap2 | hwcap2SVE2, expected: "v8.5"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, arm64VariantFromHWCAP(tc.hwcap, tc.hwcap2), tc.expected)
		})
	}
}

// Features from /proc/cpuinfo, using names from the kernel. Unknown names
// are not used for the detection.
var cpuinfoHWCAP = map[string]uint64{
	"crc32":    hwcapCRC32,
	"atomics":  hwcapATOMICS,
	"fphp":     hwcapFPHP,
	"asimdhp":  hwcapASIMDHP,
	"asimdrdm": hwcapASIMDRDM,
	"jscvt":    hwcapJSCVT,
	"fcma":     hwcapFCMA,
	"lrcpc":    hwcapLRCPC,
	"dcpop":    hwcapDCPOP,
	"asimddp":  hwcapASIMDDP,
	"dit":      hwcapDIT,
	"uscat":    hwcapUSCAT,
	"ilrcpc":   hwcapILRCPC,
	"flagm":    hwcapFLAGM,
	"sb":       hwcapSB,
	"paca":     hwcapPACA,
	"pacg":     hwcapPACG,
}

var cpuinfoHWCAP2 = map[string]uint64{
	"dcpodp": hwcap2DCPODP,
	"sve2":   hwcap2SVE2,
	"sve2p1": hwcap2SVE2P1,
	"flagm2": hwcap2FLAGM2,
	"frint":  hwcap2FRINT,
	"i8mm":   hwcap2I8MM,
	"bf16":   hwcap2BF16,
	"bti":    hwcap2BTI,
	"ecv":    hwcap2ECV,
	"wfxt":   hwcap2WFXT,
	"cssc":   hwcap2CSSC,
	"mops":   hwcap2MOPS,
	"hbc":    hwcap2HBC,
}

// TestARM64VariantFromCPUInfo uses the features reported by QEMU 11.1 for
// each CPU model.
func TestARM64VariantFromCPUInfo(t *testing.T) {
	tests := []struct {
		cpu      string
		features string
		expected string
	}{
		{cpu: "cortex-a35", features: "fp asimd aes pmull sha1 sha2 crc32 cpuid", expected: "v8"},
		{cpu: "cortex-a53", features: "fp asimd aes pmull sha1 sha2 crc32 cpuid", expected: "v8"},
		{cpu: "cortex-a57", features: "fp asimd aes pmull sha1 sha2 crc32 cpuid", expected: "v8"},
		{cpu: "cortex-a72", features: "fp asimd aes pmull sha1 sha2 crc32 cpuid", expected: "v8"},
		{cpu: "cortex-a55", features: "fp asimd aes pmull sha1 sha2 crc32 atomics fphp asimdhp cpuid asimdrdm lrcpc dcpop asimddp", expected: "v8.2"},
		{cpu: "cortex-a76", features: "fp asimd aes pmull sha1 sha2 crc32 atomics fphp asimdhp cpuid asimdrdm lrcpc dcpop asimddp", expected: "v8.2"},
		{cpu: "cortex-a78ae", features: "fp asimd aes pmull sha1 sha2 crc32 atomics fphp asimdhp cpuid asimdrdm lrcpc dcpop asimddp uscat ilrcpc flagm", expected: "v8.2"},
		{cpu: "neoverse-n1", features: "fp asimd aes pmull sha1 sha2 crc32 atomics fphp asimdhp cpuid asimdrdm lrcpc dcpop asimddp", expected: "v8.2"},
		{cpu: "a64fx", features: "fp asimd aes pmull sha1 sha2 crc32 atomics fphp asimdhp cpuid asimdrdm fcma dcpop sve", expected: "v8.2"},
		{cpu: "neoverse-v1", features: "fp asimd aes pmull sha1 sha2 crc32 atomics fphp asimdhp cpuid asimdrdm jscvt fcma lrcpc dcpop sha3 sm3 sm4 asimddp sha512 sve asimdfhm dit uscat ilrcpc flagm paca pacg dcpodp svei8mm svebf16 i8mm rng", expected: "v8.4"},
		{cpu: "cortex-a710", features: "fp asimd aes pmull sha1 sha2 crc32 atomics fphp asimdhp cpuid asimdrdm jscvt fcma lrcpc dcpop sha3 sm3 sm4 asimddp sha512 sve asimdfhm dit uscat ilrcpc flagm sb paca pacg dcpodp sve2 sveaes svepmull svebitperm svesha3 svesm4 flagm2 frint svei8mm svebf16 i8mm bf16 bti mte", expected: "v9"},
		{cpu: "neoverse-n2", features: "fp asimd aes pmull sha1 sha2 crc32 atomics fphp asimdhp cpuid asimdrdm jscvt fcma lrcpc dcpop sha3 sm3 sm4 asimddp sha512 sve asimdfhm dit uscat ilrcpc flagm sb paca pacg dcpodp sve2 sveaes svepmull svebitperm svesha3 svesm4 flagm2 frint svei8mm svebf16 i8mm bf16 rng bti mte", expected: "v9"},
	}
	for _, tc := range tests {
		t.Run(tc.cpu, func(t *testing.T) {
			var hwcap, hwcap2 uint64
			for f := range strings.FieldsSeq(tc.features) {
				hwcap |= cpuinfoHWCAP[f]
				hwcap2 |= cpuinfoHWCAP2[f]
			}
			assert.Equal(t, arm64VariantFromHWCAP(hwcap, hwcap2), tc.expected)
		})
	}
}

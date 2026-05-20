//go:build windows

package winapi

type SNPPSPGuestRequestResult struct {
	DriverStatus uint32
	PspStatus    uint64
}

//sys SnpPspIsSnpMode(snpMode *uint8) (ret uint32, err error) [failretval>0] = amdsnppspapi.SnpPspIsSnpMode?
//sys SnpPspFetchAttestationReport(reportData *uint8, guestRequestResult *SNPPSPGuestRequestResult, report *uint8) (ret uint32, err error) [failretval>0] = amdsnppspapi.SnpPspFetchAttestationReport?

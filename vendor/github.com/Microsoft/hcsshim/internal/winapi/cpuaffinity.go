package winapi

// BOOL GetProcessGroupAffinity(
//     [in]      HANDLE  hProcess,
//     [in, out] PUSHORT GroupCount,
//     [out]     PUSHORT GroupArray
// );
//
//sys GetProcessGroupAffinity(process windows.Handle, groupCount *uint16, groupArray *uint16) (err error) = kernel32.GetProcessGroupAffinity

// BOOL GetProcessAffinityMask(
//     [in]  HANDLE     hProcess,
//     [out] PDWORD_PTR lpProcessAffinityMask,
//     [out] PDWORD_PTR lpSystemAffinityMask
// );
//
//sys GetProcessAffinityMask(process windows.Handle, processAffinityMask *uintptr, systemAffinityMask *uintptr) (err error) = kernel32.GetProcessAffinityMask

// WORD GetActiveProcessorGroupCount();
//
//sys GetActiveProcessorGroupCount() (amount uint16) = kernel32.GetActiveProcessorGroupCount

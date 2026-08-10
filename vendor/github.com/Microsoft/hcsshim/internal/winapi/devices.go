//go:build windows

package winapi

import "github.com/Microsoft/go-winio/pkg/guid"

//sys CMGetDeviceInterfaceListSize(listlen *uint32, classGUID *GUID, deviceID *uint16, ulFlags uint32) (hr error) = cfgmgr32.CM_Get_Device_Interface_List_SizeW
//sys CMGetDeviceInterfaceList(classGUID *GUID, deviceID *uint16, buffer *uint16, bufLen uint32, ulFlags uint32) (hr error) = cfgmgr32.CM_Get_Device_Interface_ListW
//sys CMGetDeviceIDListSize(pulLen *uint32, pszFilter *byte, uFlags uint32) (hr error) = cfgmgr32.CM_Get_Device_ID_List_SizeA
//sys CMGetDeviceIDList(pszFilter *byte, buffer *byte, bufferLen uint32, uFlags uint32) (hr error)= cfgmgr32.CM_Get_Device_ID_ListA
//sys CMLocateDevNode(pdnDevInst *uint32, pDeviceID string, uFlags uint32) (hr error) = cfgmgr32.CM_Locate_DevNodeW
//sys CMGetDevNodeProperty(dnDevInst uint32, propertyKey *DevPropKey, propertyType *uint32, propertyBuffer *uint16, propertyBufferSize *uint32, uFlags uint32) (hr error) = cfgmgr32.CM_Get_DevNode_PropertyW

type GUID = guid.GUID

type DevPropKey struct {
	Fmtid guid.GUID
	Pid   uint32
}

package image

import (
	"sync"
)

type RefCount int

var (
	iSR = &imageStateMgr{
		imgIDMap: make(map[ID]RefCount),
	}
)

type imageStateMgr struct {
	sync.Mutex
	imgIDMap map[ID]RefCount
}

func (ism *imageStateMgr) updateIDRefcount(id ID, inc bool) {
	ism.Lock()
	defer ism.Unlock()

	if !inc {
		ism.imgIDMap[id] -= 1
		if ism.imgIDMap[id] <= 0 {
			delete(ism.imgIDMap, id)
		}
		return
	}

	ism.imgIDMap[id] += 1
}

func (ism *imageStateMgr) isImageIDInSavingInProcess(id ID) bool {
	ism.Lock()
	defer ism.Unlock()
	if _, exist := ism.imgIDMap[id]; !exist {
		return false
	}

	return !(ism.imgIDMap[id] == 0)
}

func UpdateImageIDSavingStatus(id ID, startSavingImage bool) {
	iSR.updateIDRefcount(id, startSavingImage)
}

func IsImageInSavingInProcess(id ID) bool {
	return iSR.isImageIDInSavingInProcess(id)
}

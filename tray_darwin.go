// tray_darwin.go
package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

void smpxTrayCreate(const void *png, int length);
void smpxTrayAddItem(int tag, const char *title);
void smpxTraySetTooltip(const char *tooltip);
void smpxSetAccessory(void);
*/
import "C"

import (
	"sync"
	"unsafe"
)

// macOS: 창을 닫으면 메뉴바로 숨김 (메뉴바 아이콘 / 재실행으로 다시 열기)
const trayHidesWindowOnClose = true

var (
	trayMu      sync.Mutex
	trayHandler func(action string)
)

// startTray 메뉴바 아이콘 생성
//   - NSApplication delegate 를 바꾸지 않도록 NSStatusItem 을 직접 사용 (Wails 와 충돌 방지)
func startTray(handler func(action string)) {
	trayMu.Lock()
	trayHandler = handler
	trayMu.Unlock()

	C.smpxTrayCreate(unsafe.Pointer(&trayTemplateIcon[0]), C.int(len(trayTemplateIcon)))

	for i, item := range trayItems {
		title := C.CString(item.Title)
		C.smpxTrayAddItem(C.int(i), title)
		C.free(unsafe.Pointer(title))
	}
}

func setTrayTooltip(tooltip string) {
	s := C.CString(tooltip)
	defer C.free(unsafe.Pointer(s))
	C.smpxTraySetTooltip(s)
}

// setVideoWindowAccessory 영상 창 프로세스는 Dock 아이콘 없이 표시
func setVideoWindowAccessory() {
	C.smpxSetAccessory()
}

//export smpxTrayAction
func smpxTrayAction(tag C.int) {
	trayMu.Lock()
	h := trayHandler
	trayMu.Unlock()

	if i := int(tag); h != nil && i >= 0 && i < len(trayItems) {
		go h(trayItems[i].Action)
	}
}

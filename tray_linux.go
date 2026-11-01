// tray_linux.go
package main

import "github.com/energye/systray"

// Linux: 트레이를 지원하지 않는 데스크톱(기본 GNOME 등)이 있으므로 창을 닫으면 종료
const trayHidesWindowOnClose = false

// startTray StatusNotifierItem(DBus) 트레이 아이콘, GTK 메인 루프와 별도로 동작
func startTray(handler func(action string)) {
	start, _ := systray.RunWithExternalLoop(func() {
		systray.SetIcon(trayColorIcon)
		systray.SetTitle("SMPX")
		systray.SetTooltip("SMPX")

		for _, item := range trayItems {
			if item.Action == "" {
				systray.AddSeparator()
				continue
			}
			action := item.Action
			systray.AddMenuItem(item.Title, "").Click(func() {
				go handler(action)
			})
		}
	}, nil)

	start()
}

func setTrayTooltip(tooltip string) {
	systray.SetTooltip(tooltip)
}

func setVideoWindowAccessory() {}

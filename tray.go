// tray.go
package main

import _ "embed"

// 트레이(메뉴바) 메뉴 동작
const (
	trayPlayPause = "playpause"
	trayPrevious  = "prev"
	trayNext      = "next"
	trayShow      = "show"
	trayQuit      = "quit"
)

// macOS 메뉴바용 템플릿 아이콘 (검정 + 알파, 다크 모드에서 자동 반전)
//
//go:embed build/tray/tray_template.png
var trayTemplateIcon []byte

// Linux 트레이용 컬러 아이콘
//
//go:embed build/tray/tray.png
var trayColorIcon []byte

// trayItem 메뉴 항목
type trayItem struct {
	Action string
	Title  string
}

var trayItems = []trayItem{
	{trayPlayPause, "▶︎ 재생 / 일시정지"},
	{trayPrevious, "⏮ 이전"},
	{trayNext, "⏭ 다음"},
	{"", ""}, // 구분선
	{trayShow, "SMPX 열기"},
	{trayQuit, "종료"},
}

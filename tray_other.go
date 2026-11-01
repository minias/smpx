// tray_other.go
//go:build !darwin && !linux

package main

const trayHidesWindowOnClose = false

func startTray(func(action string)) {}

func setTrayTooltip(string) {}

func setVideoWindowAccessory() {}

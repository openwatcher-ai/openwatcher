//go:build !darwin || !cgo

package main

func (a *App) installStatusItem() {}

func (a *App) refreshStatusItem() {}

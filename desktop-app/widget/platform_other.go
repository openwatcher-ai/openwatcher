//go:build !darwin && !windows

package main

import "context"

func setWidgetPlatform(context.Context) {}

//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework Cocoa -framework UniformTypeIdentifiers
#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>
typedef struct { double x,y,w,h; unsigned int displayID; } OWRect;

static void ow_on_main(dispatch_block_t block) {
  if ([NSThread isMainThread]) { block(); } else { dispatch_sync(dispatch_get_main_queue(), block); }
}
static NSWindow *ow_widget_window(void) {
  for (NSWindow *window in [[NSApplication sharedApplication] windows]) {
    if ([[window title] isEqualToString:@"OpenWatcher Widget"]) return window;
  }
  NSWindow *window=[[NSApplication sharedApplication] keyWindow];
  if (window == nil) window=[[NSApplication sharedApplication] mainWindow];
  return window;
}
static double ow_desktop_top(void) {
  NSArray<NSScreen *> *screens=[NSScreen screens];
  return [screens count] == 0 ? 0 : NSMaxY([[screens objectAtIndex:0] frame]);
}
static void ow_widget_accessory(void) {
  ow_on_main(^{ [[NSApplication sharedApplication] setActivationPolicy:NSApplicationActivationPolicyAccessory]; });
}
static int ow_screens(OWRect *out, int max) {
  __block int n=0;
  ow_on_main(^{
    NSArray<NSScreen *> *screens=[NSScreen screens]; n=(int)MIN((NSInteger)max,[screens count]);
    double top=ow_desktop_top();
    for(int i=0;i<n;i++){
      NSScreen *screen=[screens objectAtIndex:i]; NSRect r=[screen visibleFrame];
      NSNumber *number=[[screen deviceDescription] objectForKey:@"NSScreenNumber"];
      out[i]=(OWRect){r.origin.x,top-NSMaxY(r),r.size.width,r.size.height,[number unsignedIntValue]};
    }
  });
  return n;
}
static int ow_window_rect(OWRect *out) {
  __block int found=0;
  ow_on_main(^{
    NSWindow *window=ow_widget_window();
    if (window != nil) {
      NSRect r=[window frame]; double top=ow_desktop_top();
      *out=(OWRect){r.origin.x,top-NSMaxY(r),r.size.width,r.size.height,0}; found=1;
    }
  });
  return found;
}
static void ow_apply_rect(OWRect value) {
  ow_on_main(^{
    NSWindow *window=ow_widget_window();
    if (window == nil) return;
    double top=ow_desktop_top();
    NSRect r=NSMakeRect(value.x,top-value.y-value.h,value.w,value.h);
    [window setFrame:r display:YES animate:NO];
  });
}
static void ow_open_main(void) {
  ow_on_main(^{
    for (NSRunningApplication *app in [[NSWorkspace sharedWorkspace] runningApplications]) {
      if ([[app localizedName] isEqualToString:@"OpenWatcher"] && app.activationPolicy != NSApplicationActivationPolicyAccessory) { [app activateWithOptions:NSApplicationActivateAllWindows]; return; }
    }
  });
}
*/
import "C"
import (
	"context"
	"fmt"
	"openwatcher/desktop-app/widget/internal/widgetwindow"
)

func setWidgetPlatform(context.Context) { C.ow_widget_accessory() }
func nativeWorkAreas() []widgetwindow.WorkArea {
	var rects [16]C.OWRect
	n := int(C.ow_screens(&rects[0], 16))
	out := make([]widgetwindow.WorkArea, 0, n)
	for i := 0; i < n; i++ {
		r := rects[i]
		out = append(out, widgetwindow.WorkArea{X: float64(r.x), Y: float64(r.y), Width: float64(r.w), Height: float64(r.h), MonitorID: fmt.Sprintf("display-%d", uint32(r.displayID))})
	}
	return out
}
func nativeWindowGeometry(context.Context) widgetwindow.Geometry {
	var rect C.OWRect
	if C.ow_window_rect(&rect) == 0 {
		return widgetwindow.DefaultGeometry()
	}
	return widgetwindow.Geometry{X: float64(rect.x), Y: float64(rect.y), Width: float64(rect.w), Height: float64(rect.h)}
}
func nativeApplyGeometry(_ context.Context, g widgetwindow.Geometry) {
	C.ow_apply_rect(C.OWRect{x: C.double(g.X), y: C.double(g.Y), w: C.double(g.Width), h: C.double(g.Height)})
}
func nativeOpenMainApp() { C.ow_open_main() }

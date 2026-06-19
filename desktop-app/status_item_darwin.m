#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>

extern void openwatcherStatusShow(void);
extern void openwatcherStatusStartBackend(void);
extern void openwatcherStatusStopBackend(void);
extern void openwatcherStatusStartDeveloper(void);
extern void openwatcherStatusStopDeveloper(void);
extern void openwatcherStatusStartDeveloperTunnel(void);
extern void openwatcherStatusStopDeveloperTunnel(void);
extern void openwatcherStatusRefresh(void);
extern void openwatcherStatusQuit(void);

@interface OpenWatcherStatusMenuTarget : NSObject <NSMenuDelegate>
- (void)show:(id)sender;
- (void)startBackend:(id)sender;
- (void)stopBackend:(id)sender;
- (void)startDeveloper:(id)sender;
- (void)stopDeveloper:(id)sender;
- (void)startDeveloperTunnel:(id)sender;
- (void)stopDeveloperTunnel:(id)sender;
- (void)quit:(id)sender;
@end

static NSStatusItem *openwatcherStatusItem;
static OpenWatcherStatusMenuTarget *openwatcherStatusTarget;
static NSMenuItem *openwatcherBackendStatusItem;
static NSMenuItem *openwatcherStartBackendItem;
static NSMenuItem *openwatcherStopBackendItem;
static NSMenuItem *openwatcherDeveloperStatusItem;
static NSMenuItem *openwatcherStartDeveloperItem;
static NSMenuItem *openwatcherStopDeveloperItem;
static NSMenuItem *openwatcherDeveloperTunnelStatusItem;
static NSMenuItem *openwatcherStartDeveloperTunnelItem;
static NSMenuItem *openwatcherStopDeveloperTunnelItem;

@implementation OpenWatcherStatusMenuTarget

- (void)menuWillOpen:(NSMenu *)menu {
    openwatcherStatusRefresh();
}

- (void)show:(id)sender {
    openwatcherStatusShow();
}

- (void)startBackend:(id)sender {
    openwatcherStatusStartBackend();
}

- (void)stopBackend:(id)sender {
    openwatcherStatusStopBackend();
}

- (void)startDeveloper:(id)sender {
    openwatcherStatusStartDeveloper();
}

- (void)stopDeveloper:(id)sender {
    openwatcherStatusStopDeveloper();
}

- (void)startDeveloperTunnel:(id)sender {
    openwatcherStatusStartDeveloperTunnel();
}

- (void)stopDeveloperTunnel:(id)sender {
    openwatcherStatusStopDeveloperTunnel();
}

- (void)quit:(id)sender {
    openwatcherStatusQuit();
}

@end

static NSMenuItem *OpenWatcherActionItem(NSString *title, SEL action) {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:@""];
    item.target = openwatcherStatusTarget;
    return item;
}

static void OpenWatcherConfigureStatusButton(void) {
    NSStatusBarButton *button = openwatcherStatusItem.button;
    if (button == nil) {
        return;
    }
    openwatcherStatusItem.length = 116.0;
    button.toolTip = @"OpenWatcher";
    NSImage *image = [NSImage imageNamed:@"iconfile"];
    if (image == nil) {
        image = [NSImage imageNamed:@"iconfile.icns"];
    }
    if (image != nil) {
        image.size = NSMakeSize(18.0, 18.0);
        image.template = NO;
        button.image = image;
        button.imagePosition = NSImageLeft;
        button.title = @" OpenWatcher";
        return;
    }
    button.title = @"OpenWatcher";
}

static void OpenWatcherInstallStatusItemOnMain(void) {
    if (openwatcherStatusItem != nil) {
        return;
    }
    openwatcherStatusTarget = [[OpenWatcherStatusMenuTarget alloc] init];
    openwatcherStatusItem = [[[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength] retain];
    OpenWatcherConfigureStatusButton();

    NSMenu *menu = [[NSMenu alloc] initWithTitle:@"OpenWatcher"];
    menu.delegate = openwatcherStatusTarget;

    [menu addItem:OpenWatcherActionItem(@"显示 OpenWatcher", @selector(show:))];
    [menu addItem:[NSMenuItem separatorItem]];

    openwatcherBackendStatusItem = [[NSMenuItem alloc] initWithTitle:@"本机服务：检查中" action:nil keyEquivalent:@""];
    openwatcherBackendStatusItem.enabled = NO;
    [menu addItem:openwatcherBackendStatusItem];
    openwatcherStartBackendItem = OpenWatcherActionItem(@"启动本机服务", @selector(startBackend:));
    [menu addItem:openwatcherStartBackendItem];
    openwatcherStopBackendItem = OpenWatcherActionItem(@"停止本机服务", @selector(stopBackend:));
    [menu addItem:openwatcherStopBackendItem];
    [menu addItem:[NSMenuItem separatorItem]];

    openwatcherDeveloperStatusItem = [[NSMenuItem alloc] initWithTitle:@"开发环境：检查中" action:nil keyEquivalent:@""];
    openwatcherDeveloperStatusItem.enabled = NO;
    [menu addItem:openwatcherDeveloperStatusItem];
    openwatcherStartDeveloperItem = OpenWatcherActionItem(@"启动开发环境", @selector(startDeveloper:));
    [menu addItem:openwatcherStartDeveloperItem];
    openwatcherStopDeveloperItem = OpenWatcherActionItem(@"停止开发环境", @selector(stopDeveloper:));
    [menu addItem:openwatcherStopDeveloperItem];
    [menu addItem:[NSMenuItem separatorItem]];

    openwatcherDeveloperTunnelStatusItem = [[NSMenuItem alloc] initWithTitle:@"开发隧道：检查中" action:nil keyEquivalent:@""];
    openwatcherDeveloperTunnelStatusItem.enabled = NO;
    [menu addItem:openwatcherDeveloperTunnelStatusItem];
    openwatcherStartDeveloperTunnelItem = OpenWatcherActionItem(@"启动开发隧道", @selector(startDeveloperTunnel:));
    [menu addItem:openwatcherStartDeveloperTunnelItem];
    openwatcherStopDeveloperTunnelItem = OpenWatcherActionItem(@"停止开发隧道", @selector(stopDeveloperTunnel:));
    [menu addItem:openwatcherStopDeveloperTunnelItem];
    [menu addItem:[NSMenuItem separatorItem]];

    [menu addItem:OpenWatcherActionItem(@"退出 OpenWatcher", @selector(quit:))];
    openwatcherStatusItem.menu = menu;
}

void OpenWatcherInstallStatusItem(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        OpenWatcherInstallStatusItemOnMain();
    });
}

void OpenWatcherRefreshStatusItem(int serviceRunning, int developerRunning, int developerTunnelRunning) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (openwatcherStatusItem == nil) {
            return;
        }
        openwatcherBackendStatusItem.title = serviceRunning ? @"本机服务：运行中" : @"本机服务：未启动";
        openwatcherStartBackendItem.enabled = !serviceRunning;
        openwatcherStopBackendItem.enabled = serviceRunning;

        openwatcherDeveloperStatusItem.title = developerRunning ? @"开发环境：运行中" : @"开发环境：未启动";
        openwatcherStartDeveloperItem.enabled = !developerRunning;
        openwatcherStopDeveloperItem.enabled = developerRunning;

        openwatcherDeveloperTunnelStatusItem.title = developerTunnelRunning ? @"开发隧道：运行中" : @"开发隧道：未启动";
        openwatcherStartDeveloperTunnelItem.enabled = !developerTunnelRunning;
        openwatcherStopDeveloperTunnelItem.enabled = developerTunnelRunning;
    });
}

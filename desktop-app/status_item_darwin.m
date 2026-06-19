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

static NSString *OpenWatcherStatusIconBase64(void) {
    return @"iVBORw0KGgoAAAANSUhEUgAAACQAAAAkCAYAAADhAJiYAAADkklEQVR4nOyXW4hVVRzGf6czWnkKu2tWkihZ"
           @"YYVlJN2lspKwGhC6EORzEr1FT93opfBlpJ6qgXzqYkTZFCZEF4PULmSaEOGEGdaI1Fyb05zzxYLvwGK1z977zBD"
           @"Nw/lgs/da///s75v//l/W6ZHEbMIJ/7eAFF1BRegKKkJPh/6LgQ3A5UAd+BJ4Gzhu+5nAetvPAA4C7wA/lGYIZV"
           @"/iWiDpXbXHZkm9kg5KOippIrHvkLSkDFcZMSslNXLEtHBYUp+kXyT9mWEPe2uL+CoFjfEU4BhwYsmA7wN2A73A"
           @"6Rn2o8BK3zNRlNRbMsS86jy5H/g4sV0GXARcC1wAPA6M2la3yGdyGQtCmOKmyFb1fWPi03DO1Gy/RNKQpHFJI5J"
           @"GJc2fTg7dkRD1R7abE9+7Ir9A+pukDySdavtzFhJsdUkPtOPN+2SLk/U2358EpoD5QMV724HrgCPABPAXsBx4C6"
           @"gB34aPAVSBJnBWO9K8PjQMDPrb/+1eE/JnroWcBnwHHDbZF8DdwMsW0QAWAs8DX/udTd+PT0fQp8CHwBDwM/Aoc"
           @"DXwB3AucDLwicW08BWwEXgROMnRmgTW+o7/oW/ashYk9WpJl0p6P8mnQ5KWJQneE61DIr/p3Nkk6YikQfeonTNp"
           @"jIHoo4xquzLyCb1sn7t0NdpfKOlhSfslfS/pgKSfJN2ex1k0yx4Ebk32wnokWod8WhE9t7AEeMSfreL0eCPKp0"
           @"wUNcYXkvV9wF7gTuAW7zWdxDVXX8BVQJ+fp5zgoSCeBRblEeZF6GLgnGi9A9gJrAOWWlDV+3X7hEhcAbwURSv4"
           @"vOauX3H1XugK/hfyInR9sn7dx4rW8WIZ8IqjNeUrjI3HXJnjjl4/sBU43/alHisdR2hOsm56dq22qHneH3DU9ro"
           @"t7DJ5iMJn7ks32Iajvqsta07G9yaV9V5k25xRefd6fi2XtEHSusj/CUkDkrZL2pPMxNJlPyeDNCbZmmF/yKV/d"
           @"uS3yoP1mKRf3Ysq0+1DW0w05is0t6clrZe0wlM9xTZJa9yrnsqw983kgFbzjDrPlRTONvuBzz1sBz0+bsx7SYRh"
           @"59dIO4eiPjQG3AP86AE76WRe5ORe4Ea5u4SYRkZT7VhQwCGXdr+jNM8RC0fR2zzRrwE25bxjwH+zp4is6JOlCE"
           @"1ujc/aIXoHfAZqYa7FrfLzkM9Kv5cl6FTQf45Z98u1K6gIXUFF+CcAAP//+wLLKID1wCEAAAAASUVORK5CYII=";
}

static NSImage *OpenWatcherStatusTemplateImage(void) {
    NSData *data = [[[NSData alloc] initWithBase64EncodedString:OpenWatcherStatusIconBase64() options:0] autorelease];
    NSImage *embeddedImage = nil;
    if (data != nil) {
        embeddedImage = [[[NSImage alloc] initWithData:data] autorelease];
    }
    if (embeddedImage != nil) {
        embeddedImage.size = NSMakeSize(18.0, 18.0);
        embeddedImage.template = YES;
        return embeddedImage;
    }
    NSImage *image = [NSImage imageNamed:@"iconfile"];
    if (image == nil) {
        image = [NSImage imageNamed:@"iconfile.icns"];
    }
    if (image != nil) {
        image.size = NSMakeSize(18.0, 18.0);
        image.template = YES;
    }
    return image;
}

static void OpenWatcherConfigureStatusButton(void) {
    NSStatusBarButton *button = openwatcherStatusItem.button;
    if (button == nil) {
        return;
    }
    openwatcherStatusItem.length = NSSquareStatusItemLength;
    button.toolTip = @"OpenWatcher";
    [button setAccessibilityLabel:@"OpenWatcher"];
    button.title = @"";
    button.image = nil;
    NSImage *image = OpenWatcherStatusTemplateImage();
    if (image != nil) {
        button.image = image;
        button.imagePosition = NSImageOnly;
        return;
    }
    openwatcherStatusItem.length = NSVariableStatusItemLength;
    button.title = @"OW";
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

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
    return @"iVBORw0KGgoAAAANSUhEUgAAACwAAAAsCAYAAAAehFoBAAAEpklEQVR4nOyYS2xVVRSGv7agaAsCVfEVNCKm"
           @"DUGwAfEBIr5RSXwRQxwZjNHEBxPjyIExDnSkA2OciBFiYkw0RiViImpERTRReUlBpCiFFmux2AelXH6zk/8mO8"
           @"fTe3fLrUron+z0nLPXvvs/a6/1r3U6RhInEqr/awJDxSjhkcYo4ZHGSUe4CrgYaPB1KTQCs4Ga49ox6PAwxt2S"
           @"Nuqf+ErSXZFduF4laY+kQ5IGJB2VtEHSw8PZe6gLGiRtyyGaxRZJyyRtl/SbpA5J/Tl2uyRdOVKEFyQQzWK5Sbd"
           @"J6ilhd3OlCZ8/DLJFPChph6Q/HRJ56PPpleVSldhLfApcl/N8G/CeE24pMC3HpgtY4etJTtCHovkj/rseuKESSX"
           @"fZIF65I8d28SC2eyXNiexmSmp1Ah52uATv31KJkFiZQ2BxND9BUl103+gjjhFI/SKpKbILL9Brsod8/XolCH+T2"
           @"fz7TGzfK6kqs2a6pAO277VK7JPULGluZLdGUrdHn9WlJJ+UwnFJ5n6t/17k2PwVmJwpCDuBucBPQL9HiNWxwGqg"
           @"yXYbXLyqnQcTypFJIdwC/A60A/uAPcAFwCvAKcA1wBxgKjAxqnjB7iZgs8kORONF248LaeRRAP4qR2ZMAuEPgB"
           @"nAYaDXHl1lb10KnAWcYy+/482LaAWW+eWm29MFzy21KhyN1uyqBOFPgIP2cIs3D/LVEx13H/C+r7NotYy95L6j"
           @"y6HQ73DrM+HA5fNyZFJ0uNoe7rZWnpeZ/xG4EeiIntU7NOJnIVye9bF/AbwA1ALHHFrB01cDbSXZJFa601ytstg"
           @"vaXxOVeyyrl6YmRtnSQxN0iZJmyVtdU/xWAqX1PbyecdgjOCp64G6zPMznO1jHNcxQqI9Y48OOBkLDru3U4ikh"
           @"MS5Vocspvn4ngTeBTZFc4scEusyZN9yshYctzVe97hD7Q+gsxSZFA8/mvPsVpO91puHE7jCc4HoZx41EdnV9mx"
           @"VtHeQvCecwD+7yS+JFMKLMvehKHxtSZrvZiYc76smXcz4Gr/M2cBr1txjHoH0D3ZGt70+0S9UEimydlXmfp03C"
           @"RI1zzF8pjP+Q7/gFtuG7uwpa/cE39eZ7AqvCxXzdF83uTM8LsJ5a4KXXrZHgxSdGh3/ty7LoQjcB3wHTIlGkLq"
           @"nrcOzoxOa5ApaEilJ1+yKVsQBbxww3v1wNmz63D9vt+emukiEI1/peVyEJkd9RCgyy0uyGWZ7+UA0XytpZ45Nw"
           @"V8R1ZKmSJonaWy0bqGkdn8jBj3eLem5SrSXM3PIhHZxSWQzSVLnIM37ApOOW9DL3bwXUfx0mlGOT+onUjj229w"
           @"AFRx/ndbQte7MQsxudQJlsd4yF9bOAu7MsVkD3F6OSCrhEIcbXcWOOOnaHKOB9Mf+O9Hka1N+NINQOPaXM0otz"
           @"R0uFu1uUnqst7V+mVlOzINOrrJ9bQbzU8gOhXDADmCh++Nqy1ide+FG63WDy2soFm8m/OZur/kylcRQ/7cWvjw"
           @"eAe5xXLdYP+sdn/WuWiHW73fD9AawN/qNIGkfAUtcfJqHQiA1hv83OOn+3fqvY5TwSGOU8EjjhCP8dwAAAP//"
           @"sJCfzgU3vqgAAAAASUVORK5CYII=";
}

static NSImage *OpenWatcherStatusTemplateImage(void) {
    NSData *data = [[[NSData alloc] initWithBase64EncodedString:OpenWatcherStatusIconBase64() options:0] autorelease];
    NSImage *embeddedImage = nil;
    if (data != nil) {
        embeddedImage = [[[NSImage alloc] initWithData:data] autorelease];
    }
    if (embeddedImage != nil) {
        embeddedImage.size = NSMakeSize(22.0, 22.0);
        embeddedImage.template = YES;
        return embeddedImage;
    }
    NSImage *image = [NSImage imageNamed:@"iconfile"];
    if (image == nil) {
        image = [NSImage imageNamed:@"iconfile.icns"];
    }
    if (image != nil) {
        image.size = NSMakeSize(22.0, 22.0);
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

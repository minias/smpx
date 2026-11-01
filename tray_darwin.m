// tray_darwin.m
#import <Cocoa/Cocoa.h>

extern void smpxTrayAction(int tag);

@interface SmpxTrayTarget : NSObject
- (void)select:(NSMenuItem *)sender;
@end

@implementation SmpxTrayTarget
- (void)select:(NSMenuItem *)sender {
    smpxTrayAction((int)sender.tag);
}
@end

static NSStatusItem *statusItem;
static NSMenu *trayMenu;
static SmpxTrayTarget *trayTarget;

// 모든 UI 작업은 메인 스레드에서 (호출 순서 유지)
static void onMain(dispatch_block_t block) {
    dispatch_async(dispatch_get_main_queue(), block);
}

void smpxTrayCreate(const void *png, int length) {
    NSData *data = [NSData dataWithBytes:png length:length];

    onMain(^{
        trayTarget = [SmpxTrayTarget new];
        trayMenu = [NSMenu new];
        trayMenu.autoenablesItems = NO;

        NSImage *image = [[NSImage alloc] initWithData:data];
        image.size = NSMakeSize(18, 18);
        image.template = YES;

        statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
        statusItem.button.image = image;
        statusItem.button.toolTip = @"SMPX";
        statusItem.menu = trayMenu;
    });
}

void smpxTrayAddItem(int tag, const char *title) {
    NSString *text = [NSString stringWithUTF8String:title];

    onMain(^{
        if (text.length == 0) {
            [trayMenu addItem:[NSMenuItem separatorItem]];
            return;
        }
        NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:text action:@selector(select:) keyEquivalent:@""];
        item.target = trayTarget;
        item.tag = tag;
        [trayMenu addItem:item];
    });
}

void smpxTraySetTooltip(const char *tooltip) {
    NSString *text = [NSString stringWithUTF8String:tooltip];

    onMain(^{
        statusItem.button.toolTip = text;
    });
}

void smpxSetAccessory(void) {
    onMain(^{
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
        [NSApp activateIgnoringOtherApps:YES];
    });
}

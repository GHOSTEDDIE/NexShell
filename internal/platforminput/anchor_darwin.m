//go:build darwin && !ci
#import <Cocoa/Cocoa.h>
#include <stdint.h>
@interface NSView (NexShellInput)
- (void)setNexShellInputRect:(NSRect)rect;
- (void)unmarkText;
@end
void nexInputRect(uintptr_t handle,double x,double y,double width,double height,double canvasW,double canvasH,int reset){
    if (!handle || canvasW<=0 || canvasH<=0) return;
    // Native Cocoa view operations always run on the AppKit main thread.
    void (^update)(void) = ^{
        NSWindow* window=(NSWindow*)handle;
        NSView* view=[window contentView];
        if (![view respondsToSelector:@selector(setNexShellInputRect:)]) return;
        if (reset) {[[view inputContext] discardMarkedText];[view unmarkText];}
        NSSize bounds=[view bounds].size;
        CGFloat sx=bounds.width/canvasW,sy=bounds.height/canvasH;
        NSRect rect=NSMakeRect(x*sx,y*sy,MAX(width*sx,1),MAX(height*sy,1));
        if (![view isFlipped]) rect.origin.y=bounds.height-rect.origin.y-rect.size.height;
        [view setNexShellInputRect:rect];
    };
    if ([NSThread isMainThread]) update();
    else dispatch_sync(dispatch_get_main_queue(), update);
}

int nexReducedMotion(void){return [[NSWorkspace sharedWorkspace] accessibilityDisplayShouldReduceMotion] ? 1 : 0;}

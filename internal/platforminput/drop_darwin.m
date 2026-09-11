//go:build darwin && !ci
#import <Cocoa/Cocoa.h>
#include <stdint.h>
int nexDropPosition(uintptr_t handle, double* x, double* y) {
    if (!handle) return 0;
    __block int ok = 0;
    void (^read)(void) = ^{
        NSWindow* window = (NSWindow*)handle;
        NSView* view = window.contentView;
        NSPoint point = [view convertPoint:window.mouseLocationOutsideOfEventStream fromView:nil];
        NSRect bounds = view.bounds;
        if (bounds.size.width <= 0 || bounds.size.height <= 0) return;
        *x = (point.x - bounds.origin.x) / bounds.size.width;
        *y = (point.y - bounds.origin.y) / bounds.size.height;
        if (!view.isFlipped) *y = 1 - *y;
        ok = 1;
    };
    if ([NSThread isMainThread]) read(); else dispatch_sync(dispatch_get_main_queue(), read);
    return ok;
}

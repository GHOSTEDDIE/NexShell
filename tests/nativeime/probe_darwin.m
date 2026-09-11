//go:build darwin && !ci
#import <Cocoa/Cocoa.h>
#include <stdint.h>
@interface NSView (InputProbe)
- (void)setNexShellInputRect:(NSRect)rect;
@end
int imeProbe(uintptr_t handle){
 NSWindow* window=(NSWindow*)handle;
 NSView<NSTextInputClient>* client=(id)[window contentView];
 [client setMarkedText:@"ni" selectedRange:NSMakeRange(1,1) replacementRange:NSMakeRange(NSNotFound,0)];
 int failure=0;
 if (![client hasMarkedText]) failure|=1;
 if ([client markedRange].length!=2) failure|=2;
 if ([client selectedRange].location!=1) failure|=4;
 if (![client respondsToSelector:@selector(setNexShellInputRect:)]) failure|=8;
 else {
 [client setNexShellInputRect:NSMakeRect(20,30,1,20)];
 NSRect expected=[window convertRectToScreen:[client convertRect:NSMakeRect(20,30,1,20) toView:nil]];
 NSRect actual=[client firstRectForCharacterRange:NSMakeRange(0,1) actualRange:NULL];
 if (!NSEqualRects(actual,expected)) failure|=16;
 }
 // Events are injected into this isolated hidden test view, never posted to
 // the OS or any user's window. Composing Return must not reach GLFW's client.
 NSEvent* enter=[NSEvent keyEventWithType:NSEventTypeKeyDown location:NSZeroPoint modifierFlags:0 timestamp:0 windowNumber:[window windowNumber] context:nil characters:@"\r" charactersIgnoringModifiers:@"\r" isARepeat:NO keyCode:36];
 [client keyDown:enter];
 [client insertText:@"你好" replacementRange:NSMakeRange(NSNotFound,0)];
 if ([client hasMarkedText]) failure|=32;
 [client unmarkText];
 [client keyDown:enter];
 return failure;
}

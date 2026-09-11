//go:build darwin && !ci
#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
extern void nexApplicationAction(int terminate);

// Retain GLFW's delegate and all its other callbacks. These two application
// actions must be distinct from GLFW's window-close request on macOS.
@interface NexShellApplicationActions : NSObject
@end
@implementation NexShellApplicationActions
- (BOOL)applicationShouldHandleReopen:(NSApplication*)application hasVisibleWindows:(BOOL)visible {
 nexApplicationAction(0);
 return YES;
}
- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication*)application {
 nexApplicationAction(1);
 return NSTerminateCancel; // Go completes shutdown before quitting the event loop.
}
@end

int nexInstallApplicationActions(void){
 __block int installed=0;
 void (^install)(void)=^{
  id delegate=[NSApp delegate];if(!delegate)return;
  Class target=[delegate class];
  SEL selectors[]={@selector(applicationShouldHandleReopen:hasVisibleWindows:),@selector(applicationShouldTerminate:)};
  for(int i=0;i<2;i++){
   Method method=class_getInstanceMethod([NexShellApplicationActions class],selectors[i]);
   class_replaceMethod(target,selectors[i],method_getImplementation(method),method_getTypeEncoding(method));
  }
  installed=1;
 };
 if([NSThread isMainThread])install();else dispatch_sync(dispatch_get_main_queue(),install);
 return installed;
}

//go:build darwin && !ci
#import <Cocoa/Cocoa.h>
#include <stdlib.h>
#include <string.h>
char* nexClipboardFiles(void) {
 __block char* result = NULL;
 void (^read)(void) = ^{
  @autoreleasepool {
   NSArray* urls = [[NSPasteboard generalPasteboard] readObjectsForClasses:@[[NSURL class]] options:@{NSPasteboardURLReadingFileURLsOnlyKey:@YES}];
   NSMutableArray* paths = [NSMutableArray array];
   for (NSURL* url in urls) { if ([url isFileURL] && [url path]) [paths addObject:[url path]]; }
   if (paths.count) {
    NSData* json = [NSJSONSerialization dataWithJSONObject:paths options:0 error:nil];
    if (json) {result=calloc(json.length+1,1);if(result)memcpy(result,json.bytes,json.length);}
   }
  }
 };
 if ([NSThread isMainThread]) read(); else dispatch_sync(dispatch_get_main_queue(),read);
 return result;
}

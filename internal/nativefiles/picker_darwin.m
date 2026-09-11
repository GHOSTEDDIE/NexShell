//go:build darwin && !ci
#import <Cocoa/Cocoa.h>
#include <stdint.h>
extern void nexFilesSelected(uintptr_t token, const char* data);
static NSMutableDictionary<NSNumber*, NSSavePanel*>* activePanels;

void nexChooseFiles(uintptr_t token, uintptr_t parent, int mode, const char* title, const char* filename, const char* filters) {
    @autoreleasepool {
    NSString* caption = [NSString stringWithUTF8String:title];
    NSString* initial = [NSString stringWithUTF8String:filename];
    NSData* filterData = [[NSString stringWithUTF8String:filters] dataUsingEncoding:NSUTF8StringEncoding];
    NSArray* patterns = [NSJSONSerialization JSONObjectWithData:filterData options:0 error:nil];
    // Copied blocks retain Objective-C values. Native presentation and completion
    // stay on AppKit's main queue; Fyne's event loop remains unblocked.
    dispatch_async(dispatch_get_main_queue(), ^{
        NSSavePanel* panel;
        if (mode == 2) {
            panel = [NSSavePanel savePanel];
        } else {
            NSOpenPanel* open = [NSOpenPanel openPanel];
            open.canChooseFiles = mode != 3;
            open.canChooseDirectories = mode == 3;
            open.allowsMultipleSelection = mode == 1;
            panel = open;
        }
        panel.title = caption;
        panel.message = caption;
        panel.canCreateDirectories = YES;
        if (initial.length) {
            BOOL isDir = NO;
            [[NSFileManager defaultManager] fileExistsAtPath:initial isDirectory:&isDir];
            if (isDir) panel.directoryURL = [NSURL fileURLWithPath:initial isDirectory:YES];
            else {
                NSString* dir = initial.stringByDeletingLastPathComponent;
                if (dir.length) panel.directoryURL = [NSURL fileURLWithPath:dir isDirectory:YES];
                if (mode == 2) panel.nameFieldStringValue = initial.lastPathComponent;
            }
        }
        NSMutableArray* types = [NSMutableArray array];
        if ([patterns isKindOfClass:[NSArray class]]) {
            for (NSString* pattern in patterns) {
                if ([pattern hasPrefix:@"*."]) [types addObject:[pattern substringFromIndex:2]];
            }
        }
        if (types.count) panel.allowedFileTypes = types;
        if (!activePanels) activePanels = [[NSMutableDictionary alloc] init];
        activePanels[@(token)] = panel;
        void (^completion)(NSModalResponse) = ^(NSModalResponse response) {
            NSMutableArray* paths = [NSMutableArray array];
            if (response == NSModalResponseOK) {
                NSArray* urls = mode == 2 ? @[panel.URL] : [(NSOpenPanel*)panel URLs];
                for (NSURL* url in urls) if (url.isFileURL) [paths addObject:url.path];
            }
            NSData* json = paths.count ? [NSJSONSerialization dataWithJSONObject:paths options:0 error:nil] : nil;
            NSString* text = json ? [[NSString alloc] initWithData:json encoding:NSUTF8StringEncoding] : nil;
            nexFilesSelected(token, text ? text.UTF8String : NULL);
            [text release];
            [activePanels removeObjectForKey:@(token)];
        };
        NSWindow* window = parent ? (NSWindow*)parent : nil;
        if (window.isVisible) [panel beginSheetModalForWindow:window completionHandler:completion];
        else [panel beginWithCompletionHandler:completion];
    });
    }
}

void nexCancelFiles(uintptr_t token) {
    dispatch_async(dispatch_get_main_queue(), ^{ [activePanels[@(token)] cancel:nil]; });
}

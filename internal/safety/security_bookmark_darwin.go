//go:build darwin && cgo

package safety

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>
#include <stdlib.h>

typedef struct {
	char *data;
	char *path;
	char *err;
	int stale;
} YemakaBookmarkResult;

typedef struct {
	void *url;
	char *path;
	char *err;
	int stale;
	int started;
} YemakaBookmarkAccess;

static char* yemaka_strdup_nsstring(NSString *value) {
	if (value == nil) {
		return NULL;
	}
	const char *utf8 = [value UTF8String];
	if (utf8 == NULL) {
		return NULL;
	}
	return strdup(utf8);
}

static char* yemaka_error_string(NSError *error, NSString *fallback) {
	if (error != nil && [error localizedDescription] != nil) {
		return yemaka_strdup_nsstring([error localizedDescription]);
	}
	return yemaka_strdup_nsstring(fallback);
}

static YemakaBookmarkResult yemaka_create_security_bookmark(const char *path) {
	YemakaBookmarkResult result;
	result.data = NULL;
	result.path = NULL;
	result.err = NULL;
	result.stale = 0;

	@autoreleasepool {
		NSString *pathString = [NSString stringWithUTF8String:path];
		if (pathString == nil || [pathString length] == 0) {
			result.err = yemaka_strdup_nsstring(@"workspace path is empty");
			return result;
		}

		NSURL *url = [NSURL fileURLWithPath:pathString isDirectory:YES];
		NSError *error = nil;
		NSData *data = [url bookmarkDataWithOptions:NSURLBookmarkCreationWithSecurityScope
					 includingResourceValuesForKeys:nil
									  relativeToURL:nil
											  error:&error];
		if (data == nil) {
			result.err = yemaka_error_string(error, @"create security-scoped bookmark failed");
			return result;
		}

		NSString *encoded = [data base64EncodedStringWithOptions:0];
		result.data = yemaka_strdup_nsstring(encoded);
		result.path = yemaka_strdup_nsstring([url path]);
		return result;
	}
}

static YemakaBookmarkAccess yemaka_start_security_bookmark_access(const char *encoded) {
	YemakaBookmarkAccess result;
	result.url = NULL;
	result.path = NULL;
	result.err = NULL;
	result.stale = 0;
	result.started = 0;

	@autoreleasepool {
		NSString *encodedString = [NSString stringWithUTF8String:encoded];
		if (encodedString == nil || [encodedString length] == 0) {
			result.err = yemaka_strdup_nsstring(@"security bookmark data is empty");
			return result;
		}

		NSData *data = [[NSData alloc] initWithBase64EncodedString:encodedString options:0];
		if (data == nil) {
			result.err = yemaka_strdup_nsstring(@"security bookmark data is invalid base64");
			return result;
		}

		BOOL isStale = NO;
		NSError *error = nil;
		NSURL *url = [NSURL URLByResolvingBookmarkData:data
											   options:NSURLBookmarkResolutionWithSecurityScope
										 relativeToURL:nil
								   bookmarkDataIsStale:&isStale
												 error:&error];
		if (url == nil) {
			result.err = yemaka_error_string(error, @"resolve security-scoped bookmark failed");
			return result;
		}

		result.stale = isStale ? 1 : 0;
		result.started = [url startAccessingSecurityScopedResource] ? 1 : 0;
		result.path = yemaka_strdup_nsstring([url path]);
		result.url = (__bridge_retained void *)url;
		return result;
	}
}

static void yemaka_stop_security_bookmark_access(void *urlPtr, int started) {
	if (urlPtr == NULL) {
		return;
	}
	@autoreleasepool {
		NSURL *url = (__bridge_transfer NSURL *)urlPtr;
		if (started) {
			[url stopAccessingSecurityScopedResource];
		}
	}
}
*/
import "C"

import (
	"fmt"
	"runtime"
	"strings"
	"time"
	"unsafe"
)

func SecurityScopedBookmarksSupported() bool {
	return true
}

func CreateSecurityScopedBookmark(path string) (SecurityBookmark, error) {
	abs, err := ResolveWorkspace(path)
	if err != nil {
		return SecurityBookmark{}, err
	}
	cPath := C.CString(abs)
	defer C.free(unsafe.Pointer(cPath))

	result := C.yemaka_create_security_bookmark(cPath)
	defer freeCString(result.data)
	defer freeCString(result.path)
	defer freeCString(result.err)

	if result.err != nil {
		return SecurityBookmark{}, fmt.Errorf("create security-scoped bookmark: %s", C.GoString(result.err))
	}
	bookmark := strings.TrimSpace(C.GoString(result.data))
	if bookmark == "" {
		return SecurityBookmark{}, fmt.Errorf("create security-scoped bookmark: empty bookmark data")
	}
	resolvedPath := strings.TrimSpace(C.GoString(result.path))
	if resolvedPath == "" {
		resolvedPath = abs
	}
	return SecurityBookmark{
		Data:      bookmark,
		Path:      resolvedPath,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Stale:     result.stale != 0,
	}, nil
}

func StartSecurityScopedBookmarkAccess(data string) (*SecurityBookmarkAccess, error) {
	if err := validateSecurityBookmark(data); err != nil {
		return nil, err
	}
	cData := C.CString(strings.TrimSpace(data))
	defer C.free(unsafe.Pointer(cData))

	result := C.yemaka_start_security_bookmark_access(cData)
	defer freeCString(result.path)
	defer freeCString(result.err)

	if result.err != nil {
		if result.url != nil {
			C.yemaka_stop_security_bookmark_access(result.url, result.started)
		}
		return nil, fmt.Errorf("start security-scoped bookmark access: %s", C.GoString(result.err))
	}
	path := strings.TrimSpace(C.GoString(result.path))
	access := &SecurityBookmarkAccess{
		Path:    path,
		Stale:   result.stale != 0,
		Started: result.started != 0,
	}
	url := result.url
	started := result.started
	access.closeFn = func() {
		C.yemaka_stop_security_bookmark_access(url, started)
	}
	runtime.SetFinalizer(access, func(a *SecurityBookmarkAccess) {
		a.Close()
	})
	return access, nil
}

func freeCString(value *C.char) {
	if value != nil {
		C.free(unsafe.Pointer(value))
	}
}

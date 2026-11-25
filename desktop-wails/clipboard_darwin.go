package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
static int putImage(const void *bytes, size_t size) {
    @autoreleasepool {
        NSData *data = [NSData dataWithBytes:bytes length:size];
        NSPasteboard *board = [NSPasteboard generalPasteboard];
        [board clearContents];
        return [board setData:data forType:NSPasteboardTypePNG];
    }
}
*/
import "C"
import (
	"bytes"
	"encoding/base64"
	"errors"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
	"image/png"
	"unsafe"
)

func (a *App) CopyImage(encoded string) error {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	if _, err = png.DecodeConfig(bytes.NewReader(data)); err != nil {
		return err
	}
	if C.putImage(unsafe.Pointer(&data[0]), C.size_t(len(data))) == 0 {
		return errors.New("无法复制图片")
	}
	return nil
}
func (a *App) CopyTable(text string) error { return wr.ClipboardSetText(a.ctx, text) }

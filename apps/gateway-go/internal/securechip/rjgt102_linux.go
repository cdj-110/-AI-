//go:build linux && cgo && rjgt102

package securechip

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

typedef int (*crypto_init_fn)(void);
typedef void (*crypto_close_fn)(void);
typedef int (*crypto_auth_fn)(void);
typedef int (*crypto_usid_fn)(uint8_t *);

static int rjgt102_authenticate(uint8_t *usid, char *message, size_t message_len) {
    void *library = dlopen("libcryproSdr.so", RTLD_NOW | RTLD_LOCAL);
    if (library == NULL) {
        library = dlopen("/usr/lib/libcryproSdr.so", RTLD_NOW | RTLD_LOCAL);
    }
    if (library == NULL) {
        snprintf(message, message_len, "load libcryproSdr.so failed: %s", dlerror());
        return -1;
    }
    crypto_init_fn init_device = (crypto_init_fn)dlsym(library, "CryptoSdr_Init");
    crypto_close_fn close_device = (crypto_close_fn)dlsym(library, "CryptoSdr_Close");
    crypto_auth_fn authenticate = (crypto_auth_fn)dlsym(library, "CryptoSdr_AuthDev");
    crypto_usid_fn get_usid = (crypto_usid_fn)dlsym(library, "CryptoSdr_GetUsid");
    if (init_device == NULL || close_device == NULL || authenticate == NULL || get_usid == NULL) {
        snprintf(message, message_len, "RJGT102 library is missing required symbols");
        dlclose(library);
        return -2;
    }
    if (init_device() != 0) {
        snprintf(message, message_len, "CryptoSdr_Init failed");
        dlclose(library);
        return -3;
    }
    int result = authenticate();
    if (result == 0) {
        result = get_usid(usid);
    }
    close_device();
    dlclose(library);
    if (result != 0) {
        snprintf(message, message_len, "RJGT102 authentication or USID read failed");
        return -4;
    }
    return 0;
}
*/
import "C"

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"unsafe"
)

var deviceMu sync.Mutex

// Identity is returned only after the vendor authentication challenge has
// succeeded. Calls are serialized because the RJGT102 vendor library is not
// reentrant.
type Identity struct {
	USID string
}

func Authenticate() (Identity, error) {
	deviceMu.Lock()
	defer deviceMu.Unlock()
	var usid [8]C.uint8_t
	var message [256]C.char
	result := C.rjgt102_authenticate(
		(*C.uint8_t)(unsafe.Pointer(&usid[0])),
		(*C.char)(unsafe.Pointer(&message[0])),
		C.size_t(len(message)),
	)
	if result != 0 {
		return Identity{}, fmt.Errorf("%s", C.GoString(&message[0]))
	}
	raw := C.GoBytes(unsafe.Pointer(&usid[0]), 8)
	return Identity{USID: strings.ToUpper(hex.EncodeToString(raw))}, nil
}

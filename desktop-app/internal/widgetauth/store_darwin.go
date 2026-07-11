//go:build darwin && cgo

package widgetauth

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>
static CFStringRef cfstr(const char *s) { return CFStringCreateWithCString(NULL, s, kCFStringEncodingUTF8); }
static OSStatus widget_read(const char *service, const char *account, char **out, CFIndex *out_len) {
  OSStatus st = errSecSuccess;
  CFStringRef svc = NULL, acc = NULL;
  CFMutableDictionaryRef query = NULL;
  CFTypeRef result = NULL;
  *out = NULL;
  *out_len = 0;
  svc = cfstr(service);
  acc = cfstr(account);
  query = CFDictionaryCreateMutable(NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
  if (svc == NULL || acc == NULL || query == NULL) { st = errSecAllocate; goto cleanup; }
  CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
  CFDictionarySetValue(query, kSecAttrService, svc);
  CFDictionarySetValue(query, kSecAttrAccount, acc);
  CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue);
  CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
  CFDictionarySetValue(query, kSecUseAuthenticationUI, kSecUseAuthenticationUIFail);
#pragma clang diagnostic pop
  st = SecItemCopyMatching(query, &result);
  if (st != errSecSuccess) { goto cleanup; }
  if (result == NULL || CFGetTypeID(result) != CFDataGetTypeID()) { st = errSecDecode; goto cleanup; }
  CFDataRef data = (CFDataRef)result;
  CFIndex length = CFDataGetLength(data);
  if (length <= 0) { st = errSecDecode; goto cleanup; }
  char *buffer = malloc((size_t)length + 1);
  if (buffer == NULL) { st = errSecAllocate; goto cleanup; }
  memcpy(buffer, CFDataGetBytePtr(data), (size_t)length);
  buffer[length] = 0;
  *out = buffer;
  *out_len = length;
cleanup:
  if (result != NULL) CFRelease(result);
  if (query != NULL) CFRelease(query);
  if (svc != NULL) CFRelease(svc);
  if (acc != NULL) CFRelease(acc);
  return st;
}
static OSStatus widget_write(const char *service, const char *account, const char *value) {
  OSStatus st = errSecSuccess;
  CFStringRef svc = cfstr(service), acc = cfstr(account);
  CFDataRef data = NULL;
  CFMutableDictionaryRef query = NULL, update = NULL;
  if (svc == NULL || acc == NULL) { st = errSecAllocate; goto cleanup; }
  data = CFDataCreate(NULL, (const UInt8*)value, (CFIndex)strlen(value));
  query = CFDictionaryCreateMutable(NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
  if (data == NULL || query == NULL) { st = errSecAllocate; goto cleanup; }
  CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
  CFDictionarySetValue(query, kSecAttrService, svc);
  CFDictionarySetValue(query, kSecAttrAccount, acc);
  CFDictionarySetValue(query, kSecValueData, data);
  st = SecItemAdd(query, NULL);
  if (st == errSecDuplicateItem) {
    CFDictionaryRemoveValue(query, kSecValueData);
    update = CFDictionaryCreateMutable(NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    if (update == NULL) { st = errSecAllocate; goto cleanup; }
    CFDictionarySetValue(update, kSecValueData, data);
    st = SecItemUpdate(query, update);
  }
cleanup:
  if (update != NULL) CFRelease(update);
  if (query != NULL) CFRelease(query);
  if (data != NULL) CFRelease(data);
  if (svc != NULL) CFRelease(svc);
  if (acc != NULL) CFRelease(acc);
  return st;
}
static OSStatus widget_delete(const char *service, const char *account) {
  OSStatus st = errSecSuccess;
  CFStringRef svc = cfstr(service), acc = cfstr(account);
  CFMutableDictionaryRef query = NULL;
  if (svc == NULL || acc == NULL) { st = errSecAllocate; goto cleanup; }
  query = CFDictionaryCreateMutable(NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
  if (query == NULL) { st = errSecAllocate; goto cleanup; }
  CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
  CFDictionarySetValue(query, kSecAttrService, svc);
  CFDictionarySetValue(query, kSecAttrAccount, acc);
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
  CFDictionarySetValue(query, kSecUseAuthenticationUI, kSecUseAuthenticationUIFail);
#pragma clang diagnostic pop
  st = SecItemDelete(query);
cleanup:
  if (query != NULL) CFRelease(query);
  if (svc != NULL) CFRelease(svc);
  if (acc != NULL) CFRelease(acc);
  return st;
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type systemStore struct {
	service string
	account string
}

func NewSystemStore() SecretStore {
	return systemStore{service: widgetCredentialService, account: widgetCredentialAccount}
}
func newSystemStore(service, account string) systemStore {
	return systemStore{service: service, account: account}
}
func (s systemStore) Read() (string, error) {
	var out *C.char
	var length C.CFIndex
	service := C.CString(s.service)
	account := C.CString(s.account)
	defer C.free(unsafe.Pointer(service))
	defer C.free(unsafe.Pointer(account))
	st := C.widget_read(service, account, &out, &length)
	if st == C.errSecItemNotFound {
		return "", ErrNotFound
	}
	if st == C.errSecInteractionNotAllowed || st == C.errSecAuthFailed {
		return "", ErrCredentialAccess
	}
	if st != C.errSecSuccess {
		return "", fmt.Errorf("读取 Keychain 悬浮球凭据失败: %d", st)
	}
	if out == nil || length <= 0 {
		return "", ErrInvalidCredential
	}
	defer C.free(unsafe.Pointer(out))
	return string(C.GoBytes(unsafe.Pointer(out), C.int(length))), nil
}
func (s systemStore) Write(v string) error {
	if err := ValidateToken(v); err != nil {
		return err
	}
	cv := C.CString(v)
	service := C.CString(s.service)
	account := C.CString(s.account)
	defer C.free(unsafe.Pointer(cv))
	defer C.free(unsafe.Pointer(service))
	defer C.free(unsafe.Pointer(account))
	if st := C.widget_write(service, account, cv); st != C.errSecSuccess {
		return fmt.Errorf("写入 Keychain 悬浮球凭据失败: %d", st)
	}
	return nil
}
func (s systemStore) Delete() error {
	service := C.CString(s.service)
	account := C.CString(s.account)
	defer C.free(unsafe.Pointer(service))
	defer C.free(unsafe.Pointer(account))
	st := C.widget_delete(service, account)
	if st == C.errSecItemNotFound {
		return nil
	}
	if st == C.errSecInteractionNotAllowed || st == C.errSecAuthFailed {
		return ErrCredentialAccess
	}
	if st != C.errSecSuccess {
		return fmt.Errorf("删除 Keychain 悬浮球凭据失败: %d", st)
	}
	return nil
}

//go:build windows

package widgetauth

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const credTypeGeneric = 1

type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

var (
	advapi32   = windows.NewLazySystemDLL("advapi32.dll")
	credRead   = advapi32.NewProc("CredReadW")
	credWrite  = advapi32.NewProc("CredWriteW")
	credDelete = advapi32.NewProc("CredDeleteW")
	credFree   = advapi32.NewProc("CredFree")
)

type systemStore struct{ targetName string }

func NewSystemStore() SecretStore                  { return systemStore{targetName: widgetCredentialWindowsTarget} }
func newSystemStore(targetName string) systemStore { return systemStore{targetName: targetName} }
func (s systemStore) target() *uint16              { return windows.StringToUTF16Ptr(s.targetName) }
func (s systemStore) Read() (string, error) {
	var c *credential
	targetName := s.target()
	r, _, e := credRead.Call(uintptr(unsafe.Pointer(targetName)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&c)))
	runtime.KeepAlive(targetName)
	if r == 0 {
		if e == windows.ERROR_NOT_FOUND {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("读取 Windows 悬浮球凭据失败: %w", e)
	}
	defer credFree.Call(uintptr(unsafe.Pointer(c)))
	if c == nil || c.CredentialBlob == nil || c.CredentialBlobSize == 0 || c.CredentialBlobSize > 1024 {
		return "", ErrInvalidCredential
	}
	value := string(unsafe.Slice(c.CredentialBlob, c.CredentialBlobSize))
	runtime.KeepAlive(c)
	return value, nil
}
func (s systemStore) Write(v string) error {
	if err := ValidateToken(v); err != nil {
		return err
	}
	b := []byte(v)
	targetName := s.target()
	c := credential{Type: credTypeGeneric, TargetName: targetName, CredentialBlobSize: uint32(len(b)), Persist: 2}
	if len(b) > 0 {
		c.CredentialBlob = &b[0]
	}
	r, _, e := credWrite.Call(uintptr(unsafe.Pointer(&c)), 0)
	runtime.KeepAlive(targetName)
	runtime.KeepAlive(b)
	runtime.KeepAlive(&c)
	if r == 0 {
		return fmt.Errorf("写入 Windows 悬浮球凭据失败: %w", e)
	}
	return nil
}
func (s systemStore) Delete() error {
	targetName := s.target()
	r, _, e := credDelete.Call(uintptr(unsafe.Pointer(targetName)), credTypeGeneric, 0)
	runtime.KeepAlive(targetName)
	if r == 0 && e != windows.ERROR_NOT_FOUND {
		return fmt.Errorf("删除 Windows 悬浮球凭据失败: %w", e)
	}
	return nil
}

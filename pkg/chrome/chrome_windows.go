package chrome

// https://groups.google.com/d/msg/golang-nuts/bUetmxErnTw/GHC5obCcmTMJ
// https://play.golang.org/p/fknP9AuLU-

import (
	"syscall"
	"unsafe"
)

const (
	// CryptProtectUIForbidden prevents popups during CryptUnprotectData
	// as explained on MSDN:
	// https://docs.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptunprotectdata#parameters
	CryptProtectUIForbidden = 0x1
)

var (
	dllcrypt32  = syscall.NewLazyDLL("Crypt32.dll")
	dllkernel32 = syscall.NewLazyDLL("Kernel32.dll")

	procDecryptData = dllcrypt32.NewProc("CryptUnprotectData")
	procLocalFree   = dllkernel32.NewProc("LocalFree")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newBlob(d []byte) *dataBlob {
	if len(d) == 0 {
		return &dataBlob{}
	}
	return &dataBlob{
		pbData: &d[0],
		cbData: uint32(len(d)),
	}
}

func (b *dataBlob) toByteArray() []byte {
	d := make([]byte, b.cbData)
	copy(d, (*[1 << 30]byte)(unsafe.Pointer(b.pbData))[:])
	return d
}

func decrypt(data []byte) ([]byte, error) {
	var outblob dataBlob
	r, _, err := procDecryptData.Call(
		uintptr(unsafe.Pointer(newBlob(data))),
		0,
		0,
		0,
		0,
		CryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&outblob)),
	)

	if r == 0 {
		return nil, err
	}

	defer procLocalFree.Call(uintptr(unsafe.Pointer(outblob.pbData)))

	return outblob.toByteArray(), nil
}

func decryptValue(encrypted []byte) (string, error) {
	s, err := decrypt(encrypted)
	if err != nil {
		return ``, err
	}
	return string(s), nil
}

func setChromeKeychainPassword(password []byte) []byte {
	return password
}

//go:build windows

package privdir

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Design: a private object is created with a security descriptor passed to
// CreateDirectory/CreateFile, so it never exists with a default (inherited,
// permissive) DACL. The descriptor names the current user's SID as owner and
// carries a protected DACL (no inheritance from the parent) with a single
// access-allowed ACE for that SID. SYSTEM is not granted: it can reach the
// object through its privileges anyway, and an ACE would only add exposure.
// Validation tolerates a SYSTEM ACE but nothing else.

var systemSID = mustSID("S-1-5-18")

func mustSID(s string) *windows.SID {
	sid, err := windows.StringToSid(s)
	if err != nil {
		panic(err)
	}
	return sid
}

// UserTag names the current user in shared directory names (the SID).
func UserTag() string {
	sid, err := currentUser()
	if err != nil {
		return "user"
	}
	return sid.String()
}

func currentUser() (*windows.SID, error) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return u.User.Sid, nil
}

// attrs builds SECURITY_ATTRIBUTES for a private object. The returned sd must
// stay reachable until the create call returns.
func attrs(dir bool) (*windows.SecurityAttributes, *windows.SECURITY_DESCRIPTOR, error) {
	sid, err := currentUser()
	if err != nil {
		return nil, nil, err
	}
	flags := ""
	if dir {
		flags = "OICI"
	}
	sddl := fmt.Sprintf("O:%sD:P(A;%s;FA;;;%s)", sid, flags, sid)
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, nil, err
	}
	sa := &windows.SecurityAttributes{SecurityDescriptor: sd}
	sa.Length = uint32(unsafe.Sizeof(*sa))
	return sa, sd, nil
}

// Ensure creates dir with a private descriptor unless it exists, then
// validates it by handle: a real directory (not a reparse point), owned by
// this user, protected DACL granting nobody else.
func Ensure(dir string) error {
	sa, sd, err := attrs(true)
	if err != nil {
		return err
	}
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	err = windows.CreateDirectory(p, sa)
	runtime.KeepAlive(sd)
	if err != nil && err != windows.ERROR_ALREADY_EXISTS {
		return err
	}
	return Verify(dir, true)
}

// CreatePrivateFile creates a new file (it must not exist) readable and
// writable only by this user. The handle is exclusive until closed.
func CreatePrivateFile(path string) (*os.File, error) {
	sa, sd, err := attrs(false)
	if err != nil {
		return nil, err
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, sa,
		windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	runtime.KeepAlive(sd)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}

// Verify checks path as Ensure does (dir) or as a private regular file.
func Verify(path string, dir bool) error {
	h, err := openHandle(path, dir, windows.READ_CONTROL|windows.FILE_READ_ATTRIBUTES)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	if err := verifyHandle(h, dir); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// OpenVerified opens an existing private file for reading. The final
// component is never followed; the handle itself is validated, so the file
// that is read is the file that was checked.
func OpenVerified(path string) (*os.File, error) {
	h, err := openHandle(path, false, windows.GENERIC_READ|windows.READ_CONTROL)
	if err != nil {
		return nil, err
	}
	if err := verifyHandle(h, false); err != nil {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return os.NewFile(uintptr(h), path), nil
}

func openHandle(path string, dir bool, access uint32) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	flags := uint32(windows.FILE_FLAG_OPEN_REPARSE_POINT)
	if dir {
		flags |= windows.FILE_FLAG_BACKUP_SEMANTICS
	}
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	h, err := windows.CreateFile(p, access, share, nil, windows.OPEN_EXISTING, flags, 0)
	if err != nil {
		return 0, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return h, nil
}

func verifyHandle(h windows.Handle, dir bool) error {
	var tag struct{ attrs, reparseTag uint32 }
	if err := windows.GetFileInformationByHandleEx(h, windows.FileAttributeTagInfo,
		(*byte)(unsafe.Pointer(&tag)), uint32(unsafe.Sizeof(tag))); err != nil {
		return errors.New("cannot read attributes")
	}
	if tag.attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errors.New("is a reparse point (symlink or junction)")
	}
	if isDir := tag.attrs&windows.FILE_ATTRIBUTE_DIRECTORY != 0; isDir != dir {
		if dir {
			return errors.New("not a directory")
		}
		return errors.New("not a regular file")
	}
	sd, err := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return errors.New("cannot read security descriptor")
	}
	me, err := currentUser()
	if err != nil {
		return errors.New("cannot determine current user")
	}
	return CheckDescriptor(sd, me)
}

// CheckDescriptor reports why sd is not private to me: another owner, a NULL
// or unprotected DACL, or any ACE that could grant access to someone else.
// Deny ACEs grant nothing and are ignored; unknown ACE types are refused.
func CheckDescriptor(sd *windows.SECURITY_DESCRIPTOR, me *windows.SID) error {
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !owner.Equals(me) {
		return errors.New("owned by another user")
	}
	ctrl, _, err := sd.Control()
	if err != nil {
		return errors.New("cannot read security descriptor")
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		return errors.New("open to other users (no DACL)")
	}
	if ctrl&windows.SE_DACL_PROTECTED == 0 {
		return errors.New("open to other users (DACL inherits from its parent)")
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			return errors.New("cannot read DACL")
		}
		switch ace.Header.AceType {
		case windows.ACCESS_DENIED_ACE_TYPE:
			continue
		case windows.ACCESS_ALLOWED_ACE_TYPE:
			sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
			if sid.Equals(me) || sid.Equals(systemSID) {
				continue
			}
			return fmt.Errorf("open to other users (grants %s)", sid)
		default:
			return errors.New("open to other users (unsupported ACE type)")
		}
	}
	return nil
}

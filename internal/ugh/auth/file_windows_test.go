//go:build windows

package auth

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/AmadeusDE/Misa/internal/ugh/privdir"
)

func assertPrivate(t *testing.T, path string, dir bool) {
	t.Helper()
	if err := privdir.Verify(path, dir); err != nil {
		t.Errorf("%s is not private: %v", filepath.Base(path), err)
	}
}

func savedStore(t *testing.T) fileBackend {
	t.Helper()
	f := fileBackend{path: testStorePath(t)}
	c := newCredentials()
	c.Providers["openrouter"] = Credential{Type: "api_key", Key: "sk-or-v1-abcdef0123456789"}
	if err := f.Save(c); err != nil {
		t.Fatal(err)
	}
	return f
}

func grantEveryone(t *testing.T, path string) {
	t.Helper()
	me, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	everyone, _ := windows.StringToSid("S-1-1-0")
	entry := func(sid *windows.SID, perm windows.ACCESS_MASK) windows.EXPLICIT_ACCESS {
		return windows.EXPLICIT_ACCESS{AccessPermissions: perm, AccessMode: windows.GRANT_ACCESS,
			Inheritance: windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
			Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
				TrusteeValue: windows.TrusteeValueFromSID(sid)}}
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		entry(me.User.Sid, windows.GENERIC_ALL), entry(everyone, windows.GENERIC_READ)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}

func TestFileRefusesPermissiveACLs(t *testing.T) {
	f := savedStore(t)
	grantEveryone(t, f.path)
	if _, err := f.Load(); err == nil {
		t.Error("file readable by Everyone was accepted")
	}

	f = savedStore(t)
	grantEveryone(t, filepath.Dir(f.path))
	if _, err := f.Load(); err == nil {
		t.Error("directory open to Everyone was accepted for Load")
	}
	if err := f.Save(newCredentials()); err == nil {
		t.Error("directory open to Everyone was accepted for Save")
	}
}

func TestFileRefusesReplacementWithInheritedACL(t *testing.T) {
	f := savedStore(t)
	if err := os.Remove(f.path); err != nil {
		t.Fatal(err)
	}
	// An ordinary create inherits the directory's ACEs without a protected DACL.
	if err := os.WriteFile(f.path, []byte(`{"version":1,"providers":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Load(); err == nil {
		t.Error("file with an unprotected DACL was accepted")
	}
}

func TestFileRefusesJunctionAndSymlink(t *testing.T) {
	f := savedStore(t)
	dir := filepath.Dir(f.path)
	moved := dir + "-real"
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", dir, moved).CombinedOutput(); err != nil {
		t.Skipf("mklink /J: %v: %s", err, out)
	}
	if _, err := f.Load(); err == nil {
		t.Error("junctioned credentials directory was accepted")
	}
	if err := f.Save(newCredentials()); err == nil {
		t.Error("junctioned credentials directory was accepted for Save")
	}

	g := savedStore(t)
	real := g.path + ".real"
	if err := os.Rename(g.path, real); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, g.path); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := g.Load(); err == nil {
		t.Error("symlinked credentials file was accepted")
	}
}

func TestSaveReplacesInPlaceAndLeavesNoStaging(t *testing.T) {
	f := savedStore(t)
	c, err := f.Load()
	if err != nil {
		t.Fatal(err)
	}
	c.Providers["opencode"] = Credential{Type: "api_key", Key: "sk-oc-0123456789abcdef"}
	if err := f.Save(c); err != nil {
		t.Fatal(err)
	}
	got, err := f.Load()
	if err != nil || len(got.Providers) != 2 {
		t.Fatalf("reload: %v, %d providers", err, len(got.Providers))
	}
	entries, _ := os.ReadDir(filepath.Dir(f.path))
	if len(entries) != 1 {
		t.Fatalf("dir entries = %d, want only the credentials file", len(entries))
	}
}

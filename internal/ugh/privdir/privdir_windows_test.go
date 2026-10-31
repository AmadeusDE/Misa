//go:build windows

package privdir

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func me(t *testing.T) *windows.SID {
	t.Helper()
	sid, err := currentUser()
	if err != nil {
		t.Fatal(err)
	}
	return sid
}

func descriptor(t *testing.T, sddl string) *windows.SECURITY_DESCRIPTOR {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatalf("%s: %v", sddl, err)
	}
	return sd
}

func TestCheckDescriptor(t *testing.T) {
	u := me(t).String()
	cases := []struct {
		name, sddl string
		ok         bool
	}{
		{"user only", "O:" + u + "D:P(A;OICI;FA;;;" + u + ")", true},
		{"user and system", "O:" + u + "D:P(A;OICI;FA;;;" + u + ")(A;OICI;FA;;;SY)", true},
		{"deny others is harmless", "O:" + u + "D:P(D;;FA;;;WD)(A;;FA;;;" + u + ")", true},
		{"wrong owner", "O:BAD:P(A;;FA;;;" + u + ")", false},
		{"everyone", "O:" + u + "D:P(A;;FA;;;" + u + ")(A;;FR;;;WD)", false},
		{"users", "O:" + u + "D:P(A;;FA;;;" + u + ")(A;;FR;;;BU)", false},
		{"authenticated users", "O:" + u + "D:P(A;;FA;;;" + u + ")(A;;FR;;;AU)", false},
		{"administrators", "O:" + u + "D:P(A;;FA;;;" + u + ")(A;;FR;;;BA)", false},
		{"inherit-only grant to others", "O:" + u + "D:P(A;OICIIO;FR;;;WD)(A;;FA;;;" + u + ")", false},
		{"unprotected", "O:" + u + "D:(A;;FA;;;" + u + ")", false},
		{"null dacl", "O:" + u + "D:NO_ACCESS_CONTROL", false},
	}
	for _, c := range cases {
		err := CheckDescriptor(descriptor(t, c.sddl), me(t))
		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
		}
	}
}

func TestEnsureCreatesAPrivateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ugh-1")
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(dir); err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	// A child inherits the user-only ACE and validates as a private file.
	f, err := CreatePrivateFile(filepath.Join(dir, "f"))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := Verify(filepath.Join(dir, "f"), false); err != nil {
		t.Fatal(err)
	}
	g, err := OpenVerified(filepath.Join(dir, "f"))
	if err != nil {
		t.Fatal(err)
	}
	g.Close()
}

func TestEnsureRefusesInheritedPermissiveACL(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "plain")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(dir); err == nil {
		t.Fatal("directory with an inherited DACL accepted")
	}
}

func TestEnsureRefusesGrantAddedLater(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ugh-2")
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	everyone, _ := windows.StringToSid("S-1-1-0")
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		{AccessPermissions: windows.GENERIC_ALL, AccessMode: windows.GRANT_ACCESS, Inheritance: windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
			Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
				TrusteeValue: windows.TrusteeValueFromSID(me(t))}},
		{AccessPermissions: windows.GENERIC_READ, AccessMode: windows.GRANT_ACCESS, Inheritance: windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
			Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
				TrusteeValue: windows.TrusteeValueFromSID(everyone)}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = Ensure(dir)
	if err == nil || !strings.Contains(err.Error(), "open to other users") {
		t.Fatalf("Ensure = %v, want open to other users", err)
	}
}

func TestEnsureRefusesJunctionAndFile(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := Ensure(target); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(base, "junction")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Skipf("mklink /J: %v: %s", err, out)
	}
	if err := Ensure(junction); err == nil || !strings.Contains(err.Error(), "reparse point") {
		t.Fatalf("junction: %v", err)
	}
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(file); err == nil {
		t.Fatal("file accepted as directory")
	}
	if _, err := OpenVerified(file); err == nil {
		t.Fatal("file with an inherited DACL accepted")
	}
}

func TestOpenVerifiedRefusesSymlinkedFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ugh-3")
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(dir, "real")
	f, err := CreatePrivateFile(real)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := OpenVerified(link); err == nil {
		t.Fatal("symlink followed")
	}
}

func TestCreatePrivateFileRefusesExisting(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ugh-4")
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "f")
	f, err := CreatePrivateFile(p)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := CreatePrivateFile(p); err == nil {
		t.Fatal("existing file replaced")
	}
}

func TestUnderFallsBackWhenSharedDirIsUnsafe(t *testing.T) {
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "ugh-x"), 0o700); err != nil { // inherited DACL
		t.Fatal(err)
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	dir, err := Under(base, "ugh-x")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(dir) == base {
		t.Fatalf("unsafe shared directory used: %s", dir)
	}
}

func TestUnderReportsFailure(t *testing.T) {
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "ugh-y"), 0o700); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(t.TempDir(), "cache")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCALAPPDATA", blocked)
	dir, err := Under(base, "ugh-y")
	if err == nil || dir != "" {
		t.Fatalf("Under = %q, %v; want an error and no path", dir, err)
	}
}

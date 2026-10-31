package pwhash

import "testing"

// The published vectors of Ulrich Drepper's "Unix crypt using SHA-256 and
// SHA-512" (public domain) for the $5$ and $6$ formats, plus the md5-crypt
// vectors from toybox tests/mkpasswd.test (commit 6a0c0ef5eeb2c43e4feef9781
// 398c1718b8bddcb, 0BSD). They pin the algorithm: rounds clamping (rounds=10
// becomes 1000), explicit rounds echoing, salt truncation to 16 bytes for the
// SHA formats and to 8 bytes for md5-crypt, and the encoding order.
func TestPublishedVectors(t *testing.T) {
	cases := []struct {
		name    string
		pw      string
		setting string
		want    string
	}{
		// SHA-256-crypt ($5$).
		{"sha256-saltstring", "Hello world!", "$5$saltstring",
			"$5$saltstring$5B8vYYiY.CVt1RlTTf8KbXBH3hsxY/GNooZaBBGWEc5"},
		{"sha256-rounds-10-clamped", "Hello world!", "$5$rounds=10$roundstoolow",
			"$5$rounds=1000$roundstoolow$BiO0thfsibXRWnVAhxypb/bDS/8S0KICVgPbqzirYmC"},
		{"sha256-rounds-1000", "Hello world!", "$5$rounds=1000$roundstoolow",
			"$5$rounds=1000$roundstoolow$BiO0thfsibXRWnVAhxypb/bDS/8S0KICVgPbqzirYmC"},
		{"sha256-rounds-10000", "Hello world!", "$5$rounds=10000$saltstringsaltstring",
			"$5$rounds=10000$saltstringsaltst$3xv.VbSHBb41AL9AvLeujZkZRBAwqFMz2.opqey6IcA"},
		{"sha256-long-salt-truncated", "Hello world!", "$5$this is a very long salt",
			"$5$this is a very l$LOR.yshtghuOGyjNTEwbskuAqcrjqUufz/nX.2Lsnd0"},
		{"sha256-toybox-password", "password", "$5$abcdefgh",
			"$5$abcdefgh$ZLdkj8mkc2XVSrPVjskDAgZPGjtj1VGVaa1aUkrMTU/"},
		{"sha256-toybox-16char-salt", "password", "$5$./Aa0Bb1Cc2Dd3Ee",
			"$5$./Aa0Bb1Cc2Dd3Ee$5iXcesTggTRGvAAa3cWlpxmUqNGOeQh/iO3Furo4y/D"},
		{"sha256-rounds-1000-password", "password", "$5$rounds=1000$abcdefgh",
			"$5$rounds=1000$abcdefgh$GEazsRB0Tipk2UJZtYJM/eHHRMtwea4Vnto3YxMbrG0"},

		// SHA-512-crypt ($6$).
		{"sha512-saltstring", "Hello world!", "$6$saltstring",
			"$6$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1"},
		{"sha512-rounds-10-clamped", "Hello world!", "$6$rounds=10$roundstoolow",
			"$6$rounds=1000$roundstoolow$VTiyBzzTJoDUzG2edg6tTfnH44buhC6xQa2y1SRnr1w/dVOBbXKE612uZFeIlMGZ8MgLiap2x5mD5IOra0fN00"},
		{"sha512-rounds-10000", "Hello world!", "$6$rounds=10000$saltstringsaltstring",
			"$6$rounds=10000$saltstringsaltst$OW1/O6BYHV6BcXZu8QVeXbDWra3Oeqh0sbHbbMCVNSnCM/UrjmM0Dp8vOuZeHBy/YTBmSK6H9qs/y3RnOaw5v."},
		{"sha512-long-salt-truncated", "Hello world!", "$6$this is a very long salt",
			"$6$this is a very l$1PpdQN5qq0KfuvWoeli39eHmsugDO78qLqSUfiQN6PWtixDTmEfTwATiqRkHB5Kox2crQK21c4Uw3zkHI2lpy1"},
		{"sha512-toybox-password", "password", "$6$abcdefgh",
			"$6$abcdefgh$yVfUwsw5T.JApa8POvClA1pQ5peiq97DUNyXCZN5IrF.BMSkiaLQ5kvpuEm/VQ1Tvh/KV2TcaWh8qinoW5dhA1"},
		{"sha512-toybox-16char-salt", "password", "$6$./Aa0Bb1Cc2Dd3Ee",
			"$6$./Aa0Bb1Cc2Dd3Ee$PvmedaPf329sM25Jn2jv3MsfK9DaDh6tyVtJucp35A/Lmrtp9g1Ab35Mr59pkuMU3QJlbXYoWJFaxyD4OwIZ60"},
		{"sha512-rounds-10000-password", "password", "$6$rounds=10000$abcdefgh",
			"$6$rounds=10000$abcdefgh$kF57o18Xuu8qWSW2h0YBILpIeW/KEpcSse9BHaTTStgl9j7nCGzty1xRBjmP7wDA01Idc.ivBCoIRvlUbkXmQ/"},
		{"sha512-rounds-5000-echoed", "password", "$6$rounds=5000$abcdefgh",
			"$6$rounds=5000$abcdefgh$yVfUwsw5T.JApa8POvClA1pQ5peiq97DUNyXCZN5IrF.BMSkiaLQ5kvpuEm/VQ1Tvh/KV2TcaWh8qinoW5dhA1"},

		// MD5-crypt ($1$).
		{"md5-toybox-password", "password", "$1$abcdefgh",
			"$1$abcdefgh$G//4keteveJp0qb8z2DxG/"},
		{"md5-salt-truncated", "Hello world!", "$1$saltstring",
			"$1$saltstri$YMyguxXMBpd2TEZ.vS/3q1"},
		{"md5-long-salt-truncated", "Hello world!", "$1$this is a very long salt",
			"$1$this is $zv0Z6Uapvo57U9NQBMltB."},
		{"md5-short-salt", "Hello world!", "$1$short",
			"$1$short$7HC4N2zLzijYEsBZlKvgD0"},
		{"md5-punctuation-salt", "Hello world!", "$1$ab.d/",
			"$1$ab.d/$WA5ybgcSUDg8lAFdzc9qn1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Crypt(c.pw, c.setting)
			if err != nil {
				t.Fatalf("Crypt(%q, %q): %v", c.pw, c.setting, err)
			}
			if got != c.want {
				t.Errorf("Crypt(%q, %q)\n got %s\nwant %s", c.pw, c.setting, got, c.want)
			}
		})
	}
}

// TestRoundsClampEquivalence checks the rounds clamping on a cheap path: the
// outputs for rounds=10 and rounds=1000 are identical, and an explicit
// rounds=5000 hash equals the default-rounds hash of the same salt.
func TestRoundsClampEquivalence(t *testing.T) {
	for _, prefix := range []string{"$5$", "$6$"} {
		low, err := Crypt("pw", prefix+"rounds=10$abcdefgh")
		if err != nil {
			t.Fatal(err)
		}
		min, err := Crypt("pw", prefix+"rounds=1000$abcdefgh")
		if err != nil {
			t.Fatal(err)
		}
		if low != min {
			t.Errorf("%s: rounds=10 (%s) != rounds=1000 (%s)", prefix, low, min)
		}
		if want := prefix + "rounds=1000$abcdefgh$"; low[:len(want)] != want {
			t.Errorf("%s: clamped rounds not echoed: %s", prefix, low)
		}
		custom, err := Crypt("pw", prefix+"rounds=5000$abcdefgh")
		if err != nil {
			t.Fatal(err)
		}
		def, err := Crypt("pw", prefix+"abcdefgh")
		if err != nil {
			t.Fatal(err)
		}
		if custom != prefix+"rounds=5000$abcdefgh$"+def[len(prefix+"abcdefgh$"):] {
			t.Errorf("%s: rounds=5000 (%s) does not match default (%s)", prefix, custom, def)
		}
	}
}

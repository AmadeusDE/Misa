package pwhash

import "testing"

// Cross-check vectors for passwords shorter and longer than one digest block,
// explicit rounds and salt truncation. Their expected values were produced with
// `openssl passwd -1|-5|-6 -salt SALT PASSWORD`, used only as an independent
// oracle for the published algorithm: no code was taken from it.
func TestCryptOpenSSLVectors(t *testing.T) {
	cases := []struct{ pw, setting, want string }{
		{"a", "$1$saltstri", "$1$saltstri$HP4ANLG0Ln2mcWmFUeETo."},
		{"a", "$5$saltstring", "$5$saltstring$7cz4bTeQ7MnNssphNhFVrITtuJYY/1tdvLL2uzLvOk8"},
		{"a", "$6$saltstring", "$6$saltstring$MO53nAXQUKXVLlsbiXyPgMsR6q10N7eF7sPvanwdXnEeCj5kE3eYaRvFv0wVW1UZ4SnNTzc1v4OCOq1ASDQZY0"},
		{"a", "$5$rounds=1000$roundsalt", "$5$rounds=1000$roundsalt$E3zMvLbCnRbxWFqH7/FQOuuT2hOjD1F6sVvS8YHuMJ1"},
		{"a", "$6$rounds=1400$anotherlongsaltstringXX", "$6$rounds=1400$anotherlongsalts$Fu.LkXqygnUvG7rbMCVvZHM1cxxrCeKetK7WleDTj8ua4.TkZZcyRGCsWPwljjreE8gYZ4JIUQT2KPYKLxKmL."},
		{"Hello world!", "$1$saltstri", "$1$saltstri$YMyguxXMBpd2TEZ.vS/3q1"},
		{"Hello world!", "$5$saltstring", "$5$saltstring$5B8vYYiY.CVt1RlTTf8KbXBH3hsxY/GNooZaBBGWEc5"},
		{"Hello world!", "$6$saltstring", "$6$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1"},
		{"Hello world!", "$5$rounds=1000$roundsalt", "$5$rounds=1000$roundsalt$/yqDx3vwH.0zB1gYhigHVcV..RPt4qKZEGdwBPqmiq8"},
		{"Hello world!", "$6$rounds=1400$anotherlongsaltstringXX", "$6$rounds=1400$anotherlongsalts$5FGyu8c4BZDX4wJgs0Un26YOw2XibT5eTkHF1I1aP3QqStoJI9BHD2YPJYsAjEePVGUyBjdZxcNqMWlrrbIOC."},
		{"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", "$1$saltstri", "$1$saltstri$dhNPNA0oj.XF04EA8u.uu1"},
		{"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", "$5$saltstring", "$5$saltstring$xgMVX.dxWLPhqfWgxtBzsLSyyt8HLabKkX87lfNv3P/"},
		{"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", "$6$saltstring", "$6$saltstring$01cmVY11FK/cnP5tFzulg2i4v8gFoHTGMUHD.60x3Hbe8r.bNwa9VSu/WgkWe4/38pL9GKBHclUKsUoX2JTmD0"},
		{"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", "$5$rounds=1000$roundsalt", "$5$rounds=1000$roundsalt$1YvCSeR2sO88pGeFj5SxjCgK0MkOqA3KzM1i/HybEa8"},
		{"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", "$6$rounds=1400$anotherlongsaltstringXX", "$6$rounds=1400$anotherlongsalts$/Kk4jMDLnXA3mmTaQJIHPBFOFqgrlseFhWlMQScA3bKgqs6CFt97DwaPOzCnIgD2k2P.WOQsgRktkQoC4KK1M."},
		{"yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy", "$1$saltstri", "$1$saltstri$OQEItBDv98yXQzmDqBQkF0"},
		{"yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy", "$5$saltstring", "$5$saltstring$Hr38eNgCsMCSuNpnWfVypYlweKpHq0f0dED0YaL7LT."},
		{"yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy", "$6$saltstring", "$6$saltstring$40t4bLnG1.pn9cDzrS8cQNt.vbqczVYJ5TtuNgju7SFKhR6j.sR3e/fxKfLAB7Wo0/C8Th3g8S54GNXPfjN2e."},
		{"yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy", "$5$rounds=1000$roundsalt", "$5$rounds=1000$roundsalt$wIz26Otiy9o2j2Mg9wZcpPmzLVjpc7NhP0wl/tHMHpD"},
		{"yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy", "$6$rounds=1400$anotherlongsaltstringXX", "$6$rounds=1400$anotherlongsalts$SxmkTJ9zcOVNjXYFXG2LH/Ke1dKj2xRfZrqTnurMGsItuV443UgxknWPj9ecwh.uHJacVPZssNYHZlDVbA1bu."},
		{"a much longer password that spans more than one hundred and twenty eight bytes ............................................ end", "$1$saltstri", "$1$saltstri$lTVSgulS7rRnI/wI2kBBf0"},
		{"a much longer password that spans more than one hundred and twenty eight bytes ............................................ end", "$5$saltstring", "$5$saltstring$OrFePsIgKmeWxNxXmw4lMyEZLgQEeu9TkTOF6Pff911"},
		{"a much longer password that spans more than one hundred and twenty eight bytes ............................................ end", "$6$saltstring", "$6$saltstring$eoUPMacXS.jOtk.A9sHYQXMJ6U/UpXJs7EthlTYIZOq7WwzKVQZYZg0pMH3T2YJR0bECqvXzuFmHKZ376a5H31"},
		{"a much longer password that spans more than one hundred and twenty eight bytes ............................................ end", "$5$rounds=1000$roundsalt", "$5$rounds=1000$roundsalt$xTOXPfIdIn2NVlI5pSOKGkE6Le.VTTM0pzh464o7Wj8"},
		{"a much longer password that spans more than one hundred and twenty eight bytes ............................................ end", "$6$rounds=1400$anotherlongsaltstringXX", "$6$rounds=1400$anotherlongsalts$uHhioBMiThMIcROmpqrkNirnKbP1i41MAYOdyPr2BanvwScsqHk47fhXUTpkdOg8wXxRNDvDexeAzZGcSu.uW/"},
		{"", "$1$", "$1$$qRPK7m23GJusamGpoGLby/"},
	}
	for _, c := range cases {
		got, err := Crypt(c.pw, c.setting)
		if err != nil {
			t.Errorf("Crypt(%q, %q): %v", c.pw, c.setting, err)
			continue
		}
		if got != c.want {
			t.Errorf("Crypt(%q, %q) = %s, want %s", c.pw, c.setting, got, c.want)
		}
	}
}

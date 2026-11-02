package kittygfx

import "testing"

func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"",
		"a=t,f=24,s=1,v=1;AAAA",
		"a=T,f=32,s=2,v=2,i=1,p=2;AAAAAA==",
		"a=p,i=1,p=1,c=2,r=2,X=0,Y=0",
		"a=d,d=a",
		"a=q,i=1,s=1,v=1,f=24;AAAA",
		"a=t,m=1,i=4294967295,q=2;AAAA",
		"a=t,z=-2147483648;AAAA",
		"a=t,U=0,o=z,s=-1;AA*A",
		"i=3",
		";",
		",,,",
		"a=t" + string(make([]byte, 32)),
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		lim := DefaultLimits()
		cmd, err := Parse(body, lim)
		if err != nil {
			return
		}
		if cmd.Width > lim.MaxWidth || cmd.Height > lim.MaxHeight || len(cmd.Payload) > lim.MaxPayload {
			t.Fatalf("Parse(%q) exceeded limits: %+v", body, cmd)
		}
		wire, encErr := Encode(cmd, cmd.Payload)
		if encErr != nil {
			t.Fatalf("Encode(Parse(%q)) = %v", body, encErr)
		}
		if len(wire) < 5 || wire[0] != 0x1b || wire[1] != '_' || wire[2] != 'G' || wire[len(wire)-2] != 0x1b || wire[len(wire)-1] != '\\' {
			t.Fatalf("Encode(Parse(%q)) = %q is not one APC", body, wire)
		}
		back, err := Parse(wire[3:len(wire)-2], lim)
		if err != nil {
			t.Fatalf("Parse(Encode(Parse(%q))) = %v", body, err)
		}
		if !sameCommand(cmd, back) {
			t.Fatalf("round trip %q: %+v -> %+v", body, cmd, back)
		}
	})
}

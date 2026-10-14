package collector

import "testing"

func TestIEC61850MMSReference(t *testing.T) {
	domain, item, err := iec61850MMSReference("bjwkPROT/MMXU1.YC9.mag.f", "MX")
	if err != nil {
		t.Fatal(err)
	}
	if domain != "bjwkPROT" || item != "MMXU1$MX$YC9$mag$f" {
		t.Fatalf("unexpected MMS reference domain=%q item=%q", domain, item)
	}

	domain, item, err = iec61850MMSReference("LD0/LLN0", "ST")
	if err != nil || domain != "LD0" || item != "LLN0$ST" {
		t.Fatalf("unexpected logical-node reference domain=%q item=%q err=%v", domain, item, err)
	}

	for _, ref := range []string{"", "LD0", "/LLN0.Mod.stVal", "LD0/LLN0.Arr(1).stVal"} {
		if _, _, err = iec61850MMSReference(ref, "ST"); err == nil {
			t.Fatalf("expected invalid reference %q to fail", ref)
		}
	}
	if _, _, err = iec61850MMSReference("LD0/LLN0.Mod.stVal", ""); err == nil {
		t.Fatal("expected missing functional constraint to fail")
	}
}

func TestIEC61850ImplementsBatchCollector(t *testing.T) {
	var _ BatchCollector = IEC61850{}
}

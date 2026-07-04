package web

import "testing"

func TestParseCIDPointsGeneratesIEC61850ObjectRefs(t *testing.T) {
	const cid = `<?xml version="1.0" encoding="UTF-8"?>
<SCL>
  <Header nameStructure="IEDName"/>
  <IED name="bjwk">
    <AccessPoint name="AP1">
      <Server>
        <LDevice inst="PROT">
          <LN0 lnClass="LLN0" lnType="LLN0_TYPE"/>
          <LN lnClass="MMXU" inst="1" lnType="MMXU_TYPE"/>
        </LDevice>
      </Server>
    </AccessPoint>
  </IED>
  <DataTypeTemplates>
    <LNodeType id="LLN0_TYPE" lnClass="LLN0">
      <DO name="Beh" type="INS_TYPE"/>
    </LNodeType>
    <LNodeType id="MMXU_TYPE" lnClass="MMXU">
      <DO name="YC1" desc="Voltage" type="MV_TYPE"/>
      <DO name="SP1" desc="Setpoint" type="APC_TYPE"/>
    </LNodeType>
    <DOType id="INS_TYPE" cdc="INS">
      <DA name="stVal" fc="ST" bType="INT32"/>
      <DA name="q" fc="ST" bType="Quality"/>
      <DA name="t" fc="ST" bType="Timestamp"/>
    </DOType>
    <DOType id="MV_TYPE" cdc="MV">
      <DA name="mag" fc="MX" bType="Struct" type="AnalogueValue"/>
      <DA name="q" fc="MX" bType="Quality"/>
      <DA name="t" fc="MX" bType="Timestamp"/>
    </DOType>
    <DOType id="APC_TYPE" cdc="APC">
      <DA name="setMag" fc="SP" bType="Struct" type="AnalogueValue"/>
    </DOType>
    <DAType id="AnalogueValue">
      <BDA name="f" bType="FLOAT32"/>
    </DAType>
  </DataTypeTemplates>
</SCL>`

	points, summary, warnings, err := parseCIDPoints([]byte(cid), cidPointOptions{
		DeviceKey: "dev1",
		Protocol:  "iec61850",
		Address:   "192.168.1.187:102",
	})
	if err != nil {
		t.Fatalf("parseCIDPoints returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if summary.IEDName != "bjwk" || summary.LDCount != 1 || summary.LNCount != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	byRef := map[string]cidPointPreviewItem{}
	for _, point := range points {
		byRef[point.ObjectRef] = point
	}
	assertPoint := func(ref, fc, dataType string, selected bool) {
		t.Helper()
		point, ok := byRef[ref]
		if !ok {
			t.Fatalf("missing point %s; got %#v", ref, keysOfCIDPoints(byRef))
		}
		if point.FC != fc || point.DataType != dataType || point.Selected != selected {
			t.Fatalf("point %s = fc %s dataType %s selected %v", ref, point.FC, point.DataType, point.Selected)
		}
	}

	assertPoint("bjwkPROT/LLN0.Beh.stVal", "ST", "auto", true)
	assertPoint("bjwkPROT/LLN0.Beh.q", "ST", "quality", false)
	assertPoint("bjwkPROT/MMXU1.YC1.mag.f", "MX", "float32", true)
	assertPoint("bjwkPROT/MMXU1.SP1.setMag.f", "SP", "float32", true)
}

func keysOfCIDPoints(points map[string]cidPointPreviewItem) []string {
	keys := make([]string, 0, len(points))
	for key := range points {
		keys = append(keys, key)
	}
	return keys
}

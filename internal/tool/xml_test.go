package tool

import (
	"encoding/json"
	"testing"
)

func TestParseXML_SOAP(t *testing.T) {
	v, err := ParseXML(`<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body>
<R xmlns="urn:erp"><Id>C1</Id><Limit currency="USD">100</Limit><Holds><Hold><Code>A</Code></Hold><Hold><Code>B</Code></Hold></Holds></R>
</soap:Body></soap:Envelope>`)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	want := `{"Envelope":{"Body":{"R":{"Holds":{"Hold":[{"Code":"A"},{"Code":"B"}]},"Id":"C1","Limit":{"#text":"100","@currency":"USD"}}}}}`
	if string(b) != want {
		t.Fatalf("got %s", b)
	}
	if _, err := ParseXML("not xml"); err == nil {
		t.Fatal("plain text should be an error")
	}
}

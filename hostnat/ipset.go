package hostnat

import (
	"encoding/xml"

	"github.com/pkg/errors"
)

type ipsets struct {
	XMLName xml.Name `xml:"ipsets"`
	Members []string `xml:"ipset>members>elem"`
}

func unmarshalIPSetByXML(data []byte) (ipsets, error) {
	o := ipsets{}
	err := xml.Unmarshal(data, &o)
	if err != nil {
		return o, errors.Wrap(err, "Failed to Unmarshal xml")
	}

	return o, nil
}

package winroutenetsh

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
)

const (
	primaryKeyMissingErrorString = "Missing primary key %s"
)

type IRouter interface {
	GetRoutes() ([]*RouteRow, error)
	GetRouteByDestination(string) (*RouteRow, error)
	GetRoutesByInterfaceIndex(uint64) ([]*RouteRow, error)
	AddRoute(*RouteRow) error
	DeleteRouteByDest(string) error
	SetRoute(*RouteRow) error
}

type RouteRow struct {
	Publish            string     `mapstructure:"Publish" powershell:",get;set;"`
	Protocol           string     `mapstructure:"Protocol" powershell:",get;set;"`
	Store              string     `mapstructure:"Store" powershell:",get;set;"`
	AddressFamily      string     `mapstructure:"AddressFamily" powershell:",get;set;"`
	ifIndex            uint64     `mapstructure:"ifIndex" powershell:",get;"`
	Caption            string     `mapstructure:"Caption" powershell:",get;set;"`
	Description        string     `mapstructure:"Description" powershell:",get;set;"`
	ElementName        string     `mapstructure:"ElementName" powershell:",get;set;"`
	InstanceID         string     `mapstructure:"InstanceID" powershell:",get;set;"`
	AdminDistance      string     `mapstructure:"AdminDistance" powershell:",get;set;"`
	DestinationAddress string     `mapstructure:"DestinationAddress" powershell:",get;set;"`
	IsStatic           string     `mapstructure:"IsStatic" powershell:",get;set;"`
	RouteMetric        uint64     `mapstructure:"RouteMetric" powershell:",get;set;"`
	TypeOfRoute        uint64     `mapstructure:"TypeOfRoute" powershell:""`
	CompartmentId      uint64     `mapstructure:"CompartmentId" powershell:",get;"`
	DestinationPrefix  *net.IPNet `mapstructure:"DestinationPrefix" powershell:",get;"`
	InterfaceAlias     string     `mapstructure:"InterfaceAlias" powershell:",get;"`
	InterfaceIndex     uint64     `mapstructure:"InterfaceIndex" powershell:",get;"`
	NextHop            net.IP     `mapstructure:"NextHop" powershell:",get;"`
	PreferredLifetime  string     `mapstructure:"PreferredLifetime" powershell:",get;set;"`
	ValidLifetime      string     `mapstructure:"ValidLifetime" powershell:",get;set;"`
	PSComputerName     string     `mapstructure:"PSComputerName" powershell:",get;"`
}

func (row *RouteRow) Equal(target *RouteRow) bool {
	return row.DestinationPrefix.String() == target.DestinationPrefix.String() &&
		row.NextHop.Equal(target.NextHop) &&
		row.InterfaceIndex == target.InterfaceIndex &&
		row.Store == target.Store &&
		row.PSComputerName == target.PSComputerName
}

func (row *RouteRow) ToNewString() (string, error) {
	buff := bytes.NewBuffer([]byte{})
	if row.DestinationPrefix == nil {
		return "", fmt.Errorf(primaryKeyMissingErrorString, "DestinationPrefix")
	}
	buff.WriteString(fmt.Sprintf(" -DestinationPrefix %s", row.DestinationPrefix.String()))
	if len(row.NextHop) != 0 {
		buff.WriteString(fmt.Sprintf(" -NextHop %s", row.NextHop.String()))
	}
	if row.InterfaceIndex == 0 {
		return "", fmt.Errorf(primaryKeyMissingErrorString, "InterfaceIndex")
	}
	buff.WriteString(fmt.Sprintf(" -InterfaceIndex %d", row.InterfaceIndex))
	buff.WriteString(row.ToPowershellString())
	return buff.String(), nil
}

func (row *RouteRow) ToPowershellString() string {
	buff := bytes.NewBuffer([]byte{})
	value := reflect.ValueOf(row)
	value = value.Elem()
	vt := value.Type()
	for i := 0; i < vt.NumField(); i++ {
		ft := vt.Field(i)
		tname, _, set := powershellTagParse(ft.Tag.Get("powershell"))
		field := value.Field(i)
		if tname == "" {
			tname = ft.Name
		}
		if !set {
			continue
		}
		switch field.Type() {
		case reflect.TypeOf(&net.IPNet{}):
			if field.IsNil() {
				continue
			}
			tar := field.Interface().(*net.IPNet)
			buff.WriteString(fmt.Sprintf(" -%s %s", tname, tar.String()))
		case reflect.TypeOf(net.ParseIP("0.0.0.0")):
			tar := field.Interface().(net.IP)
			buff.WriteString(fmt.Sprintf(" -%s %s", tname, tar.String()))
		}
		switch field.Kind() {
		case reflect.String:
			if field.String() == "" {
				continue
			}
			buff.WriteString(fmt.Sprintf(" -%s %s", tname, field.String()))
		case reflect.Uint64:
			buff.WriteString(fmt.Sprintf(" -%s %d", tname, field.Uint()))
		}
	}
	return buff.String()
}

//parseRouteRow parse a format-list format data into route row struct
func parseRouteRow(input []byte) (*RouteRow, error) {
	tmpMap := map[string]interface{}{}
	scanner := bufio.NewScanner(bytes.NewReader(input))
	lastKey := ""
	seperatorIndex := bytes.IndexRune(input, ':')
	for scanner.Scan() {
		line := scanner.Text()
		kvpair := strings.SplitN(line, ":", 2)
		switch len(kvpair) {
		case 1:
			if lastKey == "" {
				return nil, errors.New(line + " is not a valid output")
			}
			tmpMap[lastKey] = tmpMap[lastKey].(string) + line[seperatorIndex+2:len(line)]
		case 2:
			tmpMap[strings.TrimSpace(kvpair[0])] = strings.TrimSpace(kvpair[1])
			lastKey = strings.TrimSpace(kvpair[0])
		default:
			return nil, errors.New(line + " is not a valid output")
		}
	}
	return map2Row(tmpMap)
}

func map2Row(targetMap map[string]interface{}) (*RouteRow, error) {
	if _, ok := targetMap["DestinationPrefix"]; !ok {
		return nil, fmt.Errorf("Not a valid route row, entry is %v", targetMap)
	}
	rtn := RouteRow{}
	config := &mapstructure.DecoderConfig{
		DecodeHook: stringToIPDecodeHookFunc,
		Metadata:   nil,
		Result:     &rtn,
		ZeroFields: false,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}
	return &rtn, decoder.Decode(targetMap)
}

//stringToIPDecodeHookFunc t1 is string, t2 is ip/ipnet
func stringToIPDecodeHookFunc(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
	if f.Kind() != reflect.String {
		return data, nil
	}
	switch t.Kind() {
	case reflect.Uint64:
		return strconv.ParseUint(data.(string), 10, 0)
	}
	switch t {
	case reflect.TypeOf(net.ParseIP("0.0.0.0")):
		rtn := net.ParseIP(data.(string))
		if rtn.String() != data.(string) {
			return data, errors.New("ip address " + data.(string) + " is not valid")
		}
		return rtn, nil
	case reflect.TypeOf(&net.IPNet{}):
		_, rtn, err := net.ParseCIDR(data.(string))
		return rtn, err
	default:
		return data, nil
	}
}

func parseRows(input string) ([]*RouteRow, error) {
	_input := strings.Replace(input, "\r", "", -1)
	ss := strings.Split(_input, "\n")
	var rows []*RouteRow
	buff := bytes.NewBufferString("")
	for _, s := range ss {
		if len(s) == 0 && buff.Len() != 0 {
			row, err := parseRouteRow(buff.Bytes())
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
			buff.Reset()
		} else if len(s) != 0 {
			buff.WriteString(s + "\n")
		}
	}
	if buff.Len() != 0 {
		row, err := parseRouteRow(buff.Bytes())
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
		buff.Reset()
	}
	return rows, nil
}

func powershellTagParse(tarvalue string) (Name string, get, set bool) {
	tmp := strings.Split(tarvalue, ",")
	Name = tmp[0]
	if len(tmp) == 2 {
		if strings.Index(tmp[1], "get;") != -1 {
			get = true
		}
		if strings.Index(tmp[1], "set;") != -1 {
			set = true
		}
		return
	}
	return
}

package winroutenetsh

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	ps "github.com/gorillalabs/go-powershell"
	psbe "github.com/gorillalabs/go-powershell/backend"
)

const (
	getAllRoutesCMD              = "get-netroute -erroraction Ignore | Format-List *"
	getRouteCMDByDest            = "get-netroute %s -erroraction Ignore | Format-List *"
	deleteRouteCMDByDest         = "get-netroute %s | Remove-NetRoute -Confirm:$false"
	setRouteCMDByDest            = "set-netroute"
	newRouteCMD                  = "new-netroute %s | Format-List *"
	getRoutesCMDByInterfaceIndex = "get-netroute -InterfaceIndex %d | Format-List *"
)

type router struct {
	executor ps.Shell
}

func New() IRouter {
	s, _ := ps.New(&psbe.Local{})
	return &router{
		executor: s,
	}
}
func (r *router) GetRoutes() ([]*RouteRow, error) {
	stdout, err := r.runScript(getAllRoutesCMD)
	if err != nil {
		return nil, err
	}
	return parseRows(stdout)
}
func (r *router) GetRouteByDestination(dest string) (*RouteRow, error) {
	stdout, err := r.runScript(fmt.Sprintf(getRouteCMDByDest, dest))
	if err != nil {
		return nil, err
	}
	return parseRouteRow([]byte(strings.Trim(stdout, "\n")))
}
func (r *router) AddRoute(row *RouteRow) error {
	buff := bytes.NewBuffer([]byte{})
	newS, err := row.ToNewString()
	if err != nil {
		return err
	}
	buff.WriteString(newS)
	stdout, err := r.runScript(fmt.Sprintf(newRouteCMD, buff.String()))
	if err != nil {
		return err
	}
	_, err = parseRows(stdout)
	if err != nil {
		return err
	}
	return nil
}
func (r *router) DeleteRouteByDest(dest string) error {
	stdout, err := r.runScript(fmt.Sprintf(deleteRouteCMDByDest, dest))
	if err != nil {
		return err
	}
	if stdout != "" {
		return errors.New(stdout)
	}
	return nil
}
func (r *router) SetRoute(row *RouteRow) error {
	buff := bytes.NewBufferString(setRouteCMDByDest)
	setvalue := row.ToPowershellString()
	if row.DestinationPrefix == nil {
		return fmt.Errorf(primaryKeyMissingErrorString, "DestinationPrefix")
	}
	buff.WriteString(fmt.Sprintf(" -DestinationPrefix %s", row.DestinationPrefix.String()))
	buff.WriteString(setvalue)
	stdout, err := r.runScript(buff.String())
	if err != nil {
		return err
	}
	if stdout != "" {
		return fmt.Errorf("Powershell return a not empty string:%s", stdout)
	}
	return nil
}

func (r *router) runScript(cmdLine string) (string, error) {
	stdout, _, err := r.executor.Execute(cmdLine)
	if err != nil {
		return "", err
	}
	return stdout, nil
}

func (r *router) GetRoutesByInterfaceIndex(index uint64) ([]*RouteRow, error) {
	stdout, err := r.runScript(fmt.Sprintf(getRoutesCMDByInterfaceIndex, index))
	if err != nil {
		return nil, err
	}
	return parseRows(stdout)
}

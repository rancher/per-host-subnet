package hostnat

import (
	"os/exec"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher-metadata/metadata"
	"github.com/rancher/per-host-subnet/setting"
)

const (
	perHostSubnetLabel = "io.rancher.network.per_host_subnet.subnet"
)

func Watch(c metadata.Client) error {
	ipsetPath, err := exec.LookPath("ipset")
	if err != nil {
		return errors.Wrap(err, "Failed to lookup ipset")
	}
	w := &watcher{
		c:         c,
		ipsetName: setting.DefaultDisableHostNATIPset,
		ipsetPath: ipsetPath,
	}
	go c.OnChange(5, w.onChangeNoError)
	return nil
}

type watcher struct {
	c         metadata.Client
	ipsetName string
	ipsetPath string
}

func (w *watcher) onChangeNoError(version string) {
	if err := w.onChange(version); err != nil {
		logrus.Errorf("Failed to apply ipset: %v", err)
	}
}

func (w *watcher) onChange(version string) error {
	logrus.Debug("Evaluating NAT ipset")

	selfHost, err := w.c.GetSelfHost()
	if err != nil {
		return err
	}

	allHosts, err := w.c.GetHosts()
	if err != nil {
		return err
	}

	return w.refreshIPSet(selfHost, allHosts)
}

func (w *watcher) refreshIPSet(selfHost metadata.Host, allHosts []metadata.Host) error {
	out, err := exec.Command(w.ipsetPath, "create", "--exist", w.ipsetName, "hash:net").CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "Failed to create ipset: %s", out)
	}

	current, err := w.getCurrentIPSetEntries()
	if err != nil {
		return err
	}
	desired := w.getDesiredIPSetEntries(selfHost, allHosts)

	toAddEntries, toDelEntries := w.diffIPSetEntries(current, desired)

	var optErr error
	for _, e := range toAddEntries {
		out, err = exec.Command(w.ipsetPath, "add", w.ipsetName, e, "-exist").CombinedOutput()
		if err != nil {
			optErr = errors.Wrap(optErr, err.Error())
			continue
		}
	}
	for _, e := range toDelEntries {
		out, err = exec.Command(w.ipsetPath, "del", w.ipsetName, e, "-exist").CombinedOutput()
		if err != nil {
			optErr = errors.Wrap(optErr, err.Error())
			continue
		}
	}
	return optErr
}

func (w *watcher) diffIPSetEntries(current map[string]bool, desired map[string]bool) (toAddEntries []string, toDelEntries []string) {
	for e := range desired {
		if _, ok := current[e]; !ok {
			toAddEntries = append(toAddEntries, e)
		}
	}
	for e := range current {
		if _, ok := desired[e]; !ok {
			toDelEntries = append(toDelEntries, e)
		}
	}

	return toAddEntries, toDelEntries
}

func (w *watcher) getCurrentIPSetEntries() (map[string]bool, error) {
	currentEntries := map[string]bool{}
	out, err := exec.Command(w.ipsetPath, "list", "-o", "xml", w.ipsetName).CombinedOutput()
	if err != nil {
		return currentEntries, errors.Wrapf(err, "Failed to list ipset %s: %s", w.ipsetName, out)
	}
	o, err := unmarshalIPSetByXML(out)
	if err != nil {
		return currentEntries, err
	}
	for _, e := range o.Members {
		currentEntries[e] = true
	}
	return currentEntries, nil
}

func (w *watcher) getDesiredIPSetEntries(selfHost metadata.Host, allHosts []metadata.Host) map[string]bool {
	desiredEntries := map[string]bool{}
	for _, h := range allHosts {
		if h.UUID != selfHost.UUID {
			subnet := h.Labels[perHostSubnetLabel]
			desiredEntries[subnet] = true
		}
	}
	return desiredEntries
}

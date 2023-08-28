package firewalld

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
	"text/template"
)

const (
	existingRulesZoneFilePath = "/etc/firewalld/zones/dynafire.xml"
	oldRulesZoneFilePath      = "/etc/firewalld/zones/dynafire.xml.old"
	richRuleTemplate          = `<?xml version="1.0" encoding="utf-8"?>
<zone>
{{ range . }}
  <rule family="{{.IPFamily}}">
    <source address="{{.IP}}"/>
    <{{.Rule}}/>
  </rule>
{{ end }}
</zone>
`
)

type FirewallCmd struct {
	Config Config
	rules  []RichRule
}

type RichRule struct {
	IPFamily string
	IP       net.IP
	Rule     string
}

func New() (*FirewallCmd, error) {
	// init config
	if !configExists() {
		initConfig()
	}

	conf, err := parseConfig()
	if err != nil {
		return nil, err
	}

	cmd := &FirewallCmd{
		Config: conf,
		rules:  make([]RichRule, 0),
	}

	// check host requirements
	ok, err := cmd.hostNetworkManagerRunning()
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, errors.New("please ensure NetworkManager is installed and running before continuing")
	}

	ok, err = cmd.hostFirewalldRunning()
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, errors.New("please ensure firewalld is installed and running before continuing")
	}

	ok, err = cmd.hostHasRequiredZone()
	if err != nil {
		return nil, err
	}

	if !ok {
		err = cmd.createRequiredZoneOnHost()
		if err != nil {
			return nil, err
		}
	}

	ok, err = cmd.isHostDefaultZoneDynafire()
	if err != nil {
		return nil, err
	}

	if !ok {
		err = cmd.saveAndReloadConfig()
		if err != nil {
			return nil, err
		}

		err = cmd.setHostDefaultZone()
		if err != nil {
			return nil, err
		}
	}

	err = cmd.saveAndReloadConfig()
	if err != nil {
		return nil, err
	}

	err = cmd.checkConfig()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

func (fwc *FirewallCmd) hostNetworkManagerRunning() (bool, error) {
	cmd := exec.Command("systemctl", "check", "NetworkManager")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.TrimSpace(string(out)) != "inactive" {
			if exErr, ok := err.(*exec.ExitError); ok {
				slog.Error("checking NetworkManager service status did not complete successfully", "command", "systemctl check NetworkManager", "error", exErr)
			} else {
				slog.Error("could not run systemctl to check NetworkManager service status", "error", err)
			}

			return false, err
		}
	}

	if strings.TrimSpace(string(out)) == "active" {
		return true, nil
	}

	return false, nil
}

func (fwc *FirewallCmd) hostFirewalldRunning() (bool, error) {
	cmd := exec.Command("systemctl", "check", "firewalld")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.TrimSpace(string(out)) != "inactive" {
			if exErr, ok := err.(*exec.ExitError); ok {
				slog.Error("checking firewalld service status did not complete successfully", "command", "systemctl check firewalld", "error", exErr)
			} else {
				slog.Error("could not run systemctl to check firewalld service status", "error", err)
			}

			return false, err
		}
	}

	if strings.TrimSpace(string(out)) == "active" {
		return true, nil
	}

	return false, nil
}

func (fwc *FirewallCmd) hostHasRequiredZone() (bool, error) {
	cmd := exec.Command("firewall-cmd", "--get-zones")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exErr, ok := err.(*exec.ExitError); ok {
			slog.Error("checking existing firewalld zones did not complete successfully", "command", "firewall-cmd --get-zones", "error", exErr)
		} else {
			slog.Error("could not run `firewall-cmd --get-zones` to check existing firewalld zones", "error", err)
		}

		return false, err
	}

	zones := strings.Split(strings.TrimSpace(string(out)), " ")
	for _, zone := range zones {
		if zone == "dynafire" {
			return true, nil
		}
	}

	return false, nil
}

func (fwc *FirewallCmd) createRequiredZoneOnHost() error {
	cmd := exec.Command("firewall-cmd", "--permanent", "--new-zone=dynafire")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exErr, ok := err.(*exec.ExitError); ok {
			slog.Error("creating the 'dynafire' firewalld zone did not complete successfully", "command", "firewall-cmd --permanent --new-zone=dynafire", "error", exErr)
		} else {
			slog.Error("could not run `firewall-cmd --permanent --new-zone=dynafire` to create new 'dynafire' firewalld zone", "error", err)
		}

		return nil
	}

	if strings.TrimSpace(string(out)) != "success" {
		return fmt.Errorf("unexpected output while creating new 'dynafire' firewalld zone; expected 'success' but got %s", strings.TrimSpace(string(out)))
	}

	return nil
}

func (fwc *FirewallCmd) reloadHostFirewalldConfig() error {
	cmd := exec.Command("firewall-cmd", "--reload")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exErr, ok := err.(*exec.ExitError); ok {
			slog.Error("reloading firewalld", "command", "firewall-cmd --reload", "error", exErr)
		} else {
			slog.Error("could not run `firewall-cmd --reload` to reload firewalld configuration", "error", err)
		}

		return nil
	}

	if strings.TrimSpace(string(out)) != "success" {
		return fmt.Errorf("unexpected output while reloading firewalld; expected 'success' but got %s", strings.TrimSpace(string(out)))
	}

	return nil
}

func (fwc *FirewallCmd) ResetFirewallRules() error {
	// clear any rules that have been saved to permanent config
	// there may be many, so deleting the zone config file itself is much faster than via firewall-cmd
	if _, err := os.Stat(existingRulesZoneFilePath); err == nil {
		err = os.Remove(existingRulesZoneFilePath)
		if err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if _, err := os.Stat(oldRulesZoneFilePath); err == nil {
		err = os.Remove(oldRulesZoneFilePath)
		if err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	err := fwc.reloadHostFirewalldConfig()
	if err != nil {
		return err
	}

	return nil
}

func (fwc *FirewallCmd) saveAndReloadConfig() error {
	err := fwc.saveHostRuntimeFirewalldConfig()
	if err != nil {
		return err
	}

	err = fwc.reloadHostFirewalldConfig()
	if err != nil {
		return err
	}

	return nil
}

func (fwc *FirewallCmd) saveHostRuntimeFirewalldConfig() error {
	cmd := exec.Command("firewall-cmd", "--runtime-to-permanent")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exErr, ok := err.(*exec.ExitError); ok {
			slog.Error("saving runtime firewalld configuration as permanent did not complete successfully", "command", "firewall-cmd --runtime-to-permanent", "error", exErr)
		} else {
			slog.Error("could not run `firewall-cmd --runtime-to-permanent` to save runtime firewalld configuration", "error", err)
		}

		return nil
	}

	if strings.TrimSpace(string(out)) != "success" {
		return fmt.Errorf("unexpected output while saving runtime firewalld configuration as permanent; expected 'success' but got %s", strings.TrimSpace(string(out)))
	}

	return nil
}

func (fwc *FirewallCmd) isHostDefaultZoneDynafire() (bool, error) {
	cmd := exec.Command("firewall-cmd", "--get-default-zone")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exErr, ok := err.(*exec.ExitError); ok {
			slog.Error("listing firewalld default zone", "command", "firewall-cmd --get-default-zone", "error", exErr)
		} else {
			slog.Error("could not run `firewall-cmd --get-default-zone` to list firewalld default zone", "error", err)
		}

		return false, nil
	}

	if strings.TrimSpace(string(out)) == "dynafire" {
		return true, nil
	}

	return false, nil
}

func (fwc *FirewallCmd) setHostDefaultZone() error {
	cmd := exec.Command("firewall-cmd", "--set-default-zone=dynafire")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exErr, ok := err.(*exec.ExitError); ok {
			slog.Error("setting firewalld default zone", "command", "firewall-cmd --set-default-zone=dynafire", "error", exErr)
		} else {
			slog.Error("could not run `firewall-cmd --set-default-zone=dynafire` to set firewalld default zone", "error", err)
		}

		return nil
	}

	if strings.TrimSpace(string(out)) != "success" {
		return fmt.Errorf("setting firewalld default zone; unexpected output: %s", strings.TrimSpace(string(out)))
	}

	return nil
}

func (fwc *FirewallCmd) setHostDefaultZonePolicy(policy string) error {
	switch strings.ToUpper(policy) {
	case "REJECT", "DROP", "ACCEPT":
		break
	default:
		return errors.New("unknown firewalld target policy")
	}

	cmd := exec.Command("firewall-cmd", "--permanent", "--zone=dynafire", fmt.Sprintf("--set-target=%s", policy))
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exErr, ok := err.(*exec.ExitError); ok {
			slog.Error("setting firewalld default zone traffic acceptance policy", "command", "firewall-cmd --permanent --zone=dynafire --set-target=ACCEPT", "error", exErr)
		} else {
			slog.Error("could not run `firewall-cmd --permanent --zone=dynafire --set-target=ACCEPT` to set firewalld default zone traffic acceptance policy", "error", err)
		}

		return nil
	}

	if strings.TrimSpace(string(out)) != "success" {
		return fmt.Errorf("setting firewalld default zone traffic acceptance policy; unexpected output: %s", strings.TrimSpace(string(out)))
	}

	err = fwc.reloadHostFirewalldConfig()
	if err != nil {
		return err
	}

	return nil
}

func (fwc *FirewallCmd) ruleExists(address net.IP) bool {
	var ruleStr string
	if address.To4() != nil {
		ruleStr = fmt.Sprintf("rule family=ipv4 source address=%s drop", address.String())
	} else {
		ruleStr = fmt.Sprintf("rule family=ipv6 source address=%s drop", address.String())
	}

	cmd := exec.Command("firewall-cmd", "--zone=dynafire", "--query-rich-rule", ruleStr)

	// ignoring the error here as it is sometimes non-zero but still gives the output we want
	out, _ := cmd.CombinedOutput()
	if strings.TrimSpace(string(out)) == "yes" {
		return true
	}

	return false
}

func (fwc *FirewallCmd) BlockIP(address net.IP) error {
	var ruleStr string
	if address.To4() != nil {
		ruleStr = fmt.Sprintf("rule family=ipv4 source address=%s drop", address.String())
	} else {
		ruleStr = fmt.Sprintf("rule family=ipv6 source address=%s drop", address.String())
	}

	if fwc.ruleExists(address) {
		slog.Debug("skipping adding existing rule", "rule", ruleStr)
		return nil
	}

	cmd := exec.Command("firewall-cmd", "--zone=dynafire", "--add-rich-rule", ruleStr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exErr, ok := err.(*exec.ExitError); ok {
			slog.Error("adding firewalld rich rule to block an IP", "command", fmt.Sprintf("firewall-cmd --zone=dynafire --add-rich-rule %s to blacklist an IP", ruleStr), "error", exErr)
		} else {
			slog.Error(fmt.Sprintf("could not run `firewall-cmd --zone=dynafire --add-rich-rule %s` to blacklist an IP", ruleStr), "error", err)
		}

		return nil
	}

	if strings.TrimSpace(string(out)) != "success" {
		return fmt.Errorf("unexpected output while adding a firewalld rich rule to blacklist an IP; expected 'success' but got %s", strings.TrimSpace(string(out)))
	}

	return nil
}

func (fwc *FirewallCmd) BlockIPList(blacklist []net.IP) error {
	// For speed reasons, write out a new zone.xml rules file rather than using firewall-cmd
	zoneConfigFile, err := os.OpenFile(existingRulesZoneFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	for _, ip := range blacklist {
		var ipFamily string
		if ip.To4() != nil {
			ipFamily = "ipv4"
		} else {
			ipFamily = "ipv6"
		}

		fwc.rules = append(fwc.rules, RichRule{
			IPFamily: ipFamily,
			IP:       ip,
			Rule:     "drop",
		})
	}

	tmpl, err := template.New("dynafire.xml").Parse(richRuleTemplate)
	if err != nil {
		return err
	}

	err = tmpl.Execute(zoneConfigFile, fwc.rules)
	if err != nil {
		return err
	}

	err = fwc.reloadHostFirewalldConfig()
	if err != nil {
		return err
	}

	err = fwc.setHostDefaultZonePolicy(strings.ToUpper(fwc.Config.ZoneTargetPolicy))
	if err != nil {
		return err
	}

	return nil
}

func (fwc *FirewallCmd) UnblockIP(address net.IP) error {
	var ruleStr string
	if address.To4() != nil {
		ruleStr = fmt.Sprintf("rule family=ipv4 source address=%s drop", address.String())
	} else {
		ruleStr = fmt.Sprintf("rule family=ipv6 source address=%s drop", address.String())
	}

	if !fwc.ruleExists(address) {
		slog.Debug("skipping removing non-existent rule", "rule", ruleStr)
		return nil
	}

	cmd := exec.Command("firewall-cmd", "--zone=dynafire", "--remove-rich-rule", ruleStr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exErr, ok := err.(*exec.ExitError); ok {
			slog.Error("removing firewalld rich rule to whitelist an IP", "command", fmt.Sprintf("firewall-cmd --zone=dynafire --remove-rich-rule %s to block an IP", ruleStr), "error", exErr)
		} else {
			slog.Error(fmt.Sprintf("could not run `firewall-cmd --zone=dynafire --remove-rich-rule %s` to whitelist an IP", ruleStr), "error", err)
		}

		return nil
	}

	if strings.TrimSpace(string(out)) != "success" {
		return fmt.Errorf("unexpected output while removing a firewalld rich rule to whitelist an IP; expected 'success' but got %s", strings.TrimSpace(string(out)))
	}

	return nil
}

func (fwc *FirewallCmd) checkConfig() error {
	cmd := exec.Command("firewall-cmd", "--check-config")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exErr, ok := err.(*exec.ExitError); ok {
			slog.Error("checking firewalld configuration", "command", "firewall-cmd --check-config'`", "error", exErr)
		} else {
			slog.Error("could not run `firewall-cmd --check-config'`", "error", err)
		}

		return nil
	}

	if strings.TrimSpace(string(out)) != "success" {
		return fmt.Errorf("unexpected output while checking firewalld configuration; expected 'success' but got %s", strings.TrimSpace(string(out)))
	}

	return nil
}

package ssh

import (
	"context"
	"fmt"
	"strings"
)

// InstalledPackage holds the name and installed version of a dpkg package.
type InstalledPackage struct {
	Name    string
	Version string
}

// GetInstalledPackages queries dpkg for the install status of the given package
// names. Returns a map from package name to installed version. Packages that are
// not installed are omitted from the map.
func (c *Client) GetInstalledPackages(ctx context.Context, names []string) (map[string]string, error) {
	if len(names) == 0 {
		return map[string]string{}, nil
	}

	env := map[string]string{"PKGS": strings.Join(names, " ")}
	res, err := c.Run(ctx, `dpkg-query -W -f='${Package}\t${Version}\t${Status}\n' $PKGS 2>/dev/null || true`, env, nil)
	if err != nil {
		return nil, fmt.Errorf("get installed packages: %w", err)
	}

	installed := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(res.Stdout)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue
		}
		// Status field is e.g. "install ok installed"
		if strings.Contains(fields[2], "installed") && !strings.Contains(fields[2], "not-installed") {
			installed[fields[0]] = fields[1]
		}
	}

	return installed, nil
}

// AptInstall runs apt-get install for the given packages. Each entry in the
// packages map is name -> version. An empty version string means latest.
func (c *Client) AptInstall(ctx context.Context, packages map[string]string, update bool) error {
	if len(packages) == 0 {
		return nil
	}

	if update {
		_, err := c.Run(ctx, `apt-get update -qq`, nil, nil)
		if err != nil {
			return fmt.Errorf("apt install: apt-get update: %w", err)
		}
	}

	var args []string
	envIdx := 0
	env := make(map[string]string)
	for name, version := range packages {
		pkgKey := fmt.Sprintf("PKG_%d", envIdx)
		if version != "" && version != "*" {
			verKey := fmt.Sprintf("VER_%d", envIdx)
			env[pkgKey] = name
			env[verKey] = version
			args = append(args, fmt.Sprintf(`"$%s=$%s"`, pkgKey, verKey))
		} else {
			env[pkgKey] = name
			args = append(args, fmt.Sprintf(`"$%s"`, pkgKey))
		}
		envIdx++
	}

	cmd := fmt.Sprintf(`DEBIAN_FRONTEND=noninteractive apt-get install -y -qq %s`, strings.Join(args, " "))
	_, err := c.Run(ctx, cmd, env, nil)
	if err != nil {
		return fmt.Errorf("apt install: %w", err)
	}

	return nil
}

// AptRemove runs apt-get remove (or purge) for the given package names.
func (c *Client) AptRemove(ctx context.Context, names []string, purge bool) error {
	if len(names) == 0 {
		return nil
	}

	env := make(map[string]string)
	var args []string
	for i, name := range names {
		key := fmt.Sprintf("PKG_%d", i)
		env[key] = name
		args = append(args, fmt.Sprintf(`"$%s"`, key))
	}

	action := "remove"
	if purge {
		action = "purge"
	}

	cmd := fmt.Sprintf(`DEBIAN_FRONTEND=noninteractive apt-get %s -y -qq %s`, action, strings.Join(args, " "))
	_, err := c.Run(ctx, cmd, env, nil)
	if err != nil {
		return fmt.Errorf("apt %s: %w", action, err)
	}

	return nil
}

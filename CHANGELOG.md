# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.1] - 2026-04-05

### Fixed
- Wait for `debian_systemd_service` to actually reach active state when starting, catching immediate failures.

## [0.2.0] - 2026-04-05

### Added
- Add `overwrite` attribute to resources.

### Changed
- **BREAKING**: Resources that implicitly overwrote files or data before this change now require `overwrite = true` to maintain that behavior.

### Fixed
- Fix concurrent `apt-get` operations via per-host locks.

## [0.1.4] - 2026-04-03

### Added
- Add `debian_systemd_timer` resource.
- Add documentation for `debian_systemd_service`.

### Changed
- Improve overall documentation.

## [0.1.3] - 2026-04-03

### Added
- Add `debian_systemd_service` resource.

## [0.1.2] - 2026-03-23

### Changed
- Improve first page of documentation.

## [0.1.1] - 2026-03-22

### Added
- Add more examples.

## [0.1.0] - 2026-03-22

### Added
- Initial release of `terraform-provider-debian`.
- Add `debian_systemctl_reload`, `debian_systemctl_restart`, and `debian_systemctl_daemon_reload` actions.
- Add `debian_apt_update` and `debian_apt_upgrade` actions.
- Add `debian_apt_packages` resource for managing installed packages.
- Add `debian_group_member` resource for per-membership management.
- Add `debian_user` resource for managing local users.
- Add `debian_group` resource for managing local groups.
- Add `debian_file` ephemeral resource.
- Add `debian_file` data source with content reading.
- Add `debian_release` data source.
- Add `debian_command` action.
- Add `debian_command` ephemeral resource.
- Add `debian_command` data source for running remote commands.
- Add `debian_directory` resource for managing remote directories.
- Add `debian_file` resource.
- Add Terraform acceptance tests.
- Add provider-level auth with key ring and composite import IDs.
- Add SSH connection manager with session pooling.
- Add CI workflows, Dependabot, and project config.

### Changed
- Change import IDs to key=value format.
- Return `RunError` from `Run` on non-zero exit codes.
- Bump packages, remove dependabot and format.

### Fixed
- Fix file content state mismatch when remote has changed.
- Remove file from state only when file doesn't exist.
- Remove provider private key validation.
- Fix public key in test workflow.
- Fix test workflow.

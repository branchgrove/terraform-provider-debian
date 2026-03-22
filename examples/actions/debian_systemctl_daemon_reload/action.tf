# Reload all systemd unit files after modifying a service definition
action "debian_systemctl_daemon_reload" "this" {
  ssh {
    hostname = "app01.example.com"
  }
}

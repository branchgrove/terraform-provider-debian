# Reload the nginx configuration
action "debian_systemctl_reload" "nginx" {
  unit = "nginx.service"

  ssh {
    hostname = "web01.example.com"
  }
}

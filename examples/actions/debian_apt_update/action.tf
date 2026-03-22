# Refresh the package index
action "debian_apt_update" "this" {
  ssh {
    hostname = "app01.example.com"
  }
}

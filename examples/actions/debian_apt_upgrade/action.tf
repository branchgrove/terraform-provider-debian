# Upgrade all installed packages
action "debian_apt_upgrade" "this" {
  ssh {
    hostname = "app01.example.com"
  }
}

# Perform a dist-upgrade
action "debian_apt_upgrade" "dist" {
  dist_upgrade = true

  ssh {
    hostname = "app01.example.com"
  }
}

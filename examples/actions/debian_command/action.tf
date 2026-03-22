# Run an arbitrary command as an action
action "debian_command" "migrate_db" {
  command = "/opt/myapp/bin/migrate --up"

  env = {
    DATABASE_URL = "postgres://localhost/myapp"
  }

  ssh {
    hostname = "app01.example.com"
  }
}

# Flush iptables rules
action "debian_command" "flush_iptables" {
  command = "iptables -F"

  ssh {
    hostname = "firewall.example.com"
  }
}

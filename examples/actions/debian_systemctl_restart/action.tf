# Restart the application service
action "debian_systemctl_restart" "myapp" {
  unit = "myapp.service"

  ssh {
    hostname = "app01.example.com"
  }
}

# Restart PostgreSQL
action "debian_systemctl_restart" "postgresql" {
  unit = "postgresql.service"

  ssh {
    hostname = "db01.example.com"
  }
}

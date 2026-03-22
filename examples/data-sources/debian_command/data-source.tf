# Read the system hostname
data "debian_command" "hostname" {
  command = "hostname -f"

  ssh {
    hostname = "app01.example.com"
  }
}

# Check available disk space
data "debian_command" "disk_usage" {
  command = "df -h / | tail -1 | awk '{print $4}'"

  ssh {
    hostname = "app01.example.com"
  }
}

# Run a command with environment variables and allow non-zero exit codes
data "debian_command" "check_service" {
  command     = "systemctl is-active nginx"
  allow_error = true

  ssh {
    hostname = "web01.example.com"
  }
}

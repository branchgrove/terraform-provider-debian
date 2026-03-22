# Read the contents of /etc/hostname
data "debian_file" "hostname" {
  path = "/etc/hostname"

  ssh {
    hostname = "app01.example.com"
  }
}

# Read an application config file
data "debian_file" "app_config" {
  path = "/opt/myapp/config.yaml"

  ssh {
    hostname = "app01.example.com"
  }
}

# Read a file with a size limit
data "debian_file" "large_log" {
  path     = "/var/log/syslog"
  max_size = 4096

  ssh {
    hostname = "app01.example.com"
  }
}

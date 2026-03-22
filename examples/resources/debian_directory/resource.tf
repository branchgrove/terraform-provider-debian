# Create a directory for application data
resource "debian_directory" "app_data" {
  path  = "/opt/myapp/data"
  mode  = "0755"
  owner = "myapp"
  group = "myapp"

  ssh {
    hostname = "app01.example.com"
  }
}

# Create a nested directory structure with parent creation
resource "debian_directory" "log_dir" {
  path           = "/var/log/myapp/archive"
  create_parents = true
  mode           = "0750"
  owner          = "syslog"
  group          = "adm"

  ssh {
    hostname = "app01.example.com"
  }
}

# Create a shared directory with group write access
resource "debian_directory" "shared" {
  path  = "/srv/shared"
  mode  = "0775"
  owner = "root"
  group = "developers"

  ssh {
    hostname = "dev01.example.com"
    user     = "admin"
  }
}

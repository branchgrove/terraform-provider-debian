# Create an application service user
resource "debian_user" "myapp" {
  name        = "myapp"
  group       = "myapp"
  home        = "/opt/myapp"
  shell       = "/usr/sbin/nologin"
  system      = true
  create_home = true

  ssh {
    hostname = "app01.example.com"
  }
}

# Create a regular user with supplementary groups
resource "debian_user" "alice" {
  name   = "alice"
  shell  = "/bin/bash"
  groups = ["developers", "sudo"]

  ssh {
    hostname = "dev01.example.com"
  }
}

# Create a user with a specific UID and no home directory
resource "debian_user" "prometheus" {
  name        = "prometheus"
  uid         = 9090
  group       = "prometheus"
  shell       = "/usr/sbin/nologin"
  system      = true
  create_home = false

  ssh {
    hostname = "monitoring.example.com"
  }
}

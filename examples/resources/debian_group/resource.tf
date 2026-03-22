# Create a group for an application
resource "debian_group" "myapp" {
  name = "myapp"

  ssh {
    hostname = "app01.example.com"
  }
}

# Create a system group with a specific GID
resource "debian_group" "prometheus" {
  name   = "prometheus"
  gid    = 9090
  system = true

  ssh {
    hostname = "monitoring.example.com"
  }
}

# Create a group and manage its members
resource "debian_group" "developers" {
  name  = "developers"
  users = ["alice", "bob", "carol"]

  ssh {
    hostname = "dev01.example.com"
  }
}

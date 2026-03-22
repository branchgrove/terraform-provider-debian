# Install packages with latest available versions
resource "debian_apt_packages" "base" {
  packages = {
    "curl" = "*"
    "vim"  = "*"
    "htop" = "*"
    "git"  = "*"
  }

  ssh {
    hostname = "app01.example.com"
  }
}

# Install specific package versions and skip apt-get update
resource "debian_apt_packages" "nginx" {
  packages = {
    "nginx"        = "1.22.*"
    "nginx-common" = "1.22.*"
  }
  update = false

  ssh {
    hostname = "web01.example.com"
  }
}

# Install packages with purge enabled for clean removal
resource "debian_apt_packages" "monitoring" {
  packages = {
    "prometheus-node-exporter" = "*"
  }
  purge = true

  ssh {
    hostname = "monitoring.example.com"
  }
}

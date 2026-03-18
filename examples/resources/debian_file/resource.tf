resource "debian_file" "hosts" {
  path    = "/etc/hosts"
  content = "127.0.0.1 localhost\n"
  mode    = "0644"
  owner   = "root"
  group   = "root"

  ssh {
    hostname = "192.168.1.1"
  }
}
